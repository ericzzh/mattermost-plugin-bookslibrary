package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"
	// "github.com/mattermost/mattermost-server/v5/model"
)

type workflowEnv struct {
	td                TestData
	api               *plugintest.API
	plugin            *Plugin
	realbrPosts       map[string]*model.Post
	realbrUpdPosts    map[string]*model.Post
	createdPid        map[string]string
	worker            string
	worker_botId      string
	postById          map[string]*model.Post
	realNotifyThreads map[string]*model.Post
	getCurrentPosts   func() ReturnedInfo
	updErrCtrl        map[string]bool
}

type injectOpt struct {
	onGetPost     func()
	ifUpdErrCtrl  bool
	onSearchPosts func(api *plugintest.API, plugin *Plugin, td *TestData) func()
}

func newWorkflowEnv(injects ...injectOpt) *workflowEnv {
	var inject injectOpt
	if injects != nil {
		inject = injects[0]
	}
	env := workflowEnv{}

	env.td = NewTestData()
	td := env.td

	env.api = td.ApiMockCommon()
	env.plugin = td.NewMockPlugin()
	env.plugin.SetAPI(env.api)

	var injectOpt InjectOptions

	if inject.ifUpdErrCtrl {

		env.updErrCtrl = map[string]bool{}
		env.realbrUpdPosts = map[string]*model.Post{}

		injectOpt.updatePost = func() {

			for _, chid := range []string{
				td.BorChannelId,
				td.BorId_botId,
				td.Worker1Id_botId,
				td.Worker2Id_botId,
				td.Keeper1Id_botId,
				td.Keeper2Id_botId,
			} {

				env.api.On("UpdatePost", mock.MatchedBy(td.MatchPostByChannel(chid))).
					Run(func(args mock.Arguments) {
						post := args.Get(0).(*model.Post)
						env.realbrUpdPosts[post.ChannelId] = post
					}).
					Return(nil, func(post *model.Post) *model.AppError {
						// fmt.Println(post)
						if env.updErrCtrl[post.ChannelId] {
							return &model.AppError{}
						}

						return nil

					})
			}
		}
	}

	if inject.onSearchPosts != nil {
		injectOpt.searchPosts = inject.onSearchPosts(env.api, env.plugin, &td)
	}

	env.getCurrentPosts = GenerateBorrowRequest(env.td, env.plugin, env.api, injectOpt)
	if env.realbrUpdPosts != nil {
		returnedInfo := env.getCurrentPosts()
		env.realbrPosts = returnedInfo.RealbrPost
		env.createdPid = returnedInfo.CreatedPid
	} else {
		returnedInfo := env.getCurrentPosts()
		env.realbrUpdPosts = returnedInfo.RealbrUpdPosts
		env.realbrPosts = returnedInfo.RealbrPost
env.createdPid = returnedInfo.CreatedPid
	}

	if len(env.realbrPosts) == 0 {
		return &env
	}

	var master Borrow

	json.Unmarshal([]byte(env.realbrPosts[env.td.BorChannelId].Message), &master)
	worker := master.DataOrImage.LibworkerUser
	env.worker = worker
	var worker_botId string
	if worker == "worker1" {
		env.worker_botId = td.Worker1Id_botId
	} else {
		env.worker_botId = td.Worker2Id_botId
	}
	worker_botId = env.worker_botId

	env.postById = map[string]*model.Post{}
	env.realNotifyThreads = map[string]*model.Post{}
	saveNotifiyThread := func(args mock.Arguments) {
		realNotifyThread := args.Get(0).(*model.Post)
		env.realNotifyThreads[realNotifyThread.ChannelId] = realNotifyThread
	}

	matchThreadByChannel := func(channelId string) func(*model.Post) bool {
		return func(post *model.Post) bool {
			return post.ChannelId == channelId && post.RootId != ""
		}
	}
	for _, channelId := range []string{
		td.BorChannelId,
		td.BorId_botId,
		worker_botId,
		td.Keeper1Id_botId,
		td.Keeper2Id_botId,
	} {
		env.postById[env.createdPid[channelId]] = env.realbrUpdPosts[channelId]

		//This realbrUpdPosts should be updated every time some update ocurred
		env.api.On("GetPost", env.createdPid[channelId]).
			Return(func(id string) *model.Post {
				if inject.onGetPost != nil {
					inject.onGetPost()
				}
				return env.postById[id]
			}, nil)
		env.api.On("CreatePost", mock.MatchedBy(matchThreadByChannel(channelId))).
			Run(saveNotifiyThread).Return(&model.Post{}, nil)
	}

	return &env
}

func TestHandleWorkflow(t *testing.T) {
	logSwitch = false
	_ = fmt.Println

	env := newWorkflowEnv()

	td := env.td
	plugin := env.plugin

	getCurrentPosts := env.getCurrentPosts
	realbrUpdPosts := env.realbrUpdPosts
	createdPid := env.createdPid

	worker := env.worker
	worker_botId := env.worker_botId
	postById := env.postById

	type testResult struct {
		role    string
		chid    string
		notifiy bool
		brq     BorrowRequest
	}

	type testData struct {
		wfr    WorkflowRequest
		result []testResult
	}

	t.Run("normal_borrow_workflow", func(t *testing.T) {

		testWorkflow := []testData{
			{
				WorkflowRequest{
					MasterPostKey:   createdPid[td.BorChannelId],
					ActUser:         worker,
					CurrentWorkflow: WORKFLOW_BORROW,
					MoveToWorkflow:  WORKFLOW_BORROW,
					CurrentStatus:   STATUS_REQUESTED,
					MoveToStatus:    STATUS_CONFIRMED,
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							ConfirmDate:  1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_BORROW,
									NextStatus:       STATUS_DELIVIED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:    BORROWER,
						chid:    td.BorId_botId,
						notifiy: true,
						brq: BorrowRequest{
							ConfirmDate:  1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_BORROW,
									NextStatus:       STATUS_DELIVIED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							ConfirmDate:  1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_BORROW,
									NextStatus:       STATUS_DELIVIED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper1Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ConfirmDate:  1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper2Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ConfirmDate:  1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
				},
			},
			{
				WorkflowRequest{
					MasterPostKey:   createdPid[td.BorChannelId],
					ActUser:         worker,
					CurrentWorkflow: WORKFLOW_BORROW,
					MoveToWorkflow:  WORKFLOW_BORROW,
					CurrentStatus:   STATUS_CONFIRMED,
					MoveToStatus:    STATUS_DELIVIED,
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							DeliveryDate: 1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
							LastStatus:   STATUS_CONFIRMED,
							Status:       STATUS_DELIVIED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RENEW,
									NextStatus:       STATUS_RENEW_REQUESTED,
								},
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_DELIVIED,
							},
						},
					},
					{
						role:    BORROWER,
						chid:    td.BorId_botId,
						notifiy: true,
						brq: BorrowRequest{
							DeliveryDate: 1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
							LastStatus:   STATUS_CONFIRMED,
							Status:       STATUS_DELIVIED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RENEW,
									NextStatus:       STATUS_RENEW_REQUESTED,
								},
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_DELIVIED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							DeliveryDate: 1,
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
							LastStatus:   STATUS_CONFIRMED,
							Status:       STATUS_DELIVIED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RENEW,
									NextStatus:       STATUS_RENEW_REQUESTED,
								},
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_DELIVIED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper1Id_botId,
						brq: BorrowRequest{
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper2Id_botId,
						brq: BorrowRequest{
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey:   createdPid[td.BorChannelId],
					ActUser:         worker,
					CurrentWorkflow: WORKFLOW_BORROW,
					MoveToWorkflow:  WORKFLOW_RENEW,
					CurrentStatus:   STATUS_DELIVIED,
					MoveToStatus:    STATUS_RENEW_REQUESTED,
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							RenewReqDate: 1,
							WorkflowType: WORKFLOW_RENEW,
							Worflow:      []string{STATUS_RENEW_REQUESTED, STATUS_RENEW_CONFIRMED},
							LastStatus:   STATUS_DELIVIED,
							Status:       STATUS_RENEW_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RENEW,
									NextStatus:       STATUS_RENEW_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role:    BORROWER,
						chid:    td.BorId_botId,
						notifiy: true,
						brq: BorrowRequest{
							RenewReqDate: 1,
							WorkflowType: WORKFLOW_RENEW,
							Worflow:      []string{STATUS_RENEW_REQUESTED, STATUS_RENEW_CONFIRMED},
							LastStatus:   STATUS_DELIVIED,
							Status:       STATUS_RENEW_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RENEW,
									NextStatus:       STATUS_RENEW_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							RenewReqDate: 1,
							WorkflowType: WORKFLOW_RENEW,
							Worflow:      []string{STATUS_RENEW_REQUESTED, STATUS_RENEW_CONFIRMED},
							LastStatus:   STATUS_DELIVIED,
							Status:       STATUS_RENEW_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RENEW,
									NextStatus:       STATUS_RENEW_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper1Id_botId,
						brq: BorrowRequest{
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper2Id_botId,
						brq: BorrowRequest{
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey:   createdPid[td.BorChannelId],
					ActUser:         worker,
					CurrentWorkflow: WORKFLOW_RENEW,
					MoveToWorkflow:  WORKFLOW_RENEW,
					CurrentStatus:   STATUS_RENEW_REQUESTED,
					MoveToStatus:    STATUS_RENEW_CONFIRMED,
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							RenewConfDate: 1,
							WorkflowType:  WORKFLOW_RENEW,
							Worflow:       []string{STATUS_RENEW_REQUESTED, STATUS_RENEW_CONFIRMED},
							LastStatus:    STATUS_RENEW_REQUESTED,
							Status:        STATUS_RENEW_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role:    BORROWER,
						chid:    td.BorId_botId,
						notifiy: true,
						brq: BorrowRequest{
							RenewConfDate: 1,
							WorkflowType:  WORKFLOW_RENEW,
							Worflow:       []string{STATUS_RENEW_REQUESTED, STATUS_RENEW_CONFIRMED},
							LastStatus:    STATUS_RENEW_REQUESTED,
							Status:        STATUS_RENEW_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							RenewConfDate: 1,
							WorkflowType:  WORKFLOW_RENEW,
							Worflow:       []string{STATUS_RENEW_REQUESTED, STATUS_RENEW_CONFIRMED},
							LastStatus:    STATUS_RENEW_REQUESTED,
							Status:        STATUS_RENEW_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{td.BorrowUser},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper1Id_botId,
						brq: BorrowRequest{
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper2Id_botId,
						brq: BorrowRequest{
							WorkflowType: WORKFLOW_BORROW,
							Worflow:      []string{STATUS_REQUESTED, STATUS_CONFIRMED},
							LastStatus:   STATUS_REQUESTED,
							Status:       STATUS_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_REQUESTED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_CONFIRMED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey:   createdPid[td.BorChannelId],
					ActUser:         worker,
					CurrentWorkflow: WORKFLOW_RENEW,
					MoveToWorkflow:  WORKFLOW_RETURN,
					CurrentStatus:   STATUS_RENEW_CONFIRMED,
					MoveToStatus:    STATUS_RETURN_REQUESTED,
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnReqDate: 1,
							WorkflowType:  WORKFLOW_RETURN,
							Worflow:       []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:    STATUS_RENEW_CONFIRMED,
							Status:        STATUS_RETURN_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:    BORROWER,
						chid:    td.BorId_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnReqDate: 1,
							WorkflowType:  WORKFLOW_RETURN,
							Worflow:       []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED},
							LastStatus:    STATUS_RENEW_CONFIRMED,
							Status:        STATUS_RETURN_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnReqDate: 1,
							WorkflowType:  WORKFLOW_RETURN,
							Worflow:       []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:    STATUS_RENEW_CONFIRMED,
							Status:        STATUS_RETURN_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper1Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnReqDate: 1,
							WorkflowType:  WORKFLOW_RETURN,
							Worflow:       []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:    STATUS_CONFIRMED,
							Status:        STATUS_RETURN_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper2Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnReqDate: 1,
							WorkflowType:  WORKFLOW_RETURN,
							Worflow:       []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:    STATUS_CONFIRMED,
							Status:        STATUS_RETURN_REQUESTED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURN_CONFIRMED,
								},
							},
							ActUsers: []string{worker},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_REQUESTED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey:   createdPid[td.BorChannelId],
					ActUser:         worker,
					CurrentWorkflow: WORKFLOW_RETURN,
					MoveToWorkflow:  WORKFLOW_RETURN,
					CurrentStatus:   STATUS_RETURN_REQUESTED,
					MoveToStatus:    STATUS_RETURN_CONFIRMED,
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnConfDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_REQUESTED,
							Status:         STATUS_RETURN_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURNED,
								},
							},
							ActUsers: td.ABook.KeeperUsers,
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:    BORROWER,
						chid:    td.BorId_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnConfDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED},
							LastStatus:     STATUS_RETURN_REQUESTED,
							Status:         STATUS_RETURN_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURNED,
								},
							},
							ActUsers: []string{},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnConfDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_REQUESTED,
							Status:         STATUS_RETURN_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURNED,
								},
							},
							ActUsers: td.ABook.KeeperUsers,
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper1Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnConfDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_REQUESTED,
							Status:         STATUS_RETURN_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURNED,
								},
							},
							ActUsers: td.ABook.KeeperUsers,
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper2Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnConfDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_REQUESTED,
							Status:         STATUS_RETURN_CONFIRMED,
							Next: []Next{
								{
									NextWorkFlowType: WORKFLOW_RETURN,
									NextStatus:       STATUS_RETURNED,
								},
							},
							ActUsers: td.ABook.KeeperUsers,
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURN_CONFIRMED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey:   createdPid[td.BorChannelId],
					ActUser:         worker,
					CurrentWorkflow: WORKFLOW_RETURN,
					MoveToWorkflow:  WORKFLOW_RETURN,
					CurrentStatus:   STATUS_RETURN_CONFIRMED,
					MoveToStatus:    STATUS_RETURNED,
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnDelvDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_CONFIRMED,
							Status:         STATUS_RETURNED,
							Next:           []Next{},
							ActUsers:        []string{},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURNED,
							},
						},
					},
					{
						role: BORROWER,
						chid: td.BorId_botId,
						brq: BorrowRequest{
							WorkflowType: WORKFLOW_RETURN,
							Worflow:      []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED},
							LastStatus:   STATUS_RETURN_REQUESTED,
							Status:       STATUS_RETURN_CONFIRMED,
							Next:         []Next{},
							ActUsers:      []string{},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnDelvDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_CONFIRMED,
							Status:         STATUS_RETURNED,
							Next:           []Next{},
							ActUsers:        []string{},
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURNED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper1Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnDelvDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_CONFIRMED,
							Status:         STATUS_RETURNED,
							Next:           []Next{},
							ActUsers:        []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURNED,
							},
						},
					},
					{
						role:    KEEPER,
						chid:    td.Keeper2Id_botId,
						notifiy: true,
						brq: BorrowRequest{
							ReturnDelvDate: 1,
							WorkflowType:   WORKFLOW_RETURN,
							Worflow:        []string{STATUS_RETURN_REQUESTED, STATUS_RETURN_CONFIRMED, STATUS_RETURNED},
							LastStatus:     STATUS_RETURN_CONFIRMED,
							Status:         STATUS_RETURNED,
							Next:           []Next{},
							ActUsers:        []string{},
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RETURNED,
							},
						},
					},
				},
			},
		}

		for _, step := range testWorkflow {
			env.realNotifyThreads = map[string]*model.Post{}

			for _, channelId := range []string{
				td.BorChannelId,
				td.BorId_botId,
				worker_botId,
				td.Keeper1Id_botId,
				td.Keeper2Id_botId,
			} {
				postById[createdPid[channelId]] = realbrUpdPosts[channelId]

			}
			var oldPosts map[string]*model.Post
			DeepCopy(&oldPosts, &realbrUpdPosts)

			wfrJson, _ := json.Marshal(step.wfr)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
			baseLineTime := time.Now().Unix()
			plugin.ServeHTTP(nil, w, r)

			res := new(Result)
			json.NewDecoder(w.Result().Body).Decode(&res)
			require.Emptyf(t, res.Error, "response should not has error. err:%v", res.Error)

			// Unnecessary to get again, because map is passed by reference-like,
			// but this work makes it easy to understand
			returnedInfo := getCurrentPosts()
			newPosts := returnedInfo.RealbrUpdPosts
			for index, test := range step.result {
				oldPost := oldPosts[test.chid]
				var oldBorrow Borrow
				json.Unmarshal([]byte(oldPost.Message), &oldBorrow)

				newPost := newPosts[test.chid]
				var newBorrow Borrow
				json.Unmarshal([]byte(newPost.Message), &newBorrow)

				assert.Equalf(t, test.brq.WorkflowType, newBorrow.DataOrImage.WorkflowType,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. workflow type should be %v",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, test.brq.WorkflowType)

				assert.Equalf(t, test.brq.Worflow, newBorrow.DataOrImage.Worflow,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. workflow should be %v",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, test.brq.Worflow)

				assert.Equalf(t, test.brq.LastStatus, newBorrow.DataOrImage.LastStatus,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. last status should be %v",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, test.brq.LastStatus)

				assert.Equalf(t, test.brq.Status, newBorrow.DataOrImage.Status,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. status should be %v",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, test.brq.Status)

				assert.Equalf(t, test.brq.Next, newBorrow.DataOrImage.Next,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. next should be %v",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, test.brq.Next)

				assert.Equalf(t, test.brq.ActUsers, newBorrow.DataOrImage.ActUsers,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. act user should be %v",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, test.brq.ActUsers)

				assert.Equalf(t, test.brq.Tags, newBorrow.DataOrImage.Tags,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. tags should be %v",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, test.brq.Tags)

				if test.brq.ConfirmDate != 0 {
					assert.GreaterOrEqualf(t, newBorrow.DataOrImage.ConfirmDate, baseLineTime,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Confirmed date should be correct",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)

				} else {
					assert.Equalf(t, oldBorrow.DataOrImage.ConfirmDate, newBorrow.DataOrImage.ConfirmDate,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Confirmed date should be same",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				}
				if test.brq.DeliveryDate != 0 {
					assert.GreaterOrEqualf(t, newBorrow.DataOrImage.DeliveryDate, baseLineTime,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Delivery date should be correct",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				} else {
					assert.Equalf(t, oldBorrow.DataOrImage.DeliveryDate, newBorrow.DataOrImage.DeliveryDate,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Delivery date should be same",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				}
				if test.brq.RenewReqDate != 0 {
					assert.GreaterOrEqualf(t, newBorrow.DataOrImage.RenewReqDate, baseLineTime,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Renew requested date should be correct",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				} else {
					assert.Equalf(t, oldBorrow.DataOrImage.RenewReqDate, newBorrow.DataOrImage.RenewReqDate,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Renew requested date should be same",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				}
				if test.brq.RenewConfDate != 0 {
					assert.GreaterOrEqualf(t, newBorrow.DataOrImage.RenewConfDate, baseLineTime,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Renew confirmed date should be correct",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				} else {
					assert.Equalf(t, oldBorrow.DataOrImage.RenewConfDate, newBorrow.DataOrImage.RenewConfDate,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Renew confirmed date should be same",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				}
				if test.brq.ReturnReqDate != 0 {
					assert.GreaterOrEqualf(t, newBorrow.DataOrImage.ReturnReqDate, baseLineTime,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Return requested date should be correct",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				} else {
					assert.Equalf(t, oldBorrow.DataOrImage.ReturnReqDate, newBorrow.DataOrImage.ReturnReqDate,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Return requested date should be same",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				}
				if test.brq.ReturnConfDate != 0 {
					assert.GreaterOrEqualf(t, newBorrow.DataOrImage.ReturnConfDate, baseLineTime,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Return confirmed date should be correct",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				} else {
					assert.Equalf(t, oldBorrow.DataOrImage.ReturnConfDate, newBorrow.DataOrImage.ReturnConfDate,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Return confirmed date should be same",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				}
				if test.brq.ReturnDelvDate != 0 {
					assert.GreaterOrEqualf(t, newBorrow.DataOrImage.ReturnDelvDate, baseLineTime,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Returned date should be correct",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				} else {
					assert.Equalf(t, oldBorrow.DataOrImage.ReturnDelvDate, newBorrow.DataOrImage.ReturnDelvDate,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. Returned date should be same",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)
				}

				oldBorrow.DataOrImage.ConfirmDate = 0
				newBorrow.DataOrImage.ConfirmDate = 0
				oldBorrow.DataOrImage.DeliveryDate = 0
				newBorrow.DataOrImage.DeliveryDate = 0
				oldBorrow.DataOrImage.RenewReqDate = 0
				newBorrow.DataOrImage.RenewReqDate = 0
				oldBorrow.DataOrImage.RenewConfDate = 0
				newBorrow.DataOrImage.RenewConfDate = 0
				oldBorrow.DataOrImage.ReturnReqDate = 0
				newBorrow.DataOrImage.ReturnReqDate = 0
				oldBorrow.DataOrImage.ReturnConfDate = 0
				newBorrow.DataOrImage.ReturnConfDate = 0
				oldBorrow.DataOrImage.ReturnDelvDate = 0
				newBorrow.DataOrImage.ReturnDelvDate = 0
				oldBorrow.DataOrImage.WorkflowType = ""
				newBorrow.DataOrImage.WorkflowType = ""
				oldBorrow.DataOrImage.Worflow = []string{}
				newBorrow.DataOrImage.Worflow = []string{}
				oldBorrow.DataOrImage.LastStatus = ""
				newBorrow.DataOrImage.LastStatus = ""
				oldBorrow.DataOrImage.Status = ""
				newBorrow.DataOrImage.Status = ""
				oldBorrow.DataOrImage.Next = nil
				newBorrow.DataOrImage.Next = nil
				oldBorrow.DataOrImage.ActUsers = nil
				newBorrow.DataOrImage.ActUsers = nil
				oldBorrow.DataOrImage.Tags = nil
				newBorrow.DataOrImage.Tags = nil

				assert.Equalf(t, oldBorrow, newBorrow,
					"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v.the rest fields should not be changed",
					index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role)

				if test.notifiy {
					assert.Containsf(t, env.realNotifyThreads[test.chid].Message, step.wfr.MoveToStatus,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. notification should contians: %v",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, step.wfr.MoveToStatus)
					assert.Containsf(t, env.realNotifyThreads[test.chid].Message, step.wfr.ActUser,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. notification should contians: %v",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, step.wfr.ActUser)
				} else {
					_, ok := env.realNotifyThreads[test.chid]
					assert.Equalf(t, false, ok,
						"index: %v, currentWorkflow: %v, moveToStatus: %v, role: %v. should not have notification ",
						index, step.wfr.CurrentWorkflow, step.wfr.MoveToStatus, test.role, step.wfr.ActUser)
				}

			}
		}

	})

}
func TestLock(t *testing.T) {
	logSwitch = false
	_ = fmt.Println

	var wgall sync.WaitGroup
	var once sync.Once

	start := make(chan struct{})
	end := make(chan struct{})
	startNew := make(chan struct{})

	env := newWorkflowEnv(injectOpt{
		onGetPost: func() {
			once.Do(func() {
				start <- struct{}{}
				<-end
			})
		},
	})

	td := env.td
	api := env.api
	plugin := env.plugin

	createdPid := env.createdPid

	worker := env.worker

	wgall.Add(3)

	go func() {

		req := WorkflowRequest{
			MasterPostKey:   createdPid[td.BorChannelId],
			ActUser:         worker,
			CurrentWorkflow: WORKFLOW_BORROW,
			MoveToWorkflow:  WORKFLOW_BORROW,
			CurrentStatus:   STATUS_REQUESTED,
			MoveToStatus:    STATUS_CONFIRMED,
		}

		wfrJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		plugin.ServeHTTP(nil, w, r)
		startNew <- struct{}{}
		wgall.Done()
	}()

	go func() {
		<-start

		req := WorkflowRequest{
			MasterPostKey:   createdPid[td.BorChannelId],
			ActUser:         worker,
			CurrentWorkflow: WORKFLOW_BORROW,
			MoveToWorkflow:  WORKFLOW_BORROW,
			CurrentStatus:   STATUS_REQUESTED,
			MoveToStatus:    STATUS_CONFIRMED,
		}

		wfrJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		plugin.ServeHTTP(nil, w, r)

		result := w.Result()
		var resultObj *Result
		json.NewDecoder(result.Body).Decode(&resultObj)
		assert.Containsf(t, resultObj.Error, "Failed to lock", "should return lock message")
		api.AssertNumberOfCalls(t, "UpdatePost", 5)
		end <- struct{}{}
		wgall.Done()
	}()

	go func() {
		<-startNew

		req := WorkflowRequest{
			MasterPostKey:   createdPid[td.BorChannelId],
			ActUser:         worker,
			CurrentWorkflow: WORKFLOW_BORROW,
			MoveToWorkflow:  WORKFLOW_BORROW,
			CurrentStatus:   STATUS_REQUESTED,
			MoveToStatus:    STATUS_CONFIRMED,
		}

		wfrJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		plugin.ServeHTTP(nil, w, r)

		result := w.Result()
		var resultObj Result
		json.NewDecoder(result.Body).Decode(&resultObj)
		assert.Equalf(t, resultObj.Error, "", "should normally end")
		api.AssertNumberOfCalls(t, "UpdatePost", 15)
		wgall.Done()
	}()

	wgall.Wait()

}

func TestRollback(t *testing.T) {

	logSwitch = false
	_ = fmt.Println

	env := newWorkflowEnv(injectOpt{
		ifUpdErrCtrl: true,
	})

	td := env.td

	plugin := env.plugin

	var oldPosts map[string]*model.Post
	DeepCopy(&oldPosts, &env.realbrUpdPosts)

	for _, test := range []struct {
		role string
		chid string
	}{
		{
			role: MASTER,
			chid: td.BorChannelId,
		},
		{
			role: BORROWER,
			chid: td.BorId_botId,
		},
		{
			role: LIBWORKER,
			chid: env.worker_botId,
		},
		{
			role: KEEPER,
			chid: td.Keeper1Id_botId,
		},
		{
			role: KEEPER,
			chid: td.Keeper2Id_botId,
		},
	} {
		//reset
		DeepCopy(&env.realbrUpdPosts, &oldPosts)
		env.updErrCtrl = map[string]bool{}
		env.updErrCtrl[test.chid] = true

		req := WorkflowRequest{
			MasterPostKey:   env.createdPid[td.BorChannelId],
			ActUser:         env.worker,
			CurrentWorkflow: WORKFLOW_BORROW,
			MoveToWorkflow:  WORKFLOW_BORROW,
			CurrentStatus:   STATUS_REQUESTED,
			MoveToStatus:    STATUS_CONFIRMED,
		}

		wfrJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		plugin.ServeHTTP(nil, w, r)

		var oldBorrow Borrow
		var newBorrow Borrow

		for cr, oldpost := range oldPosts {

			if cr != test.chid {
				json.Unmarshal([]byte(oldpost.Message), &oldBorrow)
				json.Unmarshal([]byte(env.realbrUpdPosts[oldpost.ChannelId].Message), &newBorrow)
				assert.Equalf(t, oldBorrow, newBorrow, "step: %v, comparing: %v. Should be same as old post", test.role, cr)
			}
		}

	}

}

func TestBorrowRestrict(t *testing.T) {
	logSwitch = false
	_ = fmt.Println

	t.Run("OK_no_requests", func(t *testing.T) {
		env := newWorkflowEnv()
		returned := env.getCurrentPosts()
		resTest := returned.HttpResponse.Result()
		var res Result
		json.NewDecoder(resTest.Body).Decode(&res)
		assert.Equalf(t, "", res.Error, "should be no error")
	})

	t.Run("OK_mixed_with_safe_status", func(t *testing.T) {
		env := newWorkflowEnv(injectOpt{
			onSearchPosts: func(api *plugintest.API, plugin *Plugin, td *TestData) func() {

				safes := []string{
					STATUS_RETURN_CONFIRMED,
					STATUS_RETURNED,
				}

				searched := []*model.Post{}
				for i := 1; i <= plugin.borrowTimes-len(safes)-1; i++ {
					br := Borrow{
						DataOrImage: &BorrowRequest{
							BorrowerUser: td.BorrowUser,
							Status:       STATUS_REQUESTED,
						},
					}
					brj, _ := json.Marshal(br)
					p := &model.Post{
						Message: string(brj),
					}
					searched = append(searched, p)
				}

				for _, safe := range safes {
					br := Borrow{
						DataOrImage: &BorrowRequest{
							BorrowerUser: td.BorrowUser,
							Status:       safe,
						},
					}
					brj, _ := json.Marshal(br)
					p := &model.Post{
						Message: string(brj),
					}
					searched = append(searched, p)
				}

				return func() {
					api.On("SearchPostsInTeam", plugin.team.Id, []*model.SearchParams{
						{
							Terms:     "BORROWER_EQ_" + td.BorrowUser,
							IsHashtag: true,
							InChannels: []string{
								plugin.borrowChannel.Id,
							},
						},
					}).Return(searched, nil)
				}
			},
		},
		)
		returned := env.getCurrentPosts()
		resTest := returned.HttpResponse.Result()
		var res Result
		json.NewDecoder(resTest.Body).Decode(&res)
		assert.Equalf(t, "", res.Error, "should be no error")
	})

	t.Run("error_borrow_limited", func(t *testing.T) {

		env := newWorkflowEnv(injectOpt{
			onSearchPosts: func(api *plugintest.API, plugin *Plugin, td *TestData) func() {

				searched := []*model.Post{}
				for i := 1; i <= plugin.borrowTimes; i++ {
					br := Borrow{
						DataOrImage: &BorrowRequest{
							BorrowerUser: td.BorrowUser,
							Status:       STATUS_REQUESTED,
						},
					}
					brj, _ := json.Marshal(br)
					p := &model.Post{
						Message: string(brj),
					}
					searched = append(searched, p)
				}

				return func() {
					api.On("SearchPostsInTeam", plugin.team.Id, []*model.SearchParams{
						{
							Terms:     "BORROWER_EQ_" + td.BorrowUser,
							IsHashtag: true,
							InChannels: []string{
								plugin.borrowChannel.Id,
							},
						},
					}).Return(searched, nil)
				}
			},
		},
		)
		returned := env.getCurrentPosts()
		resTest := returned.HttpResponse.Result()
		var res Result
		json.NewDecoder(resTest.Body).Decode(&res)
		assert.Equalf(t, "borrowing-book-limited", res.Error, "should be error")
	})

}
