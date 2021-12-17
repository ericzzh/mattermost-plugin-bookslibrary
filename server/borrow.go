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

type otherRequestData struct {
	processTime int64
}

func (p *Plugin) handleBorrowRequest(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	var otherData otherRequestData
	otherData.processTime = GetNowTime()

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

	bookInfo, err := p._lockAndGetABook(borrowRequestKey.BookPostId)
	if err != nil {
		defer lockmap.Delete(borrowRequestKey.BookPostId)
		p.API.LogError("Failed to lock or get a book.", "err", fmt.Sprintf("%+v", err))
		var errorMessage string
		if errors.Is(err, ErrLocked) || errors.Is(err, ErrStale) {
			errorMessage = p.i18n.GetText("system-busy")
		} else {
			errorMessage = p.i18n.GetText("failed-to-get-book")
		}
		resp, _ := json.Marshal(Result{
			Error: errorMessage,
		})

		w.Write(resp)
		return

	}
	defer lockmap.Delete(borrowRequestKey.BookPostId)

	if err := p._checkConditions(borrowRequestKey, bookInfo); err != nil {

		var resp []byte

		switch {
		case errors.Is(err, ErrBorrowingLimited):
			resp, _ = json.Marshal(Result{
				Error: p.i18n.GetText(err.Error()),
			})
		case errors.Is(err, ErrNoStock):
			resp, _ = json.Marshal(Result{
				Error: p.i18n.GetText(err.Error()),
			})
		default:
			p.API.LogError("Failed to call check conditons.", "err", fmt.Sprintf("%+v", err))
			resp, _ = json.Marshal(Result{
				Error: "Failed to call check conditons",
			})
		}

		w.Write(resp)
		return
	}

	//make borrow request from key
	borrowRequestMaster, err := p._makeBorrowRequest(borrowRequestKey, borrowRequestKey.BorrowerUser, []string{MASTER}, nil,
		otherData)
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
	roleByUser := p._getRoleByUser(borrowRequestMaster)

	postByRole := map[string][]*model.Post{}
	postByUser := map[string]*model.Post{}
	borrowByUser := map[string]*Borrow{}

	for user, roles := range roleByUser {

		//make borrow request from key
		borrowRequest, err := p._makeBorrowRequest(borrowRequestKey, borrowRequestKey.BorrowerUser, roles, borrowRequestMaster,
			otherData)
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
			if err := p._rollBackCreated(created); err != nil {
				p.API.LogError("Fatal Error: Failed to post and rollback error.", "roles", strings.Join(roles, ","), "user", user, "err", fmt.Sprintf("%+v", err))
				resp, _ = json.Marshal(Result{
					Error: fmt.Sprintf("Fatal Error: Failed to post to role: %v, user: %v and rollback error.", strings.Join(roles, ","), user),
				})
			}
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
		if err := p._rollBackCreated(created); err != nil {
			p.API.LogError("Fatal Error: Failed to update master record's relationships. and rollback error", "role", MASTER, "err", fmt.Sprintf("%+v", err))
			resp, _ = json.Marshal(Result{
				Error: "Fatal Error: Failed to update master record's relationships, and rollback error",
			})
		}
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
			if err := p._rollBackCreated(created); err != nil {
				p.API.LogError("Fatal Error: Failed to update relationships and rollback error.",
					"role", strings.Join(roleByUser[user], ","), "user", user, "err", fmt.Sprintf("%+v", err))
				resp, _ = json.Marshal(Result{
					Error: "Fatal Error: Failed to update relationships and rollback error.",
				})
			}
			w.Write(resp)
			return
		}
	}

	resp, _ := json.Marshal(Result{
		Error: "",
	})

	w.Write(resp)

}

func (p *Plugin) _getRoleByUser(borrowRequestMaster *BorrowRequest) map[string][]string {
	roleByUser := map[string][]string{}
	roleByUser[borrowRequestMaster.BorrowerUser] = append(roleByUser[borrowRequestMaster.BorrowerUser], BORROWER)
	roleByUser[borrowRequestMaster.LibworkerUser] = append(roleByUser[borrowRequestMaster.LibworkerUser], LIBWORKER)
	for _, kp := range borrowRequestMaster.KeeperUsers {
		roleByUser[kp] = append(roleByUser[kp], KEEPER)
	}
	return roleByUser
}

func (p *Plugin) _getBotDirectChannel(user string) (*model.Channel, error) {
	userInfo, err := p.API.GetUserByUsername(user)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to find a user. user %v", user)
	}

	directChannel, err := p.API.GetDirectChannel(userInfo.Id, p.botID)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to find or create a direct channel with user. user: %v", user)
	}

	return directChannel, nil
}
func (p *Plugin) _makeAndSendBorrowRequest(user string, channelId string, role []string, borrowRequest *BorrowRequest) (*Borrow, *model.Post, error) {

	var borrow Borrow

	borrow.Role = role
	borrow.DataOrImage = borrowRequest

	// borrow_data_bytes, err := json.Marshal(borrow)
	// if err != nil {
	// 	return nil, nil, errors.Wrapf(err, "Failed to convert to a borrow record. role:%v, user:%v", role, user)
	// }

	if user != "" {

		directChannel, err := p._getBotDirectChannel(user)

		if err != nil {
			return nil, nil, errors.Wrapf(err, "Failed to get bot direct channel. role: %v, user %v", role, user)
		}

		channelId = directChannel.Id
	}

	var post *model.Post
	var appErr *model.AppError

	if post, appErr = p.API.CreatePost(&model.Post{
		UserId:    p.botID,
		ChannelId: channelId,
		// Message:   string(borrow_data_bytes),
		Message: "",
		Type:    "custom_borrow_type",
	}); appErr != nil {
		return nil, nil, errors.Wrapf(appErr, "Failed to post a borrow record. role: %v, user: %v", role, user)
	}

	return &borrow, post, nil
}

func (p *Plugin) _rollBackCreated(posts []*model.Post) error {
	for _, post := range posts {
		if appErr := p.API.DeletePost(post.Id); appErr != nil {
			return appErr
		}
	}

	return nil
}

func (p *Plugin) _updateRelations(post *model.Post, relations RelationKeys, borrow *Borrow) error {

	borrow.RelationKeys = relations

	borrow_data_bytes, err := json.MarshalIndent(borrow, "", "  ")
	if err != nil {
		return errors.Wrapf(err, "Failed to convert to a borrow record. role: %v", borrow.Role)
	}

	post.Message = string(borrow_data_bytes)

	if _, err := p.API.UpdatePost(post); err != nil {
		return errors.Wrapf(err, "Failed to update a borrow record. role: %v", borrow.Role)
	}

	return nil

}

func (p *Plugin) _makeBorrowRequest(bqk *BorrowRequestKey, borrowerUser string, roles []string, masterBr *BorrowRequest,
	otherData otherRequestData) (*BorrowRequest, error) {

	var err error
	var book *Book

	bq := new(BorrowRequest)

	rolesSet := ConvertStringArrayToSet(roles)

	if masterBr == nil {

		bookInfo, err := p.GetABook(bqk.BookPostId)
		if err != nil {
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to get book post id %s.", bqk.BookPostId)
			}
		}
		book = bookInfo.book

		bq.BookPostId = bqk.BookPostId
		bq.BookId = book.BookPublic.Id
		bq.BookName = book.BookPublic.Name
		bq.Author = book.Author
                bq.MatchId = model.NewId()
	} else {
		bq.BookPostId = masterBr.BookPostId
		bq.BookId = masterBr.BookId
		bq.BookName = masterBr.BookName
		bq.Author = masterBr.Author
                bq.MatchId = masterBr.MatchId
	}

	bq.Tags = []string{}

	if ConstainsInStringSet(rolesSet, []string{MASTER, BORROWER, LIBWORKER}) {
		bq.BorrowerUser = borrowerUser
		if bq.BorrowerName, err = p._getDisplayNameByUser(borrowerUser); err != nil {
			return nil, errors.Wrapf(err, "Failed to get borrower display name. user:%s", borrowerUser)
		}
		bq.Tags = append(bq.Tags, []string{
			TAG_PREFIX_BORROWER + bq.BorrowerUser,
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
		TAG_PREFIX_LIBWORKER + bq.LibworkerUser,
	}...)

	if ConstainsInStringSet(rolesSet, []string{MASTER, LIBWORKER, KEEPER}) {
		if masterBr == nil {

			bq.KeeperUsers = book.KeeperUsers
			bq.KeeperInfos = book.KeeperInfos
		} else {

			bq.KeeperUsers = masterBr.KeeperUsers
			bq.KeeperInfos = masterBr.KeeperInfos
		}
		for _, k := range bq.KeeperUsers {
			bq.Tags = append(bq.Tags, TAG_PREFIX_KEEPER+k)
		}
	}

	bq.Worflow = p._createWFTemplate(otherData.processTime)
	p._setStatusTag(STATUS_REQUESTED, bq)

	if masterBr != nil && masterBr.ChosenCopyId != "" {
		bq.Tags = append(bq.Tags, TAG_PREFIX_COPYID+p._convertChosenCopyIdToTag(masterBr.ChosenCopyId))

	}

	bq.StepIndex = 0

	return bq, nil
}

func (p *Plugin) _getDisplayNameByUser(user string) (string, error) {

	userObj, appErr := p.API.GetUserByUsername(user)
	if appErr != nil {
		return "", errors.Wrapf(appErr, "Can't get user display name for user %s", user)
	}

	if userObj.LastName == "" && userObj.FirstName == "" {
		return "", errors.New("Full name is required to be identified.")
	}

	return userObj.LastName + userObj.FirstName, nil
}

func (p *Plugin) _distributeWorker(libworkers []string) string {
	rand.Seed(time.Now().UnixNano())
	who := rand.Intn(len(libworkers))
	return libworkers[who]
}

func (p *Plugin) _convertChosenCopyIdToTag(chosen string) string {
	spilt := strings.Split(chosen, " ")
	return strings.Join(spilt, "_")
}

func (p *Plugin) _resetMasterTags(bq *BorrowRequest) {
	bq.Tags = []string{}

	bq.Tags = append(bq.Tags, []string{
		TAG_PREFIX_BORROWER + bq.BorrowerUser,
	}...)
	bq.Tags = append(bq.Tags, []string{
		TAG_PREFIX_LIBWORKER + bq.LibworkerUser,
	}...)
	for _, k := range bq.KeeperUsers {
		bq.Tags = append(bq.Tags, TAG_PREFIX_KEEPER+k)
	}
	p._setStatusTag(bq.Worflow[bq.StepIndex].Status, bq)

	if bq.ChosenCopyId != "" {
		bq.Tags = append(bq.Tags, TAG_PREFIX_COPYID+p._convertChosenCopyIdToTag(bq.ChosenCopyId))
	}
}

func (p *Plugin) _setStatusTag(status string, br *BorrowRequest) {

	stag := TAG_PREFIX_STATUS + status

	for i, tag := range br.Tags {
		if strings.HasPrefix(tag, TAG_PREFIX_STATUS) {
			br.Tags[i] = stag
			return
		}
	}

	br.Tags = append(br.Tags, []string{
		TAG_PREFIX_STATUS + status,
	}...)

}

func (p *Plugin) _createWFTemplate(prt int64) []Step {

	return []Step{
		{
			WorkflowType:  WORKFLOW_BORROW,
			Status:        STATUS_REQUESTED,
			ActorRole:     LIBWORKER,
			Completed:     true,
			ActionDate:    prt,
			NextStepIndex: []int{1},
			RelatedRoles: []string{
				MASTER, BORROWER, LIBWORKER, KEEPER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_BORROW,
			Status:        STATUS_CONFIRMED,
			ActorRole:     KEEPER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: []int{2},
			RelatedRoles: []string{
				MASTER, BORROWER, LIBWORKER, KEEPER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_BORROW,
			Status:        STATUS_KEEPER_CONFIRMED,
			ActorRole:     BORROWER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: []int{3},
			RelatedRoles: []string{
				MASTER, LIBWORKER, KEEPER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_BORROW,
			Status:        STATUS_DELIVIED,
			ActorRole:     BORROWER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: []int{4, 6},
			RelatedRoles: []string{
				MASTER, BORROWER, LIBWORKER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_RENEW,
			Status:        STATUS_RENEW_REQUESTED,
			ActorRole:     LIBWORKER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: []int{5},
			RelatedRoles: []string{
				MASTER, BORROWER, LIBWORKER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_RENEW,
			Status:        STATUS_RENEW_CONFIRMED,
			ActorRole:     BORROWER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: []int{6, 4},
			RelatedRoles: []string{
				MASTER, BORROWER, LIBWORKER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_RETURN,
			Status:        STATUS_RETURN_REQUESTED,
			ActorRole:     LIBWORKER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: []int{7},
			RelatedRoles: []string{
				MASTER, BORROWER, LIBWORKER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_RETURN,
			Status:        STATUS_RETURN_CONFIRMED,
			ActorRole:     KEEPER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: []int{8},
			RelatedRoles: []string{
				MASTER, BORROWER, LIBWORKER, KEEPER,
			},
			LastActualStepIndex: -1,
		},
		{
			WorkflowType:  WORKFLOW_RETURN,
			Status:        STATUS_RETURNED,
			ActorRole:     LIBWORKER,
			Completed:     false,
			ActionDate:    0,
			NextStepIndex: nil,
			RelatedRoles: []string{
				MASTER, LIBWORKER, KEEPER,
			},
			LastActualStepIndex: -1,
		},
	}

}

func (p *Plugin) _checkConditions(brk *BorrowRequestKey, bookInfo *bookInfo) error {

	book := bookInfo.book
	//check if stock is sufficent.
	if book.BookInventory.Stock <= 0 {

		if book.BookPublic.IsAllowedToBorrow {
			book.BookPublic.IsAllowedToBorrow = false
			book.BookPublic.ReasonOfDisallowed = p.i18n.GetText("no-stock")

			if err := p._updateBookParts(updateOptions{
				pub:     book.BookPublic,
				pubPost: bookInfo.pubPost,
			}); err != nil {
				return errors.New("update pub error.")
			}
		}

		return ErrNoStock

	}

	//check if max borrowing concurrent limit is obeyed
	posts, err := p.API.SearchPostsInTeam(p.team.Id, []*model.SearchParams{
		{
			Terms:     TAG_PREFIX_BORROWER + brk.BorrowerUser,
			IsHashtag: true,
			InChannels: []string{
				p.borrowChannel.Name,
			},
		},
	})

	if err != nil {
		return errors.Wrapf(err, "search posts error.")
	}

	count := 0

	for _, post := range posts {
		if post.Type != "custom_borrow_type" {
			continue
		}
		var br Borrow
		json.Unmarshal([]byte(post.Message), &br)

		//even not very possible, this makes the result safe
		if br.DataOrImage.BorrowerUser != brk.BorrowerUser {
			continue
		}

		status := br.DataOrImage.Worflow[br.DataOrImage.StepIndex].Status

		switch {
		case status == STATUS_RETURN_CONFIRMED ||
			status == STATUS_RETURNED:
		default:
			count++
		}

	}

	if count >= p.borrowTimes {
		return ErrBorrowingLimited
	}

	return nil
}


func (p *Plugin) _lockAndGetABook(id string) (*bookInfo, error) {

	//lock pub part only
	if _, ok := lockmap.LoadOrStore(id, struct{}{}); ok {
		return nil, errors.Wrapf(ErrLocked , "Failed to get book post id %s.", id)
	}

	var (
		bookInfo *bookInfo
		err      error
	)
	if bookInfo, err = p.GetABook(id); err != nil {
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to get book post id %s.", id)
		}
	}

	return bookInfo, nil
}
