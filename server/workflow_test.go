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

	env.api = env.td.ApiMockCommon()
	env.plugin = env.td.NewMockPlugin()
	env.plugin.SetAPI(env.api)
        env.td.EmptyWorkflow = env.plugin._createWFTemplate(GetNowTime())

	td := env.td

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
	logSwitch = true
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

		var master Borrow

		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.BorChannelId].Message), &master)
		masterBrq := master.DataOrImage
                wf := plugin._createWFTemplate(masterBrq.Worflow[masterBrq.StepIndex].ActionDate)

		testWorkflow := []testData{
			{
				WorkflowRequest{
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     worker,
					NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
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
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     td.BorrowUser,
					NextStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_DELIVIED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_DELIVIED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_DELIVIED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_DELIVIED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_DELIVIED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper2Id_botId,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_DELIVIED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_DELIVIED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     td.BorrowUser,
					NextStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper2Id_botId,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_REQUESTED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     worker,
					NextStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role: KEEPER,
						chid: td.Keeper2Id_botId,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							Tags: []string{
								"#LIBWORKERUSER_EQ_" + worker,
								"#KEEPERUSER_EQ_" + "kpuser1",
								"#KEEPERUSER_EQ_" + "kpuser2",
								"#STATUS_EQ_" + STATUS_RENEW_CONFIRMED,
							},
						},
					},
				},
			}, {
				WorkflowRequest{
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     td.BorrowUser,
					NextStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
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
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     worker,
					NextStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
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
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     td.ABook.KeeperUsers[0],
					NextStepIndex: _getIndexByStatus(STATUS_RETURNED, wf),
				},
				[]testResult{
					{
						role:    MASTER,
						chid:    td.BorChannelId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RETURNED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURNED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							Tags: []string{
								"#BORROWERUSER_EQ_" + td.BorrowUser,
								"#LIBWORKERUSER_EQ_" + worker,
								"#STATUS_EQ_" + STATUS_RETURNED,
							},
						},
					},
					{
						role:    LIBWORKER,
						chid:    worker_botId,
						notifiy: true,
						brq: BorrowRequest{
							StepIndex:     _getIndexByStatus(STATUS_RETURNED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURNED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
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
							StepIndex:     _getIndexByStatus(STATUS_RETURNED, wf),
							LastStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
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

			// The workflow
			var master Borrow
			json.Unmarshal([]byte(newPosts[td.BorChannelId].Message), &master)
			wf[step.wfr.NextStepIndex].ActionDate =
				master.DataOrImage.Worflow[master.DataOrImage.StepIndex].ActionDate
			wf[step.wfr.NextStepIndex].Completed = true

			for _, test := range step.result {
				oldPost := oldPosts[test.chid]
				var oldBorrow Borrow
				json.Unmarshal([]byte(oldPost.Message), &oldBorrow)

				newPost := newPosts[test.chid]
				var newBorrow Borrow
				json.Unmarshal([]byte(newPost.Message), &newBorrow)

				expStep := &wf[test.brq.StepIndex]
				actStep := &newBorrow.DataOrImage.Worflow[newBorrow.DataOrImage.StepIndex]

				assert.Equalf(t, test.brq.StepIndex, newBorrow.DataOrImage.StepIndex,
					"in step: %v, role: %v", expStep, test.role)

				assert.Equalf(t, test.brq.LastStepIndex, newBorrow.DataOrImage.LastStepIndex,
					"in step: %v, role: %v", expStep, test.role)

				assert.GreaterOrEqualf(t, actStep.ActionDate, baseLineTime,
					"in step: %v, role: %v", expStep, test.role)

				assert.Equalf(t, wf, newBorrow.DataOrImage.Worflow,
					"in step: %v, role: %v", expStep, test.role)

				assert.Equalf(t, test.brq.Tags, newBorrow.DataOrImage.Tags,
					"in step: %v, role: %v", expStep, test.role)

				newBorrow.DataOrImage.Worflow = nil
				oldBorrow.DataOrImage.Worflow = nil
				newBorrow.DataOrImage.StepIndex = -1
				oldBorrow.DataOrImage.StepIndex = -1
				newBorrow.DataOrImage.LastStepIndex = -1
				oldBorrow.DataOrImage.LastStepIndex = -1
				newBorrow.DataOrImage.Tags = nil
				oldBorrow.DataOrImage.Tags = nil

				assert.Equalf(t, oldBorrow, newBorrow,
					"in step: %v", expStep)

				if test.notifiy {
					assert.Containsf(t, env.realNotifyThreads[test.chid].Message, expStep.Status,
						"in step: %v, role: %v", expStep, test.role)
					assert.Containsf(t, env.realNotifyThreads[test.chid].Message, _getUserByRole(expStep.ActorRole, &td, worker),
						"in step: %v, role: %v", expStep, test.role)
				} else {
					_, ok := env.realNotifyThreads[test.chid]
					assert.Equalf(t, false, ok,
						"in step: %v, role: %v", expStep, test.role)
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
			MasterPostKey: createdPid[td.BorChannelId],
			ActorUser:     worker,
			NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, env.td.EmptyWorkflow),
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
			MasterPostKey: createdPid[td.BorChannelId],
			ActorUser:     worker,
			NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, env.td.EmptyWorkflow),
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
			MasterPostKey: createdPid[td.BorChannelId],
			ActorUser:     worker,
			NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, env.td.EmptyWorkflow),
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
			MasterPostKey: env.createdPid[td.BorChannelId],
			ActorUser:     env.worker,
			NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, env.td.EmptyWorkflow),
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
							Worflow:      td.EmptyWorkflow,
							StepIndex:    _getIndexByStatus(STATUS_REQUESTED, td.EmptyWorkflow),
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
							Worflow:      td.EmptyWorkflow,
							StepIndex:    _getIndexByStatus(safe, td.EmptyWorkflow),
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
							Worflow:      td.EmptyWorkflow,
							StepIndex:    _getIndexByStatus(STATUS_REQUESTED, td.EmptyWorkflow),
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
