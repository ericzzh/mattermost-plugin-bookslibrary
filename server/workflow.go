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

	switch workflowReq.MoveToWorkflow {

	case WORKFLOW_BORROW:
		if err := p._handleWorkflowBorrow(workflowReq, all); err != nil {
			p.API.LogError("Process status error.", "status", workflowReq.MoveToStatus, "workflow", workflowReq.MoveToWorkflow)
			resp, _ := json.Marshal(Result{
				Error: fmt.Sprintf("Process status error. status %v, workflow: %v", workflowReq.MoveToStatus, workflowReq.MoveToWorkflow),
			})

			w.Write(resp)
			return
		}
	case WORKFLOW_RENEW:
		if err := p._handleWorkflowRenew(workflowReq, all); err != nil {
			p.API.LogError("Process status error.", "status", workflowReq.MoveToStatus, "workflow", workflowReq.MoveToWorkflow)
			resp, _ := json.Marshal(Result{
				Error: fmt.Sprintf("Process status error. status %v, workflow: %v", workflowReq.MoveToStatus, workflowReq.MoveToWorkflow),
			})

			w.Write(resp)
			return
		}
	case WORKFLOW_RETURN:
		if err := p._handleWorkflowReturn(workflowReq, all); err != nil {
			p.API.LogError("Process status error.", "status", workflowReq.MoveToStatus, "workflow", workflowReq.MoveToWorkflow)
			resp, _ := json.Marshal(Result{
				Error: fmt.Sprintf("Process status error. status %v, workflow: %v", workflowReq.MoveToStatus, workflowReq.MoveToWorkflow),
			})
			w.Write(resp)
			return
		}
	default:
		p.API.LogError("Next worflow type is wrong.", "workflow", workflowReq.MoveToWorkflow)
		resp, _ := json.Marshal(Result{
			Error: "Next worflow type is wrong.",
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

func (p *Plugin) _handleWorkflowBorrow(req *WorkflowRequest, all map[string][]*borrowWithPost) error {

	for _, brs := range all {
		for _, br := range brs {
			switch req.MoveToStatus {
			case STATUS_CONFIRMED:
				p._changeStatus(req, br.borrow, STATUS_CONFIRMED)
				// should be put after change status, because the workfolw will be also set in that process.
				br.borrow.DataOrImage.ConfirmDate = p._setDateIf(STATUS_CONFIRMED, br.borrow.DataOrImage)
			case STATUS_DELIVIED:
				p._changeStatus(req, br.borrow, STATUS_DELIVIED)
				// should be put after change status, because the workfolw will be also set in that process.
				br.borrow.DataOrImage.DeliveryDate = p._setDateIf(STATUS_DELIVIED, br.borrow.DataOrImage)
			default:
				return errors.New(fmt.Sprintf("Unknown workflow status: %v", req.MoveToStatus))
			}
		}
	}

	return nil
}

func (p *Plugin) _handleWorkflowRenew(req *WorkflowRequest, all map[string][]*borrowWithPost) error {

	for _, brs := range all {
		for _, br := range brs {
			switch req.MoveToStatus {
			case STATUS_RENEW_REQUESTED:
				p._changeStatus(req, br.borrow, STATUS_RENEW_REQUESTED)
				// should be put after change status, because the workfolw will be also set in that process.
				br.borrow.DataOrImage.RenewReqDate = p._setDateIf(STATUS_RENEW_REQUESTED, br.borrow.DataOrImage)
			case STATUS_RENEW_CONFIRMED:
				p._changeStatus(req, br.borrow, STATUS_RENEW_CONFIRMED)
				// should be put after change status, because the workfolw will be also set in that process.
				br.borrow.DataOrImage.RenewConfDate = p._setDateIf(STATUS_RENEW_CONFIRMED, br.borrow.DataOrImage)
			default:
				return errors.New(fmt.Sprintf("Unknown workflow status: %v", req.MoveToStatus))
			}
		}
	}
	return nil
}

func (p *Plugin) _handleWorkflowReturn(req *WorkflowRequest, all map[string][]*borrowWithPost) error {

	for _, brs := range all {
		for _, br := range brs {
			switch req.MoveToStatus {
			case STATUS_RETURN_REQUESTED:
				p._changeStatus(req, br.borrow, STATUS_RETURN_REQUESTED)
				br.borrow.DataOrImage.ReturnReqDate = p._setDateIf(STATUS_RETURN_REQUESTED, br.borrow.DataOrImage)
			case STATUS_RETURN_CONFIRMED:
				p._changeStatus(req, br.borrow, STATUS_RETURN_CONFIRMED)
				br.borrow.DataOrImage.ReturnConfDate = p._setDateIf(STATUS_RETURN_CONFIRMED, br.borrow.DataOrImage)
			case STATUS_RETURNED:
				p._changeStatus(req, br.borrow, STATUS_RETURNED)
				br.borrow.DataOrImage.ReturnDelvDate = p._setDateIf(STATUS_RETURNED, br.borrow.DataOrImage)
			default:
				return errors.New(fmt.Sprintf("Unknown workflow status: %v", req.MoveToStatus))
			}
		}
	}
	return nil
}
func (p *Plugin) _setStatus(role string, all map[string]*borrowWithPost, toStatus string) error {
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

func (p *Plugin) _changeStatus(req *WorkflowRequest, br *Borrow, moveToStatus string) {
	//reset workflow
	if req.CurrentWorkflow != req.MoveToWorkflow {
		rolesSet := ConvertStringArrayToSet(br.Role)
		p._setVisibleWorkflowByRoles(req.MoveToWorkflow, rolesSet, br.DataOrImage)
	}
	p._setStatusByWorkflow(moveToStatus, br.DataOrImage)
}

func (p *Plugin) _notifyStatusChange(all map[string][]*borrowWithPost, req *WorkflowRequest) error {
	for role, brw := range all {
		for _, br := range brw {
			if br.borrow.DataOrImage.Status == req.MoveToStatus &&
				br.borrow.DataOrImage.WorkflowType == req.MoveToWorkflow {
				if _, appErr := p.API.CreatePost(&model.Post{
					UserId:    p.botID,
					ChannelId: br.post.ChannelId,
					Message:   fmt.Sprintf("Status was changed to %v, by @%v.", 
                                           br.borrow.DataOrImage.Status, req.ActUser ),
					RootId:    br.post.Id,
				}); appErr != nil {
					return errors.Wrapf(appErr, 
                                        "Failed to notify status change. role: %v, userid: %v", role, br.post.UserId)
				}
			}
		}
	}
	return nil
}
