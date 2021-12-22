package main

import (
	// "fmt"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

var lockmap sync.Map

type borrowWithPost struct {
	post   *model.Post
	borrow *Borrow
	delete bool
	create bool
}

func (p *Plugin) handleWorkflowRequest(c *plugin.Context, w http.ResponseWriter, r *http.Request) {

	workflowReq := new(WorkflowRequest)
	err := json.NewDecoder(r.Body).Decode(workflowReq)
	if err != nil {
		p.API.LogError("Failed to convert from workflow request.", "err", err.Error())
		resp, _ := json.Marshal(Result{
			Error: "Failed to convert from workflow request.",
		})

		w.Write(resp)
		return

	}

	all, err := p._loadAndLock(workflowReq)

	if err != nil {
		defer p._unlock(all)

		p.API.LogError("Failed to lock and get posts from workflow requests.", "err", err.Error())
		var errorMessage string
		if errors.Is(err, ErrLocked) || errors.Is(err, ErrStale) {
			errorMessage = p.i18n.GetText("system-busy")
		} else {
			errorMessage = p.i18n.GetText("failed-to-get-borrow")
		}
		resp, _ := json.Marshal(Result{
			Error: errorMessage,
		})

		w.Write(resp)
		return

	}

	defer p._unlock(all)

	bookPostId := all[MASTER][0].borrow.DataOrImage.BookPostId
	bookInfo, err := p._lockAndGetABook(bookPostId)
	if err != nil {
		defer lockmap.Delete(bookPostId)
		p.API.LogError("Failed to lock or get a book.", "err", err.Error())
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

	defer lockmap.Delete(bookPostId)

	if workflowReq.Delete {

		if err := p._deleteBorrowRequest(workflowReq, all, bookInfo); err != nil {
			p.API.LogError("delete borrow request error, please retry.", "error", err.Error())
			resp, _ := json.Marshal(Result{
				Error: fmt.Sprintf("delete borrow request error, please retry."),
			})

			w.Write(resp)
			return
		}

		resp, _ := json.Marshal(Result{
			Error: "",
		})
		w.Write(resp)
		return

	}

	if err := p._process(workflowReq, all, bookInfo); err != nil {
		p.API.LogError("Process  error.", "error", err.Error())
		var errText string
		switch {
		case errors.Is(err, ErrChooseInStockCopy):
			errText = p.i18n.GetText(err.Error())
		case errors.Is(err, ErrNoStock):
			errText = p.i18n.GetText(err.Error())
		default:
			errText = err.Error()
		}

		resp, _ := json.Marshal(Result{
			// Error: fmt.Sprintf("process error."),
			Error: errText,
		})

		w.Write(resp)
		return
	}

	if err := p._save(all, bookInfo); err != nil {
		p.API.LogError("Save error.", "err", err.Error())
		resp, _ := json.Marshal(Result{
			Error: "Save error.",
		})

		w.Write(resp)
		return
	}

	if err := p._notifyStatusChange(all, workflowReq); err != nil {
		p.API.LogError("notify status change error.", "err", err.Error())
		resp, _ := json.Marshal(Result{
			Error: "notify status change error.",
		})

		w.Write(resp)
		return
	}

	resp, _ := json.Marshal(Result{
		Error: "",
	})

	w.Write(resp)

}

type _initPassedParams struct {
	tempPassed map[string]struct{}
}

func (p *Plugin) _initPassed(workflow []Step, fromStep Step, toStep Step, passed map[string]struct{}, params ..._initPassedParams) {

	var tempPassed map[string]struct{}

	if params != nil {
		if params[0].tempPassed == nil {
			tempPassed = map[string]struct{}{}
		} else {
			DeepCopy(&tempPassed, &params[0].tempPassed)
		}
	} else {
		tempPassed = map[string]struct{}{}
	}

	if fromStep.NextStepIndex == nil {
		return
	}

	tempPassed[fromStep.Status] = struct{}{}

	if fromStep.Status == toStep.Status {
		for passedStatus := range tempPassed {
			passed[passedStatus] = struct{}{}
		}
		return
	}

	for _, i := range fromStep.NextStepIndex {
		nextStep := workflow[i]
		if _, ok := tempPassed[nextStep.Status]; ok {
			return
		}
		p._initPassed(workflow, nextStep, toStep, passed, _initPassedParams{
			tempPassed: tempPassed,
		})
	}

}

func (p *Plugin) _clearNextSteps(step *Step, workflow []Step, checked map[string]struct{}) {

	if step.NextStepIndex == nil {
		return
	}

	checked[step.Status] = struct{}{}

	for _, i := range step.NextStepIndex {
		nextStep := &workflow[i]
		if _, ok := checked[nextStep.Status]; ok {
			return
		}
		nextStep.ActionDate = 0
		nextStep.Completed = false
		p._clearNextSteps(nextStep, workflow, checked)
	}
}

func (p *Plugin) _processInventoryOfSingleStep(brwq *borrowWithPost, currStep *Step, nextStep *Step, bookInfo *bookInfo, req *WorkflowRequest) error {

	var (
		increment int
		refStep   *Step
	)

	brq := brwq.borrow.DataOrImage

	if !req.Backward {
		increment = 1
		refStep = nextStep
	} else {
		increment = -1
		refStep = currStep
	}

	inv := bookInfo.book.BookInventory
	pub := bookInfo.book.BookPublic
	switch refStep.WorkflowType {
	case WORKFLOW_BORROW:
		switch refStep.Status {
		case STATUS_REQUESTED:
		case STATUS_CONFIRMED:
			if !req.Backward && inv.Stock <= 0 {
				return ErrNoStock
			}

		case STATUS_KEEPER_CONFIRMED:
			if !req.Backward && inv.Stock <= 0 {
				return ErrNoStock
			}

			if !req.Backward && inv.Copies[req.ChosenCopyId].Status != COPY_STATUS_INSTOCK {
				return ErrChooseInStockCopy
			}

			inv.Stock -= increment

			if inv.Stock <= 0 && pub.IsAllowedToBorrow {
				pub.IsAllowedToBorrow = false
				pub.ReasonOfDisallowed = p.i18n.GetText("no-stock")
			}

			if inv.Stock != 0 && !pub.IsAllowedToBorrow && !pub.ManuallyDisallowed {
				pub.IsAllowedToBorrow = true
				pub.ReasonOfDisallowed = ""
			}

			inv.TransmitOut += increment

			if !req.Backward {
				//First should use request's chosen id, the following should
				inv.Copies[req.ChosenCopyId] = BookCopy{COPY_STATUS_TRANSOUT}
			} else {
				inv.Copies[brq.ChosenCopyId] = BookCopy{COPY_STATUS_INSTOCK}
			}

		case STATUS_DELIVIED:
			inv.TransmitOut -= increment
			inv.Lending += increment

			if !req.Backward {
				inv.Copies[brq.ChosenCopyId] = BookCopy{COPY_STATUS_LENDING}
			} else {
				inv.Copies[brq.ChosenCopyId] = BookCopy{COPY_STATUS_TRANSOUT}
			}

		default:
			return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", refStep.Status, refStep.WorkflowType))
		}
	case WORKFLOW_RENEW:
		switch refStep.Status {
		case STATUS_RENEW_REQUESTED:
		case STATUS_RENEW_CONFIRMED:
		default:
			return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", refStep.Status, refStep.WorkflowType))
		}
	case WORKFLOW_RETURN:
		switch refStep.Status {
		case STATUS_RETURN_REQUESTED:
		case STATUS_RETURN_CONFIRMED:
			inv.Lending -= increment
			inv.TransmitIn += increment

			if !req.Backward {
				inv.Copies[brq.ChosenCopyId] = BookCopy{COPY_STATUS_TRANSIN}
			} else {
				inv.Copies[brq.ChosenCopyId] = BookCopy{COPY_STATUS_LENDING}
			}
		case STATUS_RETURNED:
			inv.TransmitIn -= increment
			inv.Stock += increment

			if inv.Stock > 0 &&
				!pub.IsAllowedToBorrow && !pub.ManuallyDisallowed {
				pub.IsAllowedToBorrow = true
				pub.ReasonOfDisallowed = ""
			}
			if inv.Stock <= 0 &&
				pub.IsAllowedToBorrow {
				pub.IsAllowedToBorrow = false
				pub.ReasonOfDisallowed = p.i18n.GetText("no-stock")
			}
			if !req.Backward {
				inv.Copies[brq.ChosenCopyId] = BookCopy{COPY_STATUS_INSTOCK}
			} else {
				inv.Copies[brq.ChosenCopyId] = BookCopy{COPY_STATUS_TRANSIN}
			}
		default:
			return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", refStep.Status, refStep.WorkflowType))
		}
	default:
		return errors.New(fmt.Sprintf("Unknown workflow: %v", refStep.WorkflowType))
	}

	return nil
}

func (p *Plugin) _processRenewOfSingleStep(br *borrowWithPost, workflow []Step, currStep *Step, nextStep *Step, bookInfo *bookInfo, backward bool) error {
	var (
		increment int
		refStep   *Step
	)

	if !backward {
		increment = 1
		refStep = nextStep
	} else {
		increment = -1
		refStep = currStep
	}

	switch refStep.WorkflowType {
	case WORKFLOW_BORROW:
		switch refStep.Status {
		case STATUS_REQUESTED:
		case STATUS_CONFIRMED:
		case STATUS_KEEPER_CONFIRMED:
		case STATUS_DELIVIED:
		default:
			return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", refStep.Status, refStep.WorkflowType))
		}
	case WORKFLOW_RENEW:
		switch refStep.Status {
		case STATUS_RENEW_REQUESTED:
			if br.borrow.DataOrImage.RenewedTimes >= p.maxRenewTimes {
				return ErrRenewLimited
			}
		case STATUS_RENEW_CONFIRMED:
			br.borrow.DataOrImage.RenewedTimes += increment
		default:
			return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", refStep.Status, refStep.WorkflowType))
		}
	case WORKFLOW_RETURN:
		switch refStep.Status {
		case STATUS_RETURN_REQUESTED:
		case STATUS_RETURN_CONFIRMED:
		case STATUS_RETURNED:
		default:
			return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", refStep.Status, refStep.WorkflowType))
		}
	default:
		return errors.New(fmt.Sprintf("Unknown workflow: %v", refStep.WorkflowType))
	}
	return nil
}

func (p *Plugin) _getKeeperUserByCopyId(copyId string, bookInfo *bookInfo) (string, error) {
	if keeper, ok := bookInfo.book.BookPrivate.CopyKeeperMap[copyId]; ok {
		return keeper.User, nil
	} else {
		return "", errors.New("can't find keeper by copyid:" + copyId)
	}
}

func (p *Plugin) _processSyncKeeper(req *WorkflowRequest, masterBr *borrowWithPost,
	currStep *Step, nextStep *Step, bookInfo *bookInfo) error {

	if nextStep.Status != STATUS_KEEPER_CONFIRMED &&
		currStep.Status != STATUS_KEEPER_CONFIRMED {
		return nil
	}

	if !req.Backward && nextStep.Status == STATUS_KEEPER_CONFIRMED {
		//ignore(delete) other keepers relations
		keeperUser, err := p._getKeeperUserByCopyId(req.ChosenCopyId, bookInfo)
		if err != nil {
			return err
		}
		masterBr.borrow.DataOrImage.KeeperUsers = []string{keeperUser}
		keeperName, err := p._getDisplayNameByUser(keeperUser)
		if err != nil {
			return errors.Errorf("can't find keeper name. keeper: %v", keeperUser)
		}
		masterBr.borrow.DataOrImage.KeeperInfos = KeeperInfoMap{keeperUser: {keeperName}}
		masterBr.borrow.DataOrImage.ChosenCopyId = req.ChosenCopyId

		return nil
	}

	if req.Backward && currStep.Status == STATUS_KEEPER_CONFIRMED {
		//create the other keepers( backward process)
		masterBr.borrow.DataOrImage.KeeperUsers = bookInfo.book.KeeperUsers
		masterBr.borrow.DataOrImage.KeeperInfos = bookInfo.book.KeeperInfos
		masterBr.borrow.DataOrImage.ChosenCopyId = ""

		return nil
	}

	//unrelevent status
	return nil
}

func (p *Plugin) _copyFromMasterAndMark(all map[string][]*borrowWithPost, bookInfo *bookInfo) error {

	master := all[MASTER][0]
	masterWf := master.borrow.DataOrImage.Worflow
	masterSt := masterWf[master.borrow.DataOrImage.StepIndex]

	roleByUser := p._getRoleByUser(master.borrow.DataOrImage)
	brqByUser := map[string]*BorrowRequest{}

	for user, roles := range roleByUser {

		var nBrq *BorrowRequest
		brq, err := p._makeBorrowRequest(
			&BorrowRequestKey{
				BookPostId:   bookInfo.pubPost.Id,
				BorrowerUser: master.borrow.DataOrImage.BorrowerUser,
			},
			master.borrow.DataOrImage.BorrowerUser,
			roles,
			master.borrow.DataOrImage,
			otherRequestData{})
		if err != nil {
			return errors.Wrapf(err, "in _copyFromMasterAndChange, calling _makeBorrowRequest error.")
		}

		//Caution: this workflow share the same space, if some modification is necessary,
		//must deep copy it
		nBrq = brq
		nBrq.Worflow = masterWf
		nBrq.StepIndex = master.borrow.DataOrImage.StepIndex
		p._setStatusTag(masterSt.Status, nBrq)
		nBrq.RenewedTimes = master.borrow.DataOrImage.RenewedTimes
		nBrq.ChosenCopyId = master.borrow.DataOrImage.ChosenCopyId

		brqByUser[user] = nBrq
	}

	roleByUserId := map[string][]string{}

	__setNoKeeperAndMake := func(role string, user string) error {

		all[role][0].borrow.DataOrImage = brqByUser[user]
		directChannel, err := p._getBotDirectChannel(user)
		if err != nil {
			return errors.Wrapf(err, "can't get direct bot channel, user:%v", user)
		}
		roleByUserId[directChannel.Id] = roleByUser[user]
		return nil
	}

	if err := __setNoKeeperAndMake(BORROWER, all[MASTER][0].borrow.DataOrImage.BorrowerUser); err != nil {
		return err
	}

	if err := __setNoKeeperAndMake(LIBWORKER, all[MASTER][0].borrow.DataOrImage.LibworkerUser); err != nil {
		return err
	}

	savedDc := map[string]string{}
	//if keeper is lacked in all[KEEPER], flag it to be created
	__findAnotherRole := func(roleByUserOrId map[string][]string, keeperUserOrId string) string {
		var (
			anotherRole string
			roles       []string
			ok          bool
		)
		if roles, ok = roleByUserOrId[keeperUserOrId]; !ok {
			return ""
		}
		for _, role := range roles {
			if role != KEEPER {
				anotherRole = role
				break
			}
		}
		return anotherRole
	}

	master.borrow.RelationKeys.Keepers = []string{}

	for _, keeperUser := range master.borrow.DataOrImage.KeeperUsers {

		directChannel, err := p._getBotDirectChannel(keeperUser)
		if err != nil {
			return errors.Wrapf(err, "can't get direct bot channel, user:%v", keeperUser)
		}
		savedDc[keeperUser] = directChannel.Id

		var existed bool

		for _, keeper := range all[KEEPER] {
			if keeper.post.ChannelId == directChannel.Id {
				keeper.borrow.DataOrImage = brqByUser[keeperUser]
				master.borrow.RelationKeys.Keepers =
					append(master.borrow.RelationKeys.Keepers, keeper.post.Id)
				existed = true
				break
			}
		}

		if !existed {
			anotherRole := __findAnotherRole(roleByUser, keeperUser)

			if anotherRole == "" {
				msgBytes, err := json.Marshal(brqByUser[keeperUser])
				if err != nil {
					return errors.Wrapf(err, "mashal keeper borrow error")
				}
				//the creation will effect the relation key
				//but make all the db operation in _save, for the sake of rollback
				all[KEEPER] = append(all[KEEPER], &borrowWithPost{
					borrow: &Borrow{
						Role: roleByUser[keeperUser],
						RelationKeys: RelationKeys{
							Book:   bookInfo.pubPost.Id,
							Master: master.post.Id,
						},
						DataOrImage: brqByUser[keeperUser],
					},
					post: &model.Post{
						ChannelId: directChannel.Id,
						Message:   string(msgBytes),
					},
					create: true,
				})
			} else {
				master.borrow.RelationKeys.Keepers =
					append(master.borrow.RelationKeys.Keepers, all[anotherRole][0].post.Id)
			}

		}
	}

	//if keeper is lacked in master.keeperUsers, flag it to be deleted
	for _, keeper := range all[KEEPER] {

		var existed bool
		for _, keeperUser := range master.borrow.DataOrImage.KeeperUsers {
			if savedDc[keeperUser] == keeper.post.ChannelId {
				existed = true
				break
			}

		}

		if !existed {
			anotherRole := __findAnotherRole(roleByUserId, keeper.post.ChannelId)

			if anotherRole == "" {
				keeper.delete = true
			}

			// 			newKeys := []string{}
			// 			for _, key := range master.borrow.RelationKeys.Keepers {
			// 				if key != keeper.post.Id {
			// 					newKeys = append(newKeys, key)
			// 				}
			// 			}
			//
			// 			master.borrow.RelationKeys.Keepers = newKeys
		}
	}

	return nil
}

func (p *Plugin) _process(req *WorkflowRequest, all map[string][]*borrowWithPost, bookInfo *bookInfo) error {

	actionTime := GetNowTime()

	//in-place change
	for _, br := range all[MASTER] {
		workflow := br.borrow.DataOrImage.Worflow
		nextStep := &workflow[req.NextStepIndex]
		currStep := &workflow[br.borrow.DataOrImage.StepIndex]

		// usersSet := ConvertStringArrayToSet(p._getUserByRole(nextStep.ActorRole, br.borrow.DataOrImage))
		// if role == MASTER {
		// 	if !ConstainsInStringSet(usersSet, []string{req.ActorUser}) {
		// 		return errors.New(fmt.Sprintf("This user is not of current actor role"))
		// 	}
		// }

		switch nextStep.WorkflowType {
		case WORKFLOW_BORROW:
			switch nextStep.Status {
			case STATUS_REQUESTED:
			case STATUS_CONFIRMED:
			case STATUS_KEEPER_CONFIRMED:
			case STATUS_DELIVIED:
			default:
				return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", nextStep.Status, nextStep.WorkflowType))
			}
		case WORKFLOW_RENEW:
			switch nextStep.Status {
			case STATUS_RENEW_REQUESTED:
			case STATUS_RENEW_CONFIRMED:
			default:
				return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", nextStep.Status, nextStep.WorkflowType))
			}
		case WORKFLOW_RETURN:
			switch nextStep.Status {
			case STATUS_RETURN_REQUESTED:
			case STATUS_RETURN_CONFIRMED:
			case STATUS_RETURNED:
			default:
				return errors.New(fmt.Sprintf("Unknown status: %v in workflow: %v", nextStep.Status, nextStep.WorkflowType))
			}
		default:
			return errors.New(fmt.Sprintf("Unknown workflow: %v", nextStep.WorkflowType))
		}

		if err := p._processInventoryOfSingleStep(br, currStep, nextStep, bookInfo, req); err != nil {
			return err
		}

		if err := p._processRenewOfSingleStep(br, workflow, currStep, nextStep, bookInfo, req.Backward); err != nil {
			return err
		}

		//Sync the keepers' br, these br will be sync to database in _save methoc
		if err := p._processSyncKeeper(req, br, currStep, nextStep, bookInfo); err != nil {
			return err
		}

		if !req.Backward {
			nextStep.ActionDate = actionTime
			nextStep.Completed = true
			nextStep.LastActualStepIndex = br.borrow.DataOrImage.StepIndex
		}
		br.borrow.DataOrImage.StepIndex = req.NextStepIndex
		p._resetMasterTags(br.borrow.DataOrImage)

		passed := map[string]struct{}{}
		p._initPassed(workflow, workflow[0], *nextStep, passed)
		p._clearNextSteps(nextStep, workflow, passed)

		br.borrow.DataOrImage.MatchId = model.NewId()
	}

	//Set other roles' borrow request
	if err := p._copyFromMasterAndMark(all, bookInfo); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) _deleteBorrowRequest(req *WorkflowRequest, all map[string][]*borrowWithPost, bookInfo *bookInfo) error {

	ms := all[MASTER][0]
	ix := ms.borrow.DataOrImage.StepIndex
	cs := ms.borrow.DataOrImage.Worflow[ix]
	st := cs.Status
	savedDeleted := map[string]bool{}

	if st != STATUS_REQUESTED &&
		st != STATUS_CONFIRMED &&
		st != STATUS_KEEPER_CONFIRMED &&
		st != STATUS_RETURNED {

		return errors.New("the request is not allowed to be deleted.")
	}

	// put master deletion at last
	for _, role := range []string{
		KEEPER, BORROWER, LIBWORKER, MASTER,
	} {
		for _, brwp := range all[role] {
			if brwp == nil {
				continue
			}
			//skip nil post(already deleted)
			if brwp.post != nil {
				if role == MASTER {
					// we must place the inventory adjustment firslty when processing Master
					// this leave a chance to retry when update book parts error
					if st == STATUS_KEEPER_CONFIRMED {
						inv := bookInfo.book.BookInventory
						inv.TransmitOut--
						inv.Stock++

						if inv.Stock > 0 && !bookInfo.book.IsAllowedToBorrow && !bookInfo.book.ManuallyDisallowed {
							bookInfo.book.IsAllowedToBorrow = true
							bookInfo.book.ReasonOfDisallowed = ""
						}

						chosen := brwp.borrow.DataOrImage.ChosenCopyId
						//Don't need make the chosen field in brq blank, because they are deleted

						if _, ok := inv.Copies[chosen]; !ok {
							return errors.New(fmt.Sprintf("can't find copy id: %v", chosen))
						}

						inv.Copies[chosen] = BookCopy{COPY_STATUS_INSTOCK}

						if err := p._updateBookParts(updateOptions{
							pub:     bookInfo.book.BookPublic,
							pubPost: bookInfo.pubPost,
							inv:     inv,
							invPost: bookInfo.invPost,
						}); err != nil {
							return errors.Wrapf(err, "adjust inventory error")
						}

					}
				}
				if _, ok := savedDeleted[brwp.post.Id]; ok {
					continue
				}
				if appErr := p.API.DeletePost(brwp.post.Id); appErr != nil {
					return errors.Wrapf(appErr, "delete error, please retry or contact admin")
				}
				savedDeleted[brwp.post.Id] = true

			}
		}
	}

	return nil
}
func (p *Plugin) _getUserByRole(step Step, brqRole string, brq *BorrowRequest) []string {

	switch step.ActorRole {
	case BORROWER:
		if brq.BorrowerUser == "" {
			return nil
		}
		return []string{brq.BorrowerUser}
	case LIBWORKER:
		if brq.LibworkerUser == "" {
			return nil
		}
		return []string{brq.LibworkerUser}
	case KEEPER:
		if len(brq.KeeperUsers) == 0 {
			return nil
		}
		return brq.KeeperUsers
	}

	return nil
}

func (p *Plugin) _getBorrowById(id string) (*borrowWithPost, error) {
	var bwp borrowWithPost

	post, appErr := p.API.GetPost(id)
	if appErr != nil {
		if appErr.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, errors.Wrapf(appErr, "Get post error.")
	}
	bwp.post = post

	br := new(Borrow)
	if err := json.Unmarshal([]byte(post.Message), br); err != nil {
		return nil, errors.Wrapf(appErr, "Unmarshal post error.")
	}
	bwp.borrow = br

	return &bwp, nil
}

func (p *Plugin) _loadAndLock(req *WorkflowRequest) (map[string][]*borrowWithPost, error) {

	allBorrows := map[string][]*borrowWithPost{}

	if _, ok := lockmap.LoadOrStore(req.MasterPostKey, struct{}{}); ok {
		return nil, errors.New(fmt.Sprintf("Lock %v error", MASTER))
	}

	// p.API.LogInfo("Checking stock.",
	// 	"actor", req.ActorUser,
	// 	"master", req.MasterPostKey,
	// )

	master, err := p._getBorrowById(req.MasterPostKey)
	if err != nil {
		// not found is a fatal error for master post
		// so it should be end (different with other kind posts)
		defer lockmap.Delete(req.MasterPostKey)
		return nil, errors.Wrapf(err, fmt.Sprintf("Get %v borrow error", MASTER))
	}

	// p.API.LogInfo("Checking stale.",
	// 	"actor", req.ActorUser,
	// 	"lastupdated", master.borrow.DataOrImage.MatchId,
	// 	"etag", req.Etag,
	// 	"stale", master.borrow.DataOrImage.MatchId != req.Etag)

	if master.borrow.DataOrImage.MatchId != req.Etag {
		defer lockmap.Delete(req.MasterPostKey)
		return nil, errors.Wrapf(ErrStale, fmt.Sprintf("Get %v borrow stale", MASTER))
	}
	allBorrows[MASTER] = append(allBorrows[MASTER], master)

	lockedIds := map[string]bool{}
	savedBr := map[string]*borrowWithPost{}

	for _, role := range []struct {
		name string
		ids  []string
	}{
		{
			name: BORROWER,
			ids:  []string{master.borrow.RelationKeys.Borrower},
		},
		{
			name: LIBWORKER,
			ids:  []string{master.borrow.RelationKeys.Libworker},
		},
		{
			name: KEEPER,
			ids:  master.borrow.RelationKeys.Keepers,
		},
	} {
		for _, id := range role.ids {

			//for the case of same roles, the id is the same,
			//so we have to check this
			if _, ok := lockedIds[id]; ok {
				if br, ok := savedBr[id]; ok {
					allBorrows[role.name] = append(allBorrows[role.name], br)
				}
				continue
			}
			if _, ok := lockmap.LoadOrStore(id, struct{}{}); ok {
				return nil, errors.New(fmt.Sprintf("Lock %v error", role.name))
			}

			lockedIds[id] = true

			br, err := p._getBorrowById(id)
			if err != nil {
				defer lockmap.Delete(id)
				if errors.Is(err, ErrNotFound) && req.Delete {
					br = nil
				} else {
					return nil, errors.Wrapf(err, fmt.Sprintf("Get %v borrow error", role.name))
				}
			}
			allBorrows[role.name] = append(allBorrows[role.name], br)
			savedBr[id] = br
		}

	}

	return allBorrows, nil
}

func (p *Plugin) _unlock(all map[string][]*borrowWithPost) {

	if all == nil {
		return
	}

	for _, bwp := range all {
		for _, b := range bwp {
			if b != nil {
				lockmap.Delete(b.post.Id)
			}
		}
	}
}

func (p *Plugin) _updateRelationsKeys(all map[string][]*borrowWithPost, oper string, role string, key string) error {

	master := all[MASTER][0]
	switch oper {
	case "create":
		switch role {
		case KEEPER:
			master.borrow.RelationKeys.Keepers = append(master.borrow.RelationKeys.Keepers, key)
		default:
		}
	case "delete":
		switch role {
		case KEEPER:
			var newKeys []string
			for _, keeper := range all[KEEPER] {
				if keeper.post.Id != key {
					newKeys = append(newKeys, keeper.post.Id)
				}
			}
			master.borrow.RelationKeys.Keepers = newKeys
		default:
		}
	default:
	}

	return nil
}

func (p *Plugin) _rollBackDeleted(posts []*model.Post) (map[string]*model.Post, error) {

	created := map[string]*model.Post{}
	for _, post := range posts {
		var oldPost model.Post
		DeepCopy(&oldPost, post)
		oldPost.Id = ""
		if newPost, appErr := p.API.CreatePost(&oldPost); appErr != nil {
			return nil, appErr
		} else {
			created[post.Id] = newPost
		}
	}

	return created, nil
}

func (p *Plugin) _updateMasterForRollback(master *borrowWithPost, rbCreated map[string]*model.Post) error {
	var masterBor Borrow
	//get old master borrow
	if err := json.Unmarshal([]byte(master.post.Message), &masterBor); err != nil {
		return errors.Wrapf(err, "unmarshal old master error")
	}

	for oldPid, newPost := range rbCreated {
		var newBorrow Borrow
		if err := json.Unmarshal([]byte(newPost.Message), &newBorrow); err != nil {
			return errors.Wrapf(err, "unmarshal created borrow error")
		}

		for _, role := range newBorrow.Role {
			switch role {
			case KEEPER:
				newIds := []string{}
				for _, oldId := range masterBor.RelationKeys.Keepers {
					if oldId == oldPid {
						newIds = append(newIds, newPost.Id)
						continue
					}
					newIds = append(newIds, oldId)
				}
				masterBor.RelationKeys.Keepers = newIds
			case BORROWER:
				if masterBor.RelationKeys.Borrower == oldPid {
					masterBor.RelationKeys.Borrower = newPost.Id
				}
			case LIBWORKER:
				if masterBor.RelationKeys.Libworker == oldPid {
					masterBor.RelationKeys.Libworker = newPost.Id
				}
			default:
			}
		}

	}

	if masterBytes, err := json.Marshal(masterBor); err != nil {
		return errors.Wrapf(err, "marshal master error")
	} else {
		var newMasterPost model.Post
		DeepCopy(&newMasterPost, master.post)
		newMasterPost.Message = string(masterBytes)
		if newMasterPost.Message != master.post.Message {
			if _, err := p.API.UpdatePost(&newMasterPost); err != nil {
				return errors.Wrapf(err, "update master post error")
			}
		}

	}

	return nil
}

func (p *Plugin) _workflowUpdateRollback(all map[string][]*borrowWithPost, role string, updated []*model.Post,
	created []*model.Post, deleted []*model.Post) error {
	if err := p._rollbackToOld(updated); err != nil {
		return err
	}
	if err := p._rollBackCreated(created); err != nil {
		return err
	}

	//because MASTER is the latest to be updated, so there is no need to udpate relation
	//if MASTER  failed, no update to DB
	//if something before MASTER failed, no udpate to DB, too

	if rbCreated, err := p._rollBackDeleted(deleted); err != nil {
		return err
	} else {
		if err := p._updateMasterForRollback(all[MASTER][0], rbCreated); err != nil {
			return err
		}
	}

	return nil

}

func (p *Plugin) _save(all map[string][]*borrowWithPost, bookInfo *bookInfo) error {

	updated := []*model.Post{}
	created := []*model.Post{}
	deleted := []*model.Post{}

	processed := map[string]bool{}

	for _, role := range []string{
		BORROWER,
		KEEPER,
		LIBWORKER,
		//MUST MAKE MASTER TO BE UPDATED LAST
		//As there maybe updating relationskeys in the process
		//presumely, MASTER updating must be succussfuly, or every thing will be ruined
		MASTER,
	} {

		brw := all[role]
		for _, br := range brw {

			//prevent a user with multi-roles from repeating prcess
			if br.post != nil {
				if _, ok := processed[br.post.Id]; ok {
					continue
				}
			}

			if br.delete {

				if appErr := p.API.DeletePost(br.post.Id); appErr != nil {
					if err := p._workflowUpdateRollback(all, role, updated, created, deleted); err != nil {
						return errors.Wrapf(err, "Fatal Error, Failed to delete a borrow record. role: %v, and rollback error", role)
					}
				} else {
					deleted = append(deleted, br.post)
				}

				continue

			}

			if br.create {
				if post, appErr := p.API.CreatePost(&model.Post{
					UserId:    p.botID,
					ChannelId: br.post.ChannelId,
					Message:   "",
					Type:      "custom_borrow_type",
				}); appErr != nil {
					if err := p._workflowUpdateRollback(all, role, updated, created, deleted); err != nil {
						return errors.Wrapf(err, "Fatal Error, Failed to create a new borrow record. role: %v, and rollback error", role)
					}
					return errors.Wrapf(appErr, "Failed to create a new borrow record. role: %v", role)
				} else {
					br.post = post
					//Only create should update relation key
					//the updating of deleting is done at _process stage
					p._updateRelationsKeys(all, "create", role, post.Id)
					created = append(created, post)
				}
			}

			brJson, err := json.MarshalIndent(br.borrow, "", "  ")
			if err != nil {
				if err := p._workflowUpdateRollback(all, role, updated, created, deleted); err != nil {
					return errors.Wrapf(err, "Fatal Error, mashal error, role: %v, and rollback error", role)
				}
				return errors.Wrapf(err, fmt.Sprintf("Marshal %v error.", role))
			}
			updBr := &model.Post{}
			if err = DeepCopy(updBr, br.post); err != nil {
				if err := p._workflowUpdateRollback(all, role, updated, created, deleted); err != nil {
					return errors.Wrapf(err, "Fatal Error, deepcopy error, role: %v, and rollback error", role)
				}
				return errors.Wrapf(err, fmt.Sprintf("Deep copy error. role: %v, postid: %v", role, br.post.Id))
			}

			updBr.Message = string(brJson)
			if updBr.Message != br.post.Message {
				if _, err := p.API.UpdatePost(updBr); err != nil {
					if err := p._workflowUpdateRollback(all, role, updated, created, deleted); err != nil {
						return errors.Wrapf(err, "Fatal Error, update post error, role: %v, and rollback error", role)
					}
					return errors.Wrapf(err, fmt.Sprintf("Update post error. role: %v, postid: %v", role, br.post.Id))
				}

				updated = append(updated, br.post)
			}

			processed[br.post.Id] = true
		}

	}

	if err := p._updateBookParts(updateOptions{
		pub:     bookInfo.book.BookPublic,
		pubPost: bookInfo.pubPost,
		pri:     bookInfo.book.BookPrivate,
		priPost: bookInfo.priPost,
		inv:     bookInfo.book.BookInventory,
		invPost: bookInfo.invPost,
	}); err != nil {
		//Only Keeper will be considered for relation updateds
		if err := p._workflowUpdateRollback(all, KEEPER, updated, created, deleted); err != nil {
			return errors.Wrapf(err, "Fatal Error, update pub error, and rollback error")
		}
		p._rollbackToOld(updated)
		return errors.New("update pub error.")
	}
	return nil
}

func (p *Plugin) _rollbackToOld(updated []*model.Post) error {
	for _, post := range updated {
		if _, appErr := p.API.UpdatePost(post); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (p *Plugin) _notifyStatusChange(all map[string][]*borrowWithPost, req *WorkflowRequest) error {
	for _, role := range []string{
		MASTER, BORROWER, LIBWORKER, KEEPER,
	} {
		for _, br := range all[role] {
			if br.delete {
				continue
			}
			currStep := br.borrow.DataOrImage.Worflow[br.borrow.DataOrImage.StepIndex]

			relatedRoleSet := ConvertStringArrayToSet(currStep.RelatedRoles)

			if ConstainsInStringSet(relatedRoleSet, []string{role}) {
				if _, appErr := p.API.CreatePost(&model.Post{
					UserId:    p.botID,
					ChannelId: br.post.ChannelId,
					Message: fmt.Sprintf("Status was changed to %v, by @%v.",
						currStep.Status, req.ActorUser),
					RootId: br.post.Id,
				}); appErr != nil {
					return errors.Wrapf(appErr,
						"Failed to notify status change. role: %v, userid: %v", role, br.post.UserId)
				}
			}
		}
	}
	return nil
}
