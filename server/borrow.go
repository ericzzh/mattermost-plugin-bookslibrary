package main

import (
	// "fmt"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

func (p *Plugin) handleBorrowRequest(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	var borrowRequestKey *BorrowRequestKey
	err := json.NewDecoder(r.Body).Decode(&borrowRequestKey)
	if err != nil {
		p.API.LogError("Failed to convert from borrow request.", "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "Failed to convert from borrow request.",
		})

		w.Write(resp)
		return

	}

	//make borrow request from key
	borrowRequest, err := p._makeBorrowRequest(borrowRequestKey, borrowRequestKey.BorrowerUser)
	if err != nil {
		p.API.LogError("Failed to make borrow request.", "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "Failed to make borrow request.",
		})

		w.Write(resp)
		return

	}

	// start a simple transaction
	// created save all the posted post, to be able to rollback
	created := []*model.Post{}

	//post a masterPost
	mb, mp, err := p._makeAndSendBorrowRequest("", p.borrowChannel.Id, []string{MASTER}, borrowRequest)
	if err != nil {
		p.API.LogError("Failed to post.", "role", MASTER, "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "Failed to post a master record.",
		})

		w.Write(resp)
		return

	}
	created = append(created, mp)

	//post a borrowerPost
	roleByUser := map[string][]string{}
	roleByUser[borrowRequest.BorrowerUser] = append(roleByUser[borrowRequest.BorrowerUser], BORROWER)
	roleByUser[borrowRequest.LibworkerUser] = append(roleByUser[borrowRequest.LibworkerUser], LIBWORKER)
	for _, kp := range borrowRequest.KeeperUsers {
		roleByUser[kp] = append(roleByUser[kp], KEEPER)
	}

	postByRole := map[string][]*model.Post{}
	postByUser := map[string]*model.Post{}
	borrowByUser := map[string]*Borrow{}

	for user, roles := range roleByUser {

		bb, bp, err := p._makeAndSendBorrowRequest(user, "", roles, borrowRequest)
		if err != nil {
			p.API.LogError("Failed to post.", "roles", strings.Join(roles, ","), "user", user, "err", fmt.Sprintf("%+v", err))
			resp, _ := json.Marshal(Result{
				Error: fmt.Sprintf("Failed to post to role: %v, user: %v.", strings.Join(roles, ","), user),
			})
			p._rollBackCreated(created)
			w.Write(resp)
			return

		}
		created = append(created, bp)

		for _, r := range roles {
			postByRole[r] = append(postByRole[r], bp)
		}

		postByUser[user] = bp
		borrowByUser[user] = bb
	}

	// 	bb, bp, err := p._makeAndSendBorrowRequest(borrowRequest.BorrowerUser, "", BORROWER, borrowRequest)
	// 	if err != nil {
	// 		p.API.LogError("Failed to post.", "role", BORROWER, "user", borrowRequest.BorrowerUser, "err", fmt.Sprintf("%+v", err))
	// 		resp, _ := json.Marshal(Result{
	// 			Error: "Failed to post a borrower record.",
	// 		})
	// 		p._rollBackCreated(created)
	// 		w.Write(resp)
	// 		return
	//
	// 	}
	// 	created = append(created, bp)
	//
	// 	//post a libworker record
	// 	lb, lp, err := p._makeAndSendBorrowRequest(borrowRequest.LibworkerUser, "", LIBWORKER, borrowRequest)
	// 	if err != nil {
	// 		p.API.LogError("Failed to post.", "role", LIBWORKER, "user", borrowRequest.LibworkerUser, "err", fmt.Sprintf("%+v", err))
	// 		resp, _ := json.Marshal(Result{
	// 			Error: "Failed to post a libworker record.",
	// 		})
	// 		p._rollBackCreated(created)
	// 		w.Write(resp)
	// 		return
	//
	// 	}
	// 	created = append(created, lp)

	// 	//post a keeper record
	// 	kpIds := []string{}
	// 	kps := map[string]*model.Post{}
	// 	kbs := map[string]*Borrow{}
	//
	// 	for _, user := range borrowRequest.KeeperUsers {
	//
	// 		kb, kp, err := p._makeAndSendBorrowRequest(user, "", KEEPER, borrowRequest)
	//
	// 		if err != nil {
	// 			p.API.LogError("Failed to post.", "role", KEEPER, "user", user, "err", fmt.Sprintf("%+v", err))
	// 			resp, _ := json.Marshal(Result{
	// 				Error: "Failed to post a keeper record.",
	// 			})
	// 			p._rollBackCreated(created)
	// 			w.Write(resp)
	// 			return
	//
	// 		}
	// 		kpIds = append(kpIds, kp.Id)
	// 		kps[user] = kp
	// 		kbs[user] = kb
	// 		created = append(created, kp)
	// 	}

	//Update relationships
	//Master

	kpIds := []string{}
	for _, kp := range postByRole[KEEPER] {
		kpIds = append(kpIds, kp.Id)
	}
        //Just for easy tesing
        sort.Strings(kpIds)

	err = p._updateRelations(mp, RelationKeys{
		Book:      borrowRequestKey.BookPostId,
		Borrower:  postByRole[BORROWER][0].Id,
		Libworker: postByRole[LIBWORKER][0].Id,
		Keepers:   kpIds,
	}, mb)
	if err != nil {
		p.API.LogError("Failed to update master record's relationships.", "role", MASTER, "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "Failed to update master record's relationships.",
		})
		p._rollBackCreated(created)
		w.Write(resp)
		return

	}

	for user, post := range postByUser {

		err = p._updateRelations(post, RelationKeys{
			Book:   borrowRequestKey.BookPostId,
			Master: mp.Id,
		}, borrowByUser[user])
		if err != nil {
			p.API.LogError("Failed to update relationships.",
				"role", strings.Join(roleByUser[user],","), "user", user, "err", fmt.Sprintf("%+v", err))
			resp, _ := json.Marshal(Result{
				Error: "Failed to update relationships.",
			})
			p._rollBackCreated(created)
			w.Write(resp)
			return
		}
	}

	//Borrower
	// 	err = p._updateRelations(bp, RelationKeys{
	// 		Book:   borrowRequestKey.BookPostId,
	// 		Master: mp.Id,
	// 	}, bb)
	// 	if err != nil {
	// 		p.API.LogError("Failed to update borrower record's relationships.", "role", BORROWER, "user", borrowRequest.BorrowerUser, "err", fmt.Sprintf("%+v", err))
	// 		resp, _ := json.Marshal(Result{
	// 			Error: "Failed to update borrower record's relationships.",
	// 		})
	// 		p._rollBackCreated(created)
	// 		w.Write(resp)
	// 		return
	// 	}
	//
	// 	//Library worker
	// 	err = p._updateRelations(lp, RelationKeys{
	// 		Book:   borrowRequestKey.BookPostId,
	// 		Master: mp.Id,
	// 	}, lb)
	// 	if err != nil {
	// 		p.API.LogError("Failed to update libworker's relationships.", "role", LIBWORKER, "user", borrowRequest.LibworkerUser, "err", fmt.Sprintf("%+v", err))
	// 		resp, _ := json.Marshal(Result{
	// 			Error: "Failed to update libworker's relationships..",
	// 		})
	// 		p._rollBackCreated(created)
	// 		w.Write(resp)
	// 		return
	// 	}
	//
	// 	//Keeper
	// 	for _, user := range borrowRequest.KeeperUsers {
	// 		err = p._updateRelations(kps[user], RelationKeys{
	// 			Book:   borrowRequestKey.BookPostId,
	// 			Master: mp.Id,
	// 		}, kbs[user])
	//
	// 		if err != nil {
	// 			p.API.LogError("Failed to update keeper record's relationships.", "role", KEEPER, "user", user, "err", fmt.Sprintf("%+v", err))
	// 			resp, _ := json.Marshal(Result{
	// 				Error: "Failed to update keeper record's relationships.",
	// 			})
	// 			p._rollBackCreated(created)
	// 			w.Write(resp)
	// 			return
	//
	// 		}
	// 	}
	resp, _ := json.Marshal(Result{
		Error: "",
	})

	w.Write(resp)

}

func (p *Plugin) _makeAndSendBorrowRequest(user string, channelId string, role []string, borrowRequest *BorrowRequest) (*Borrow, *model.Post, error) {

	var borrow Borrow

	borrow.Role = role
	borrow.DataOrImage = borrowRequest

	borrow_data_bytes, err := json.MarshalIndent(borrow, "", "")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to convert to a borrow record. role:%v, user:%v", role, user)
	}

	if user != "" {
		userInfo, err := p.API.GetUserByUsername(user)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "Failed to find a user. role: %v, user %v", role, user)
		}

		directChannel, err := p.API.GetDirectChannel(userInfo.Id, p.botID)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "Failed to find or create a direct channel with user. role: %v, user: %v", role, user)
		}

		channelId = directChannel.Id
	}

	var post *model.Post
	var appErr *model.AppError

	if post, appErr = p.API.CreatePost(&model.Post{
		UserId:    p.botID,
		ChannelId: channelId,
		Message:   string(borrow_data_bytes),
		Type:      "custom_borrow_type",
	}); appErr != nil {
		return nil, nil, errors.Wrapf(appErr, "Failed to post a borrow record. role: %v, user: %v", role, user)
	}

	return &borrow, post, nil
}

func (p *Plugin) _rollBackCreated(posts []*model.Post) {
	for _, post := range posts {
		p.API.DeletePost(post.Id)
	}
}

func (p *Plugin) _updateRelations(post *model.Post, relations RelationKeys, borrow *Borrow) error {

	borrow.RelationKeys = relations

	borrow_data_bytes, err := json.MarshalIndent(borrow, "", "")
	if err != nil {
		return errors.Wrapf(err, "Failed to convert to a borrow record. role: %v", borrow.Role)
	}

	post.Message = string(borrow_data_bytes)

	if _, err := p.API.UpdatePost(post); err != nil {
		return errors.Wrapf(err, "Failed to update a borrow record. role: %v", borrow.Role)
	}

	return nil

}

func (p *Plugin) _makeBorrowRequest(bqk *BorrowRequestKey, borrowerUser string) (*BorrowRequest, error) {

	var err error

	bookPost, appErr := p.API.GetPost(bqk.BookPostId)

	if appErr != nil {
		return nil, errors.Wrapf(appErr, "Failed to get book post id %s.", bqk.BookPostId)
	}

	var book Book
	if err := json.Unmarshal([]byte(bookPost.Message), &book); err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal bookpost. post id:%s", bqk.BookPostId)
	}

	bq := new(BorrowRequest)

	bq.BookPostId = bqk.BookPostId
	bq.BookId = book.Id
	bq.BookName = book.Name
	bq.Author = book.Author
	bq.BorrowerUser = borrowerUser
	if bq.BorrowerName, err = p._getDisplayNameByUser(borrowerUser); err != nil {
		return nil, errors.Wrapf(err, "Failed to get borrower display name. user:%s", borrowerUser)
	}
	bq.LibworkerUser = p._distributeWorker(book.LibworkerUsers)
	if bq.LibworkerName, err = p._getDisplayNameByUser(bq.LibworkerUser); err != nil {
		return nil, errors.Wrapf(err, "Failed to get library worker display name. user:%s", bq.LibworkerUser)
	}
	bq.KeeperUsers = book.KeeperUsers
	bq.KeeperNames = book.KeeperNames
	bq.RequestDate = time.Now().UnixNano() / int64(time.Millisecond)
	bq.Status = STATUS_REQUESTED
        bq.WorkflowType = WORKFLOW_BORROW
        bq.Worflow = []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED}

	bq.Tags = []string{
		"#STATUS_EQ_" + bq.Status,
		"#BORROWERUSER_EQ_" + bq.BorrowerUser,
		"#LIBWORKERUSER_EQ_" + bq.LibworkerUser,
	}

	for _, k := range bq.KeeperUsers {
		bq.Tags = append(bq.Tags, "#KEEPERUSER_EQ_"+k)
	}

	return bq, nil
}

func (p *Plugin) _getDisplayNameByUser(user string) (string, error) {

	userObj, appErr := p.API.GetUserByUsername(user)
	if appErr != nil {
		return "", errors.Wrapf(appErr, "Can't get user display name for user %s", user)
	}

	return userObj.LastName + userObj.FirstName, nil
}

func (p *Plugin) _distributeWorker(libworkers []string) string {
	rand.Seed(time.Now().UnixNano())
	who := rand.Intn(len(libworkers))
	return libworkers[who]
}
