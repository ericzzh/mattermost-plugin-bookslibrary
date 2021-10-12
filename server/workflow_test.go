package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
	td     *TestData
	api    *plugintest.API
	plugin *Plugin
	//created posts, replace previous
	realbrPosts map[string]*model.Post
	//updated posts, replace previous
	realbrUpdPosts    map[string]*model.Post
	realbrDelPosts    map[string]string
	realbrDelPostsSeq []string
	createdPid        map[string]string
	chidByCreatedPid  map[string]string
	worker            string
	worker_botId      string
	postById          map[string]*model.Post
	realNotifyThreads map[string]*model.Post
	getCurrentPosts   func() ReturnedInfo
	updErrCtrl        map[string]bool
	injectedOption    *injectOpt
}

type injectOpt struct {
	onGetPost     func()
	ifUpdErrCtrl  bool
	onSearchPosts func(api *plugintest.API, plugin *Plugin, td *TestData) func()
	invInject     *BookInventory
	onGetPostErr  func(id string) *model.AppError
}

//because some injections need the data generated, so have to make the inject as seperated
func (env *workflowEnv) injectOption(inj *injectOpt) {
	env.injectedOption = inj
}

func newWorkflowEnv(injects ...injectOpt) *workflowEnv {
	var inject injectOpt
	if injects != nil {
		inject = injects[0]
	}
	env := workflowEnv{}

	env.td = NewTestData()
	env.td.ABookInvInjected = inject.invInject

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
		injectOpt.searchPosts = inject.onSearchPosts(env.api, env.plugin, td)
	}

	env.getCurrentPosts = GenerateBorrowRequest(env.td, env.plugin, env.api, injectOpt)
	if env.realbrUpdPosts != nil {
		returnedInfo := env.getCurrentPosts()
		env.realbrPosts = returnedInfo.RealbrPost
		env.createdPid = returnedInfo.CreatedPid
		env.chidByCreatedPid = returnedInfo.ChidByCreatedPid
	} else {
		returnedInfo := env.getCurrentPosts()
		env.realbrUpdPosts = returnedInfo.RealbrUpdPosts
		env.realbrPosts = returnedInfo.RealbrPost
		env.createdPid = returnedInfo.CreatedPid
		env.chidByCreatedPid = returnedInfo.ChidByCreatedPid
	}

	if len(env.realbrPosts) == 0 {
		return &env
	}

	var master Borrow

	json.Unmarshal([]byte(env.realbrUpdPosts[env.td.BorChannelId].Message), &master)
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

	env.realbrDelPosts = map[string]string{}

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
				return env.realbrUpdPosts[env.chidByCreatedPid[id]]
			}, func(id string) *model.AppError {
				if env.injectedOption != nil {
					if env.injectedOption.onGetPostErr != nil {
						return env.injectedOption.onGetPostErr(id)
					}
				}

				return nil
			})

		env.api.On("DeletePost", env.createdPid[channelId]).
			Return(func(id string) *model.AppError {
				env.realbrDelPosts[env.chidByCreatedPid[id]] = id
				env.realbrDelPostsSeq = append(env.realbrDelPostsSeq, id)
				return nil
			})
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
		role                string
		chid                string
		notifiy             bool
		brq                 BorrowRequest
		LastActualStepIndex int
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
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_CONFIRMED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_CONFIRMED,
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
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
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
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
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
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
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
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
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
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
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
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURNED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURNED,
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURNED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_RETURNED,
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURNED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURNED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURNED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURNED,
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURNED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_KEEPER + "kpuser2",
								TAG_PREFIX_STATUS + STATUS_RETURNED,
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
				wf[step.wfr.NextStepIndex].LastActualStepIndex = test.LastActualStepIndex

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
				newBorrow.DataOrImage.Tags = nil
				oldBorrow.DataOrImage.Tags = nil

				//we don't check this filed, leave this to another testing
				oldBorrow.DataOrImage.RenewedTimes = 0
				newBorrow.DataOrImage.RenewedTimes = 0

				assert.Equalf(t, oldBorrow, newBorrow,
					"in step: %v", expStep)

				if test.notifiy {
					assert.Containsf(t, env.realNotifyThreads[test.chid].Message, expStep.Status,
						"in step: %v, role: %v", expStep, test.role)
					assert.Containsf(t, env.realNotifyThreads[test.chid].Message, step.wfr.ActorUser,
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

func TestInvFlow(t *testing.T) {
	logSwitch = true
	_ = fmt.Println

	type testResult struct {
		inv BookInventory
	}

	type testData struct {
		wfr    WorkflowRequest
		result testResult
	}

	t.Run("normal", func(t *testing.T) {

		env := newWorkflowEnv()

		plugin := env.plugin

		createdPid := env.createdPid

		worker := env.worker
		var master Borrow

		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.BorChannelId].Message), &master)
		masterBrq := master.DataOrImage
		wf := plugin._createWFTemplate(masterBrq.Worflow[masterBrq.StepIndex].ActionDate)

		testWorkflow := []testData{
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
				},
				testResult{
					inv: BookInventory{
						Stock:       2,
						TransmitOut: 1,
					},
				},
			},
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
				},
				testResult{
					inv: BookInventory{
						Stock:   2,
						Lending: 1,
					},
				},
			},
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, wf),
				},
				testResult{
					inv: BookInventory{
						Stock:   2,
						Lending: 1,
					},
				},
			},
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
				},
				testResult{
					inv: BookInventory{
						Stock:   2,
						Lending: 1,
					},
				},
			},
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
				},
				testResult{
					inv: BookInventory{
						Stock:   2,
						Lending: 1,
					},
				},
			},
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_RETURN_CONFIRMED, wf),
				},
				testResult{
					inv: BookInventory{
						Stock:      2,
						TransmitIn: 1,
					},
				},
			},
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_RETURNED, wf),
				},
				testResult{
					inv: BookInventory{
						Stock: 3,
					},
				},
			},
		}

		for _, step := range testWorkflow {
			step.wfr.ActorUser = worker
			step.wfr.MasterPostKey = createdPid[env.td.BorChannelId]
			wfrJson, _ := json.Marshal(step.wfr)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
			plugin.ServeHTTP(nil, w, r)

			invPost := env.td.RealBookPostUpd[env.td.BookChIdInv]
			var inv BookInventory
			json.Unmarshal([]byte(invPost.Message), &inv)
			inv.Id = ""
			inv.Name = ""
			inv.Relations = nil
			assert.Equalf(t, step.result.inv, inv, "inventory should be same, at %v", wf[step.wfr.NextStepIndex].Status)

			env.td.ABookInv.Stock = step.result.inv.Stock
			env.td.ABookInv.TransmitOut = step.result.inv.TransmitOut
			env.td.ABookInv.Lending = step.result.inv.Lending
			env.td.ABookInv.TransmitIn = step.result.inv.TransmitIn

		}

	})

	t.Run("be unavailable if unsufficient, available if returned. ", func(t *testing.T) {
		env := newWorkflowEnv(injectOpt{
			invInject: &BookInventory{
				Stock: 1,
			},
		})
		td := env.td
		plugin := env.plugin
		createdPid := env.createdPid

		worker := env.worker
		var master Borrow

		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.BorChannelId].Message), &master)
		masterBrq := master.DataOrImage
		wf := plugin._createWFTemplate(masterBrq.Worflow[masterBrq.StepIndex].ActionDate)

		td.ABookPub.IsAllowedToBorrow = true
		wfrJson, _ := json.Marshal(WorkflowRequest{
			ActorUser:     worker,
			MasterPostKey: createdPid[td.BorChannelId],
			NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		plugin.ServeHTTP(nil, w, r)

		postPub := env.td.RealBookPostUpd[td.BookChIdPub]
		var pub BookPublic
		json.Unmarshal([]byte(postPub.Message), &pub)
		assert.Equalf(t, false, pub.IsAllowedToBorrow, "should be unavailable")

		td.ABookPub.IsAllowedToBorrow = false

		for _, status := range []string{
			STATUS_DELIVIED,
			STATUS_RETURN_REQUESTED,
			STATUS_RETURN_CONFIRMED,
			STATUS_RETURNED,
		} {
			wfrJson, _ := json.Marshal(WorkflowRequest{
				ActorUser:     worker,
				MasterPostKey: createdPid[td.BorChannelId],
				NextStepIndex: _getIndexByStatus(status, wf),
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
			plugin.ServeHTTP(nil, w, r)
		}

		postPub = env.td.RealBookPostUpd[td.BookChIdPub]
		json.Unmarshal([]byte(postPub.Message), &pub)
		assert.Equalf(t, true, pub.IsAllowedToBorrow, "should be available")

	})

}

func TestLock(t *testing.T) {
	logSwitch = false
	_ = fmt.Println

	var wgall sync.WaitGroup
	var once sync.Once

	t.Run("lock borrow request", func(t *testing.T) {
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
		// api := env.api
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
			// api.AssertNumberOfCalls(t, "UpdatePost", 5)
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
			// api.AssertNumberOfCalls(t, "UpdatePost", 15)
			wgall.Done()
		}()

		wgall.Wait()
	})

	t.Run("lock book", func(t *testing.T) {

		env := newWorkflowEnv()

		env.td.block0 = make(chan struct{})
		env.td.block1 = make(chan struct{})

		var wait sync.WaitGroup

		go func() {

			_sendAndCheckATestWFRequest(t, env, STATUS_CONFIRMED, false)

			wait.Done()

		}()

		go func() {
			env.td.block1 <- struct{}{}

			_sendAndCheckATestWFRequest(t, env, STATUS_CONFIRMED, true)

			assert.Equalf(t, 0, len(env.realbrUpdPosts), "should have no br update")
			assert.Equalf(t, 0, len(env.td.RealBookPostUpd), "should have no book update")

			env.td.block0 <- struct{}{}

			wait.Done()

		}()

		wait.Wait()

	})
}

func _sendAndCheckATestWFRequest(t *testing.T, env *workflowEnv, status string, assertError bool) {

	req := WorkflowRequest{
		MasterPostKey: env.createdPid[env.td.BorChannelId],
		ActorUser:     env.worker,
		NextStepIndex: _getIndexByStatus(status, env.td.EmptyWorkflow),
	}

	wfrJson, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
	env.plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	var resultObj Result
	json.NewDecoder(result.Body).Decode(&resultObj)

	if assertError {
		assert.NotEmpty(t, resultObj.Error, "", "should have error")
	} else {
		assert.Equalf(t, resultObj.Error, "", "should normally end")
	}
}

func TestRollback(t *testing.T) {

	logSwitch = false
	_ = fmt.Println

	t.Run("rollback borrow requests", func(t *testing.T) {

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
	})

	t.Run("rollback as book update failed", func(t *testing.T) {
		env := newWorkflowEnv()

		var oldPosts map[string]*model.Post
		DeepCopy(&oldPosts, &env.realbrUpdPosts)

		env.td.updateBookErr = true
		_sendAndCheckATestWFRequest(t, env, STATUS_CONFIRMED, true)

		for _, oldpost := range oldPosts {
			var oldBorrow Borrow
			var newBorrow Borrow

			json.Unmarshal([]byte(env.realbrUpdPosts[oldpost.ChannelId].Message), &newBorrow)
			json.Unmarshal([]byte(oldpost.Message), &oldBorrow)
			assert.Equalf(t, oldBorrow, newBorrow, "all updates to br should be rollback")
		}
	})

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
						Type:    "custom_borrow_type",
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
						Type:    "custom_borrow_type",
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
						Type:    "custom_borrow_type",
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

	t.Run("error_unsufficent_stock", func(t *testing.T) {
		env := newWorkflowEnv(injectOpt{
			invInject: &BookInventory{
				Stock: 0,
			},
		})
		returned := env.getCurrentPosts()
		resTest := returned.HttpResponse.Result()
		var res Result
		json.NewDecoder(resTest.Body).Decode(&res)
		assert.Equalf(t, ErrNoStock.Error(), res.Error, "should be error")

		postPub := env.td.RealBookPostUpd[env.td.BookChIdPub]
		var pub BookPublic
		json.Unmarshal([]byte(postPub.Message), &pub)
		assert.Equalf(t, false, pub.IsAllowedToBorrow, "should be unavailable")
	})

}

func TestRenewTimes(t *testing.T) {

	t.Run("renew until max times", func(t *testing.T) {

		env := newWorkflowEnv()
		env.plugin.maxRenewTimes = 1

		wfrJson, _ := json.Marshal(WorkflowRequest{
			ActorUser:     env.worker,
			MasterPostKey: env.createdPid[env.td.BorChannelId],
			NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, env.td.EmptyWorkflow),
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		env.plugin.ServeHTTP(nil, w, r)

		for _, status := range []string{
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
		} {
			wfrJson, _ := json.Marshal(WorkflowRequest{
				ActorUser:     env.worker,
				MasterPostKey: env.createdPid[env.td.BorChannelId],
				NextStepIndex: _getIndexByStatus(status, env.td.EmptyWorkflow),
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
			env.plugin.ServeHTTP(nil, w, r)
		}

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		assert.Equalf(t, 1, bor.DataOrImage.RenewedTimes, "renewed times should be 1")

		wfrJson, _ = json.Marshal(WorkflowRequest{
			ActorUser:     env.worker,
			MasterPostKey: env.createdPid[env.td.BorChannelId],
			NextStepIndex: _getIndexByStatus(STATUS_RENEW_REQUESTED, env.td.EmptyWorkflow),
		})

		w = httptest.NewRecorder()
		r = httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))

		var oldPosts map[string]*model.Post
		DeepCopy(&oldPosts, &env.realbrUpdPosts)

		env.plugin.ServeHTTP(nil, w, r)
		assert.Equalf(t, oldPosts, env.realbrUpdPosts, "should be no updated")

		res := new(Result)
		json.NewDecoder(w.Result().Body).Decode(&res)
		assert.Equalf(t, ErrRenewLimited.Error(), res.Error, "renew error")

	})

	t.Run("reset confirmed step at second renew time", func(t *testing.T) {

		env := newWorkflowEnv()
		env.plugin.maxRenewTimes = 2

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
			STATUS_RENEW_REQUESTED,
		} {
			wfrJson, _ := json.Marshal(WorkflowRequest{
				ActorUser:     env.worker,
				MasterPostKey: env.createdPid[env.td.BorChannelId],
				NextStepIndex: _getIndexByStatus(status, env.td.EmptyWorkflow),
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
			env.plugin.ServeHTTP(nil, w, r)
			res := new(Result)
			json.NewDecoder(w.Result().Body).Decode(&res)
			assert.Equalf(t, "", res.Error, "should be no error")
		}

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		confirmedStep := bor.DataOrImage.Worflow[_getIndexByStatus(STATUS_RENEW_CONFIRMED, env.td.EmptyWorkflow)]
		assert.Equalf(t, false, confirmedStep.Completed, "completed should be cleard")
		assert.Equalf(t, int64(0), confirmedStep.ActionDate, "action date should be cleared")

	})

}

func TestWorkflowJump(t *testing.T) {

	t.Run("forward cyclic jump", func(t *testing.T) {

		env := newWorkflowEnv()

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		workflow := bor.DataOrImage.Worflow
		reqStepOld := workflow[_getIndexByStatus(STATUS_REQUESTED, workflow)]

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
			STATUS_RETURN_REQUESTED,
			STATUS_RETURN_CONFIRMED,
			STATUS_RETURNED,
			STATUS_REQUESTED,
		} {
			wfrJson, _ := json.Marshal(WorkflowRequest{
				ActorUser:     env.worker,
				MasterPostKey: env.createdPid[env.td.BorChannelId],
				NextStepIndex: _getIndexByStatus(status, env.td.EmptyWorkflow),
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
			env.plugin.ServeHTTP(nil, w, r)
			res := new(Result)
			json.NewDecoder(w.Result().Body).Decode(&res)
			assert.Equalf(t, "", res.Error, "should be no error")
		}

		postBor = env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		workflow = bor.DataOrImage.Worflow
		reqStepNew := workflow[_getIndexByStatus(STATUS_REQUESTED, workflow)]

		assert.NotEqualf(t, reqStepOld, reqStepNew, "the new repeated step should not be equal")

		checked := map[string]struct{}{}

		var checkNextBlank func(s Step)
		checkNextBlank = func(step Step) {
			if step.NextStepIndex == nil {
				return
			}

			checked[step.Status] = struct{}{}

			for _, i := range step.NextStepIndex {
				nextStep := workflow[i]
				if _, ok := checked[nextStep.Status]; ok {
					return
				}
				assert.Emptyf(t, nextStep.ActionDate, "Status:%v, action date should be empty", nextStep.Status)
				assert.Equalf(t, false, nextStep.Completed, "Status:%v, complete status should be false", nextStep.Status)
				checkNextBlank(nextStep)
			}
		}

		checkNextBlank(workflow[0])
	})

	t.Run("backward cyclic jump(reject)", func(t *testing.T) {

		env := newWorkflowEnv()

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		workflow := bor.DataOrImage.Worflow
		reqStepOld := workflow[_getIndexByStatus(STATUS_REQUESTED, workflow)]

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_REQUESTED,
		} {

			wfrJson, _ := json.Marshal(WorkflowRequest{
				ActorUser:     env.worker,
				MasterPostKey: env.createdPid[env.td.BorChannelId],
				NextStepIndex: _getIndexByStatus(status, env.td.EmptyWorkflow),
				Backward:      true,
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
			env.plugin.ServeHTTP(nil, w, r)
			res := new(Result)
			json.NewDecoder(w.Result().Body).Decode(&res)
			assert.Equalf(t, "", res.Error, "should be no error")
		}

		postBor = env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		workflow = bor.DataOrImage.Worflow
		reqStepNew := workflow[_getIndexByStatus(STATUS_REQUESTED, workflow)]

		assert.Equalf(t, reqStepOld, reqStepNew, "the new back returned step should be equal")
	})
}

func TestBorrowDelete(t *testing.T) {
	logSwitch = true
	_ = fmt.Println

	type returnedEnv struct {
		*workflowEnv
		wf []Step
	}

	newEnv := func(injopts ...injectOpt) returnedEnv {
		var env *workflowEnv
		if injopts != nil {
			env = newWorkflowEnv(injopts[0])
		} else {
			env = newWorkflowEnv()
		}

		var master Borrow
		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.BorChannelId].Message), &master)
		masterBrq := master.DataOrImage
		wf := env.plugin._createWFTemplate(masterBrq.Worflow[masterBrq.StepIndex].ActionDate)

		return returnedEnv{
			env, wf,
		}
	}

	performDelete := func(env *workflowEnv, assertError bool) {
		wfrJson, _ := json.Marshal(WorkflowRequest{
			ActorUser:     env.worker,
			MasterPostKey: env.createdPid[env.td.BorChannelId],
			Delete:        true,
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		env.plugin.ServeHTTP(nil, w, r)

		res := new(Result)
		json.NewDecoder(w.Result().Body).Decode(&res)

		if assertError {
			assert.NotEmptyf(t, res.Error, "response should has error. err:%v", res.Error)
		} else {
			assert.Emptyf(t, res.Error, "response should not has error. err:%v", res.Error)
		}
	}

	performNext := func(env *workflowEnv, status string, assertError bool) {

		req := WorkflowRequest{
			MasterPostKey: env.createdPid[env.td.BorChannelId],
			ActorUser:     env.worker,
			NextStepIndex: _getIndexByStatus(STATUS_CONFIRMED, env.td.EmptyWorkflow),
		}

		wfrJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		env.plugin.ServeHTTP(nil, w, r)

		res := new(Result)
		json.NewDecoder(w.Result().Body).Decode(&res)

		if assertError {
			require.NotEmptyf(t, res.Error, "response should has error. err:%v", res.Error)
		} else {
			require.Emptyf(t, res.Error, "response should not has error. err:%v", res.Error)
		}
	}

	t.Run("all steps test.", func(t *testing.T) {

		env := newEnv()

		performDelete(env.workflowEnv, false)

		for _, step := range []struct {
			status      string
			assertError bool
		}{
			{
				STATUS_CONFIRMED,
				false,
			},
			{
				STATUS_DELIVIED,
				true,
			},
			{
				STATUS_RENEW_REQUESTED,
				true,
			},
			{
				STATUS_RENEW_CONFIRMED,
				true,
			},
			{
				STATUS_RETURN_REQUESTED,
				true,
			},
			{
				STATUS_RETURN_CONFIRMED,
				true,
			},
			{
				STATUS_RETURNED,
				false,
			},
		} {
			performNext(env.workflowEnv, step.status, false)

			performDelete(env.workflowEnv, false)
		}

	})

	t.Run("delete confirmed", func(t *testing.T) {

		getInv := func(env *workflowEnv) *BookInventory {
			invPost := env.td.RealBookPostUpd[env.td.BookChIdInv]
			inv := &BookInventory{}
			json.Unmarshal([]byte(invPost.Message), inv)
			return inv
		}

		env := newEnv()
		performNext(env.workflowEnv, STATUS_CONFIRMED, false)
		inv := getInv(env.workflowEnv)
		assert.Equalf(t, 2, inv.Stock, "stock")
		assert.Equalf(t, 1, inv.TransmitOut, "stock")
		performDelete(env.workflowEnv, false)
		inv = getInv(env.workflowEnv)
		assert.Equalf(t, 3, inv.Stock, "stock")
		assert.Equalf(t, 0, inv.TransmitOut, "stock")
	})

	t.Run("delete broken data", func(t *testing.T) {
		env := newEnv()

		for _, channelId := range []string{
			env.td.BorChannelId,
			env.td.BorId_botId,
			env.worker_botId,
			env.td.Keeper1Id_botId,
			env.td.Keeper2Id_botId,
		} {
			env.injectOption(&injectOpt{
				onGetPostErr: func(id string) *model.AppError {
					if env.chidByCreatedPid[id] == channelId {
						return model.NewAppError("GetSinglePost", "app.post.get.app_error", nil, "", http.StatusNotFound)
					}
					return nil
				},
			})

			env.realbrDelPosts = map[string]string{}
			env.realbrDelPostsSeq = []string{}

			if channelId == env.td.BorChannelId {
				performDelete(env.workflowEnv, true)
			} else {
				performDelete(env.workflowEnv, false)
			}

			if channelId == env.td.BorChannelId {
				assert.Equalf(t, 0, len(env.realbrDelPosts), "master error is fatal error, no deletion should be performed")
			} else {
				assert.Equalf(t, 4, len(env.realbrDelPosts), "should be deleted, except the error one")
			}

			_, ok := env.realbrDelPosts[channelId]
			assert.Equalf(t, false, ok, "should be deleted false")

		}
	})

	t.Run("delete sequence", func(t *testing.T) {
		env := newEnv()
		performDelete(env.workflowEnv, false)
		seqLen := len(env.realbrDelPostsSeq)
		assert.Equalf(t, env.createdPid[env.td.BorChannelId], env.realbrDelPostsSeq[seqLen-1], "last should be borrow channel")
		assert.Equalf(t, env.createdPid[env.worker_botId], env.realbrDelPostsSeq[seqLen-2], "last second should be borrow channel")

	})

}
