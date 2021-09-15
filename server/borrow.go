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
	borrowRequestMaster, err := p._makeBorrowRequest(borrowRequestKey, borrowRequestKey.BorrowerUser, []string{MASTER}, nil)
	if err != nil {
		p.API.LogError("Failed to make borrow request.", "err", fmt.Sprintf("%+v", err), "role", MASTER)
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
	mb, mp, err := p._makeAndSendBorrowRequest("", p.borrowChannel.Id, []string{MASTER}, borrowRequestMaster)
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
	roleByUser[borrowRequestMaster.BorrowerUser] = append(roleByUser[borrowRequestMaster.BorrowerUser], BORROWER)
	roleByUser[borrowRequestMaster.LibworkerUser] = append(roleByUser[borrowRequestMaster.LibworkerUser], LIBWORKER)
	for _, kp := range borrowRequestMaster.KeeperUsers {
		roleByUser[kp] = append(roleByUser[kp], KEEPER)
	}

	postByRole := map[string][]*model.Post{}
	postByUser := map[string]*model.Post{}
	borrowByUser := map[string]*Borrow{}

	for user, roles := range roleByUser {

		//make borrow request from key
		borrowRequest, err := p._makeBorrowRequest(borrowRequestKey, borrowRequestKey.BorrowerUser, roles, borrowRequestMaster)
		if err != nil {
			p.API.LogError("Failed to make borrow request.", "err", fmt.Sprintf("%+v", err), "roles", strings.Join(roles, ","))
			resp, _ := json.Marshal(Result{
				Error: "Failed to make borrow request.",
			})

			w.Write(resp)
			return

		}
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
				"role", strings.Join(roleByUser[user], ","), "user", user, "err", fmt.Sprintf("%+v", err))
			resp, _ := json.Marshal(Result{
				Error: "Failed to update relationships.",
			})
			p._rollBackCreated(created)
			w.Write(resp)
			return
		}
	}

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

func (p *Plugin) _makeBorrowRequest(bqk *BorrowRequestKey, borrowerUser string, roles []string, masterBr *BorrowRequest) (*BorrowRequest, error) {

	var err error
	var book Book

	bq := new(BorrowRequest)

	rolesSet := ConvertStringArrayToSet(roles)

	if masterBr == nil {

		bookPost, appErr := p.API.GetPost(bqk.BookPostId)

		if appErr != nil {
			return nil, errors.Wrapf(appErr, "Failed to get book post id %s.", bqk.BookPostId)
		}

		if err := json.Unmarshal([]byte(bookPost.Message), &book); err != nil {
			return nil, errors.Wrapf(err, "Failed to unmarshal bookpost. post id:%s", bqk.BookPostId)
		}

		bq.BookPostId = bqk.BookPostId
		bq.BookId = book.Id
		bq.BookName = book.Name
		bq.Author = book.Author
		bq.RequestDate = GetNowTime()
	} else {

		bq.BookPostId = masterBr.BookPostId
		bq.BookId = masterBr.BookId
		bq.BookName = masterBr.BookName
		bq.Author = masterBr.Author
		bq.RequestDate = masterBr.RequestDate
	}

	bq.Tags = []string{}

	if ConstainsInStringSet(rolesSet, []string{MASTER, BORROWER, LIBWORKER}) {
		bq.BorrowerUser = borrowerUser
		if bq.BorrowerName, err = p._getDisplayNameByUser(borrowerUser); err != nil {
			return nil, errors.Wrapf(err, "Failed to get borrower display name. user:%s", borrowerUser)
		}
		bq.Tags = append(bq.Tags, []string{
			"#BORROWERUSER_EQ_" + bq.BorrowerUser,
		}...)
	}

	if masterBr == nil {
		bq.LibworkerUser = p._distributeWorker(book.LibworkerUsers)
		if bq.LibworkerName, err = p._getDisplayNameByUser(bq.LibworkerUser); err != nil {
			return nil, errors.Wrapf(err, "Failed to get library worker display name. user:%s", bq.LibworkerUser)
		}
	} else {
		bq.LibworkerUser = masterBr.LibworkerUser
		bq.LibworkerName = masterBr.LibworkerName
	}

	bq.Tags = append(bq.Tags, []string{
		"#LIBWORKERUSER_EQ_" + bq.LibworkerUser,
	}...)

	if ConstainsInStringSet(rolesSet, []string{MASTER, LIBWORKER, KEEPER}) {
		if masterBr == nil {

			bq.KeeperUsers = book.KeeperUsers
			bq.KeeperNames = book.KeeperNames
		} else {

			bq.KeeperUsers = masterBr.KeeperUsers
			bq.KeeperNames = masterBr.KeeperNames
		}
		for _, k := range bq.KeeperUsers {
			bq.Tags = append(bq.Tags, "#KEEPERUSER_EQ_"+k)
		}
	}

	bq.WorkflowType = WORKFLOW_BORROW

	p._setVisibleWorkflowByRoles(bq.WorkflowType, rolesSet, bq)
	p._setStatusByWorkflow(STATUS_REQUESTED, bq)

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

// func (p *Plugin) _adjustFieldsByRoles(roles []string, brFull *BorrowRequest) *BorrowRequest {
// 	var brDup *BorrowRequest
// 	DeepCopyBorrowRequest(brDup, brFull)
//
// 	p._clearAllVisible(brDup)
// 	for _, role := range roles {
// 		p._setVisibleFieldsByRole(role, brDup, brFull)
// 	}
//
// 	// stepsSet := ConvertStringArrayToSet(brDup.Worflow)
//
// 	return nil
// }
//
// func (p *Plugin) _clearAllVisible(br *BorrowRequest) {
// 	br.KeeperUsers = []string{}
// 	br.KeeperNames = []string{}
// 	br.BorrowerUser = ""
// 	br.BorrowerName = ""
// 	br.Worflow = []string{}
// 	br.Status = ""
// 	br.Tags = []string{}
// }
//
// func (p *Plugin) _setVisibleFieldsByRole(role string, br *BorrowRequest, brFull *BorrowRequest) {
//
// 	switch role {
// 	case BORROWER:
// 		br.BorrowerUser = brFull.BorrowerUser
// 		br.BorrowerName = brFull.BorrowerName
// 		br.Tags = append(br.Tags)
// 	case LIBWORKER:
// 		br.BorrowerUser = brFull.BorrowerUser
// 		br.BorrowerName = brFull.BorrowerName
// 		br.KeeperUsers = brFull.KeeperUsers
// 		br.KeeperNames = brFull.KeeperNames
// 	case KEEPER:
// 		br.KeeperUsers = brFull.KeeperUsers
// 		br.KeeperNames = brFull.KeeperNames
// 	default:
// 	}
//
// }

func (p *Plugin) _setDateIf(status string, br *BorrowRequest) int64 {
	stepsSet := ConvertStringArrayToSet(br.Worflow)

	if ConstainsInStringSet(stepsSet, []string{status}) {
		return GetNowTime()
	}

	return 0
}
func (p *Plugin) _setStatusByWorkflow(status string, br *BorrowRequest) {
	stepsSet := ConvertStringArrayToSet(br.Worflow)

	if ConstainsInStringSet(stepsSet, []string{status}) {
		br.Status = status
		stag := "#STATUS_EQ_" + br.Status

		for i, tag := range br.Tags {
			if strings.HasPrefix(tag, "#STATUS_EQ_") {
				br.Tags[i] = stag
				return
			}
		}

		br.Tags = append(br.Tags, []string{
			"#STATUS_EQ_" + br.Status,
		}...)
	}

}

func (p *Plugin) _setVisibleWorkflowByRoles(wt string, rolesSet map[string]bool, br *BorrowRequest) {
	// 1. workflow steps are variant
	// 2. if not relevent, change nothning ( keep same as last workflow)
	switch wt {
	case WORKFLOW_BORROW:

		// common
		br.WorkflowType = WORKFLOW_BORROW
		br.Worflow = []string{}
		br.Worflow = append(br.Worflow, STATUS_REQUESTED)
		br.Worflow = append(br.Worflow, STATUS_CONFIRMED)
		if ConstainsInStringSet(rolesSet, []string{MASTER, BORROWER, LIBWORKER}) {
			br.Worflow = append(br.Worflow, STATUS_DELIVIED)
		}
	case WORKFLOW_RENEW:
		// keepers are not relevant, there is no common part
		if ConstainsInStringSet(rolesSet, []string{MASTER, BORROWER, LIBWORKER}) {
			br.Worflow = []string{}
			br.WorkflowType = WORKFLOW_RENEW
			br.Worflow = append(br.Worflow, STATUS_RENEW_REQUESTED)
			br.Worflow = append(br.Worflow, STATUS_RENEW_CONFIRMED)
		}
	case WORKFLOW_RETURN:
		//common
		br.Worflow = []string{}
		br.WorkflowType = WORKFLOW_RETURN
		br.Worflow = append(br.Worflow, STATUS_RETURN_REQUESTED)
		br.Worflow = append(br.Worflow, STATUS_RETURN_CONFIRMED)
		if ConstainsInStringSet(rolesSet, []string{MASTER, KEEPER, LIBWORKER}) {
			br.Worflow = append(br.Worflow, STATUS_RETURNED)
		}
	}

}
