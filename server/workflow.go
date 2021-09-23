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
		p.API.LogError("Failed to lock and get posts from workflow requests.", "err", err.Error())
		resp, _ := json.Marshal(Result{
			Error: "Failed to lock and get posts from workflow requests.",
		})

		w.Write(resp)
		return

	}

	defer p._unlock(all)

	if err := p._process(workflowReq, all); err != nil {
		p.API.LogError("Process  error.", "error", err.Error())
		resp, _ := json.Marshal(Result{
			Error: fmt.Sprintf("process error."),
		})

		w.Write(resp)
		return
	}

	if err := p._save(all); err != nil {
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

}

func (p *Plugin) _process(req *WorkflowRequest, all map[string][]*borrowWithPost) error {

	actionTime := GetNowTime()
	for _, role := range []string{
		MASTER, BORROWER, LIBWORKER, KEEPER,
	} {
		for _, br := range all[role] {
			nextStep := &br.borrow.DataOrImage.Worflow[req.NextStepIndex]

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

			nextStep.ActionDate = actionTime
			nextStep.Completed = true
			br.borrow.DataOrImage.LastStepIndex = br.borrow.DataOrImage.StepIndex
			br.borrow.DataOrImage.StepIndex = req.NextStepIndex
			p._setTags(nextStep.Status, br.borrow.DataOrImage)

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
		return nil, errors.Wrapf(err, fmt.Sprintf("Get %v borrow error", MASTER))
	}
	allBorrows[MASTER] = append(allBorrows[MASTER], master)

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

			if _, ok := lockmap.LoadOrStore(id, struct{}{}); ok {
				return nil, errors.New(fmt.Sprintf("Lock %v error", role.name))
			}

			br, err := p._getBorrowById(id)
			if err != nil {
				return nil, errors.Wrapf(err, fmt.Sprintf("Get %v borrow error", role.name))
			}
			allBorrows[role.name] = append(allBorrows[role.name], br)
		}

	}

	return allBorrows, nil
}

func (p *Plugin) _unlock(all map[string][]*borrowWithPost) {

	for _, bwp := range all {
		for _, b := range bwp {
			lockmap.Delete(b.post.Id)
		}
	}
}

func (p *Plugin) _save(all map[string][]*borrowWithPost) error {

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
			brJson, err := json.MarshalIndent(br.borrow, "", "")
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
