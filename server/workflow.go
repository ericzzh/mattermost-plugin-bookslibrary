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
		resp, _ := json.Marshal(Result{
			Error: "Failed to lock and get posts from workflow requests.",
		})

		w.Write(resp)
		return

	}

	defer p._unlock(all)

	bookPostId := all[MASTER][0].borrow.DataOrImage.BookPostId
	bookInfo, err := p._lockAndGetABook(bookPostId)
	if err != nil {
		defer lockmap.Delete(bookPostId)
		p.API.LogError("Failed to lock or get a book.", "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "Failed to lock or get a book.",
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
		resp, _ := json.Marshal(Result{
			// Error: fmt.Sprintf("process error."),
			Error: err.Error(),
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

func (p *Plugin) _initChecked(workflow []Step, fromStep Step, toStep Step, checked map[string]struct{}) {

	checked[fromStep.Status] = struct{}{}

	if fromStep.NextStepIndex == nil {
		return
	}

	if fromStep.Status == toStep.Status {
		return
	}

	for _, i := range fromStep.NextStepIndex {
		nextStep := workflow[i]
		if _, ok := checked[nextStep.Status]; ok {
			return
		}
		p._initChecked(workflow, nextStep, toStep, checked)
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

func (p *Plugin) _processInventoryOfSingleStep(workflow []Step, currStep *Step, nextStep *Step, bookInfo *bookInfo, backward bool) error {

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

	inv := bookInfo.book.BookInventory
	pub := bookInfo.book.BookPublic
	switch refStep.WorkflowType {
	case WORKFLOW_BORROW:
		switch refStep.Status {
		case STATUS_REQUESTED:
		case STATUS_CONFIRMED:

			//just set stock only once
			//As master is the most completed role
			if !backward && inv.Stock <= 0 {
				return errors.New(fmt.Sprintf("No stock."))
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
		case STATUS_KEEPER_CONFIRMED:
		case STATUS_DELIVIED:
			inv.TransmitOut -= increment
			inv.Lending += increment
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

		if err := p._processInventoryOfSingleStep(workflow, currStep, nextStep, bookInfo, req.Backward); err != nil {
			return err
		}

		if err := p._processRenewOfSingleStep(br, workflow, currStep, nextStep, bookInfo, req.Backward); err != nil {
			return err
		}

		if !req.Backward {
			nextStep.ActionDate = actionTime
			nextStep.Completed = true
			nextStep.LastActualStepIndex = br.borrow.DataOrImage.StepIndex
		}
		br.borrow.DataOrImage.StepIndex = req.NextStepIndex
		p._setTags(nextStep.Status, br.borrow.DataOrImage)

		checked := map[string]struct{}{}
		p._initChecked(workflow, workflow[0], *nextStep, checked)
		p._clearNextSteps(nextStep, workflow, checked)
	}

	//Set others
	//We seperate these from master process, for the performance reason
	master := all[MASTER][0]
	masterWf := master.borrow.DataOrImage.Worflow
	masterSt := masterWf[req.NextStepIndex]

	for _, role := range []string{
		BORROWER, LIBWORKER, KEEPER,
	} {
		for _, br := range all[role] {
			//Caution: this workflow share the same space, if some modification is necessary,
			//must deep copy it
			br.borrow.DataOrImage.Worflow = masterWf
			br.borrow.DataOrImage.StepIndex = req.NextStepIndex
			p._setTags(masterSt.Status, br.borrow.DataOrImage)
			br.borrow.DataOrImage.RenewedTimes = master.borrow.DataOrImage.RenewedTimes
		}
	}

	return nil
}

func (p *Plugin) _deleteBorrowRequest(req *WorkflowRequest, all map[string][]*borrowWithPost, bookInfo *bookInfo) error {

	ms := all[MASTER][0]
	ix := ms.borrow.DataOrImage.StepIndex
	cs := ms.borrow.DataOrImage.Worflow[ix]
	st := cs.Status

	if st != STATUS_REQUESTED &&
		st != STATUS_CONFIRMED &&
                st != STATUS_KEEPER_CONFIRMED &&
		st != STATUS_RETURNED {

		return errors.New("the request is not allowed to be deleted.")
	}

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
					if st == STATUS_CONFIRMED || st == STATUS_KEEPER_CONFIRMED {
						inv := bookInfo.book.BookInventory
						inv.TransmitOut--
						inv.Stock++

						if inv.Stock > 0 && !bookInfo.book.IsAllowedToBorrow && !bookInfo.book.ManuallyDisallowed {
							bookInfo.book.IsAllowedToBorrow = true
							bookInfo.book.ReasonOfDisallowed = ""
						}

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
				if appErr := p.API.DeletePost(brwp.post.Id); appErr != nil {
					return errors.Wrapf(appErr, "delete error, please retry or contact admin")
				}
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
		if len(brq.KeeperNames) == 0 {
			return nil
		}
		return brq.KeeperNames
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

	master, err := p._getBorrowById(req.MasterPostKey)
	if err != nil {
		// not found is a fatal error for master post
		// so it should be end (different with other kind posts)
		defer lockmap.Delete(req.MasterPostKey)
		return nil, errors.Wrapf(err, fmt.Sprintf("Get %v borrow error", MASTER))
	}
	allBorrows[MASTER] = append(allBorrows[MASTER], master)

	lockedIds := map[string]bool{}

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

func (p *Plugin) _save(all map[string][]*borrowWithPost, bookInfo *bookInfo) error {

	updated := []*model.Post{}

	for _, role := range []struct {
		name string
	}{
		{
			name: MASTER,
		},
		{
			name: BORROWER,
		},
		{
			name: LIBWORKER,
		},
		{
			name: KEEPER,
		},
	} {

		brw := all[role.name]
		for _, br := range brw {
			brJson, err := json.MarshalIndent(br.borrow, "", "  ")
			if err != nil {
				p._rollbackToOld(updated)
				return errors.Wrapf(err, fmt.Sprintf("Marshal %v error.", role))
			}
			updBr := &model.Post{}
			if err = DeepCopy(updBr, br.post); err != nil {
				p._rollbackToOld(updated)
				return errors.Wrapf(err, fmt.Sprintf("Deep copy error. role: %v, postid: %v", role.name, br.post.Id))
			}

			updBr.Message = string(brJson)
			if updBr.Message != br.post.Message {
				if _, err := p.API.UpdatePost(updBr); err != nil {
					p._rollbackToOld(updated)
					return errors.Wrapf(err, fmt.Sprintf("Update post error. role: %v, postid: %v", role.name, br.post.Id))
				}

				updated = append(updated, br.post)
			}
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
		p._rollbackToOld(updated)
		return errors.New("update pub error.")
	}
	return nil
}

func (p *Plugin) _rollbackToOld(updated []*model.Post) {
	for _, post := range updated {
		p.API.UpdatePost(post)
	}
}

func (p *Plugin) _notifyStatusChange(all map[string][]*borrowWithPost, req *WorkflowRequest) error {
	for _, role := range []string{
		MASTER, BORROWER, LIBWORKER, KEEPER,
	} {
		for _, br := range all[role] {
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
