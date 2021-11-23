package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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
	//for pass API mock matching
	createdPid_1      map[string]string
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
	bookInjectOpt *bookInjectOptions
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

	if inject.bookInjectOpt != nil {
		env.td = NewTestData(*inject.bookInjectOpt)
	} else {
		env.td = NewTestData()
	}
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

		matchFun := func(channelId string) func(string) bool {
			return func(id string) bool {
				matchId, ok := env.createdPid_1[channelId]
				if !ok {
					matchId = env.createdPid[channelId]
				}
				return matchId == id
			}
		}
		env.api.On("DeletePost", mock.MatchedBy(
			matchFun(channelId))).Return(
			func(id string) *model.AppError {
				env.realbrDelPosts[env.chidByCreatedPid[id]] = id
				env.realbrDelPostsSeq = append(env.realbrDelPostsSeq, id)
				return nil
			})
		env.api.On("CreatePost", mock.MatchedBy(matchThreadByChannel(channelId))).
			Run(saveNotifiyThread).Return(&model.Post{}, nil)
	}

	env.createdPid_1 = map[string]string{}

	return &env
}

func TestWorkflowHandle(t *testing.T) {
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
		delete              bool
		relationKeys        *RelationKeys
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
			}, {
				WorkflowRequest{
					MasterPostKey: createdPid[td.BorChannelId],
					ActorUser:     td.ABook.KeeperUsers[0],
					NextStepIndex: _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
					ChosenCopyId:  "zzh-book-001 b1",
				},
				[]testResult{
					{
						role:                MASTER,
						chid:                td.BorChannelId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						relationKeys: &RelationKeys{
							Book:      td.BookPostIdPub,
							Borrower:  createdPid[td.BorId_botId],
							Libworker: createdPid[worker_botId],
							Keepers: []string{
								createdPid[td.Keeper1Id_botId],
							},
						},
						brq: BorrowRequest{
							StepIndex:    _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
							KeeperUsers:  []string{"kpuser1"},
							KeeperNames:  []string{"kpname1"},
							ChosenCopyId: "zzh-book-001 b1",
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_STATUS + STATUS_KEEPER_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             false,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex:    _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
							ChosenCopyId: "zzh-book-001 b1",
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_KEEPER_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex:    _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
							KeeperUsers:  []string{"kpuser1"},
							KeeperNames:  []string{"kpname1"},
							ChosenCopyId: "zzh-book-001 b1",
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_STATUS + STATUS_KEEPER_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex:    _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
							KeeperUsers:  []string{"kpuser1"},
							KeeperNames:  []string{"kpname1"},
							ChosenCopyId: "zzh-book-001 b1",
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_STATUS + STATUS_KEEPER_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper2Id_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_CONFIRMED, wf),
						delete:              true,
						brq:                 BorrowRequest{},
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
						LastActualStepIndex: _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                BORROWER,
						chid:                td.BorId_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                LIBWORKER,
						chid:                worker_botId,
						notifiy:             true,
						LastActualStepIndex: _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_BORROWER + td.BorrowUser,
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						LastActualStepIndex: _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_DELIVIED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_STATUS + STATUS_DELIVIED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RENEW_REQUESTED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RENEW_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
							},
						},
					},
					{
						role:                KEEPER,
						chid:                td.Keeper1Id_botId,
						notifiy:             false,
						LastActualStepIndex: _getIndexByStatus(STATUS_RENEW_CONFIRMED, wf),
						brq: BorrowRequest{
							StepIndex: _getIndexByStatus(STATUS_RETURN_REQUESTED, wf),
							Tags: []string{
								TAG_PREFIX_LIBWORKER + worker,
								TAG_PREFIX_KEEPER + "kpuser1",
								TAG_PREFIX_STATUS + STATUS_RETURN_REQUESTED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURN_CONFIRMED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURNED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURNED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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
								TAG_PREFIX_STATUS + STATUS_RETURNED,
								TAG_PREFIX_COPYID + "zzh-book-001_b1",
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

				if test.delete {
					assert.Equalf(t, createdPid[test.chid], env.realbrDelPosts[test.chid], "this post should be deleted, role:%v", test.role)
					continue
				}

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

				if test.brq.KeeperUsers != nil {
					assert.Equalf(t, test.brq.KeeperUsers, newBorrow.DataOrImage.KeeperUsers,
						"in step: %v, role: %v", expStep, test.role)
					assert.Equalf(t, test.brq.KeeperNames, newBorrow.DataOrImage.KeeperNames,
						"in step: %v, role: %v", expStep, test.role)
					newBorrow.DataOrImage.KeeperUsers = nil
					oldBorrow.DataOrImage.KeeperUsers = nil
					newBorrow.DataOrImage.KeeperNames = nil
					oldBorrow.DataOrImage.KeeperNames = nil
				}

				if test.brq.ChosenCopyId != "" {
					assert.Equalf(t, test.brq.ChosenCopyId, newBorrow.DataOrImage.ChosenCopyId,
						"in step: %v, role: %v", expStep, test.role)
					newBorrow.DataOrImage.ChosenCopyId = ""
					oldBorrow.DataOrImage.ChosenCopyId = ""
				}

				if test.relationKeys != nil {
					assert.Equalf(t, *test.relationKeys, newBorrow.RelationKeys,
						"in step: %v, role: %v", expStep, test.role)
					newBorrow.RelationKeys = RelationKeys{}
					oldBorrow.RelationKeys = RelationKeys{}
				}

				newBorrow.DataOrImage.Worflow = nil
				oldBorrow.DataOrImage.Worflow = nil
				newBorrow.DataOrImage.StepIndex = -1
				oldBorrow.DataOrImage.StepIndex = -1
				newBorrow.DataOrImage.Tags = nil
				oldBorrow.DataOrImage.Tags = nil

				//we don't check this filed, leave this to another testing
				oldBorrow.DataOrImage.RenewedTimes = 0
				newBorrow.DataOrImage.RenewedTimes = 0

                                //because the non-master part will be recontructed every time
                                //the order is not granteened
                                sort.Strings(oldBorrow.RelationKeys.Keepers)
                                sort.Strings(newBorrow.RelationKeys.Keepers)
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

func getActor(env *workflowEnv, status string) string {
	switch status {
	case STATUS_CONFIRMED:
		return env.worker
	case STATUS_KEEPER_CONFIRMED:
		if env.worker != env.td.ABookPri.KeeperUsers[0] {
			return env.td.ABookPri.KeeperUsers[0]
		} else {
			return env.td.ABookPri.KeeperUsers[1]
		}
	case STATUS_DELIVIED:
		return env.td.BorrowUser
	case STATUS_RENEW_REQUESTED:
		return env.td.BorrowUser
	case STATUS_RENEW_CONFIRMED:
		return env.worker
	case STATUS_RETURN_REQUESTED:
		return env.td.BorrowUser
	case STATUS_RETURN_CONFIRMED:
		return env.worker
	case STATUS_RETURNED:
		return env.td.ABookPri.KeeperUsers[0]
	}

	return ""
}

type performNextOption struct {
	chosen       string
	backward     bool
	errorMessage string
}

func performNext(t *testing.T, env *workflowEnv, status string, assertError bool, opt performNextOption) {

	var chosen string
	if status == STATUS_KEEPER_CONFIRMED {
		chosen = opt.chosen
	}
	req := WorkflowRequest{
		MasterPostKey: env.createdPid[env.td.BorChannelId],
		ActorUser:     getActor(env, status),
		NextStepIndex: _getIndexByStatus(status, env.td.EmptyWorkflow),
		ChosenCopyId:  chosen,
		Backward:      opt.backward,
	}

	wfrJson, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
	env.plugin.ServeHTTP(nil, w, r)

	res := new(Result)
	json.NewDecoder(w.Result().Body).Decode(&res)

	if assertError {
		assert.NotEmptyf(t, res.Error, "response should has error. err:%v", res.Error)
	} else {
		assert.Emptyf(t, res.Error, "response should not has error. err:%v", res.Error)
		if opt.errorMessage != "" {
			assert.Containsf(t, res.Error, opt.errorMessage, "should contain message:%v", opt.errorMessage)
		}
	}
}
func TestWorkflowInvFlow(t *testing.T) {
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
						Stock:       3,
						TransmitOut: 0,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
					},
				},
			},
			{
				WorkflowRequest{
					NextStepIndex: _getIndexByStatus(STATUS_KEEPER_CONFIRMED, wf),
					ChosenCopyId:  "zzh-book-001 b1",
				},
				testResult{
					inv: BookInventory{
						Stock:       2,
						TransmitOut: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_TRANSOUT},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
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
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
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
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
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
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
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
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
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
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_TRANSIN},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
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
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
							"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
					},
				},
			},
		}

		for _, step := range testWorkflow {
			step.wfr.ActorUser = getActor(env, wf[step.wfr.NextStepIndex].Status)
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
			env.td.ABook.Copies = step.result.inv.Copies

		}

	})

	t.Run("error if unsufficient, confirmed", func(t *testing.T) {
		env := newWorkflowEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPri = &BookPrivate{
			KeeperUsers: []string{"kpuser1"},
			KeeperNames: []string{"kpname1"},
			CopyKeeperMap: map[string]Keeper{
				"zzh-book-001 b1": {User: "kpuser1"},
			},
		}

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
		//confirm should precheck stock
		performNext(t, env, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		//keeper confirm actual reduce stock
		performNext(t, env, STATUS_CONFIRMED, true, performNextOption{})

	})

	t.Run("error if unsufficient, keeper confirmed", func(t *testing.T) {
		env := newWorkflowEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPri = &BookPrivate{
			KeeperUsers: []string{"kpuser1"},
			KeeperNames: []string{"kpname1"},
			CopyKeeperMap: map[string]Keeper{
				"zzh-book-001 b1": {User: "kpuser1"},
			},
		}

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
		//confirm should precheck stock
		performNext(t, env, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		//keeper confirm actual reduce stock
		performNext(t, env, STATUS_KEEPER_CONFIRMED, true, performNextOption{})

	})

	t.Run("error if choose a not instock copy", func(t *testing.T) {
		env := newWorkflowEnv()

		env.td.ABookInv = &BookInventory{
			Stock: 2,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
				"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_TRANSIN},
			},
		}
		env.td.ABookPri.CopyKeeperMap = map[string]Keeper{
			"zzh-book-001 b1": {User: "kpuser1"},
			"zzh-book-001 b2": {User: "kpuser2"},
		}

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
		performNext(t, env, STATUS_KEEPER_CONFIRMED, true, performNextOption{chosen: "zzh-book-001 b2"})

	})

	t.Run("be unavailable if unsufficient, available if returned. ", func(t *testing.T) {
		env := newWorkflowEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPri = &BookPrivate{
			KeeperUsers: []string{"kpuser1"},
			KeeperNames: []string{"kpname1"},
			CopyKeeperMap: map[string]Keeper{
				"zzh-book-001 b1": {User: "kpuser1"},
			},
		}

		env.td.ABookPub.IsAllowedToBorrow = true

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
		performNext(t, env, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})

		postPub := env.td.RealBookPostUpd[env.td.BookChIdPub]
		var pub BookPublic
		json.Unmarshal([]byte(postPub.Message), &pub)
		assert.Equalf(t, false, pub.IsAllowedToBorrow, "should be unavailable")
		assert.Equalf(t, "无库存", pub.ReasonOfDisallowed, "should be text of no-stock")

		env.td.ABookPub.IsAllowedToBorrow = false

		for _, status := range []string{
			STATUS_DELIVIED,
			STATUS_RETURN_REQUESTED,
			STATUS_RETURN_CONFIRMED,
			STATUS_RETURNED,
		} {
			performNext(t, env, status, false, performNextOption{})
		}

		postPub = env.td.RealBookPostUpd[env.td.BookChIdPub]
		json.Unmarshal([]byte(postPub.Message), &pub)
		assert.Equalf(t, true, pub.IsAllowedToBorrow, "should be available")
		assert.Equalf(t, "", pub.ReasonOfDisallowed, "should be cleared")
	})

	t.Run("no toggling on if manually disallowed", func(t *testing.T) {

		env := newWorkflowEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPri = &BookPrivate{
			KeeperUsers: []string{"kpuser1"},
			KeeperNames: []string{"kpname1"},
			CopyKeeperMap: map[string]Keeper{
				"zzh-book-001 b1": {User: "kpuser1"},
			},
		}
		env.td.ABookPub.IsAllowedToBorrow = false
		env.td.ABookPub.ManuallyDisallowed = true

		for _, status := range []string{
			STATUS_KEEPER_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RETURN_REQUESTED,
			STATUS_RETURN_CONFIRMED,
			STATUS_RETURNED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
		}

		var pub BookPublic
		postPub := env.td.RealBookPostUpd[env.td.BookChIdPub]
		json.Unmarshal([]byte(postPub.Message), &pub)
		assert.Equalf(t, false, pub.IsAllowedToBorrow, "should not be available")
		assert.Equalf(t, true, pub.ManuallyDisallowed, "manully disallowed should not be affected")

	})

}

func TestWorkflowRevert(t *testing.T) {

	type testResult struct {
		inv        BookInventory
		brq        BorrowRequest
		renewTimes int
	}

	type testData struct {
		status string
		result testResult
	}

	t.Run("normal reverted", func(t *testing.T) {

		env := newWorkflowEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPri.CopyKeeperMap = map[string]Keeper{
			"zzh-book-001 b1": {User: "kpuser1"},
		}

		//move to last initially
		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_KEEPER_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
			STATUS_RETURN_REQUESTED,
			STATUS_RETURN_CONFIRMED,
			STATUS_RETURNED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
		}

		testWorkflow := []testData{
			{
				STATUS_RETURN_CONFIRMED,
				testResult{
					inv: BookInventory{
						TransmitIn: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_TRANSIN},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "zzh-book-001 b1",
					},
					renewTimes: 1,
				},
			},
			{
				STATUS_RETURN_REQUESTED,
				testResult{
					inv: BookInventory{
						Lending: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "zzh-book-001 b1",
					},
					renewTimes: 1,
				},
			},
			{
				STATUS_RENEW_CONFIRMED,
				testResult{
					inv: BookInventory{
						Lending: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "zzh-book-001 b1",
					},
					renewTimes: 1,
				},
			},
			{
				STATUS_RENEW_REQUESTED,
				testResult{
					inv: BookInventory{
						Lending: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "zzh-book-001 b1",
					},
					renewTimes: 0,
				},
			},
			{
				STATUS_DELIVIED,
				testResult{
					inv: BookInventory{
						Lending: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_LENDING},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "zzh-book-001 b1",
					},
					renewTimes: 0,
				},
			},
			{
				STATUS_KEEPER_CONFIRMED,
				testResult{
					inv: BookInventory{
						TransmitOut: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_TRANSOUT},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "zzh-book-001 b1",
					},
					renewTimes: 0,
				},
			},
			{
				STATUS_CONFIRMED,
				testResult{
					inv: BookInventory{
						Stock: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "",
					},
					renewTimes: 0,
				},
			},
			{
				STATUS_REQUESTED,
				testResult{
					inv: BookInventory{
						Stock: 1,
						Copies: BookCopies{
							"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
						},
					},
					brq: BorrowRequest{
						ChosenCopyId: "",
					},
					renewTimes: 0,
				},
			},
		}

		for _, step := range testWorkflow {

			performNext(t, env, step.status, false, performNextOption{chosen: "zzh-book-001 b1", backward: true})

			var bor Borrow
			postBor := env.realbrUpdPosts[env.td.BorChannelId]
			json.Unmarshal([]byte(postBor.Message), &bor)

			//Inventory check
			invPost := env.td.RealBookPostUpd[env.td.BookChIdInv]
			var inv BookInventory
			json.Unmarshal([]byte(invPost.Message), &inv)
			inv.Id = ""
			inv.Name = ""
			inv.Relations = nil
			assert.Equalf(t, step.result.inv, inv, "inventory should be same, at %v", step.status)

			if step.result.inv.Stock == 0 {
				assert.Equalf(t, false, env.td.ABookPub.IsAllowedToBorrow, "should not be allowed to borrow")
				assert.Equalf(t, "无库存", env.td.ABookPub.ReasonOfDisallowed, "should be text of no-stock")

			} else {
				assert.Equalf(t, true, env.td.ABookPub.IsAllowedToBorrow, "should be allowed to borrow")
				assert.Equalf(t, "", env.td.ABookPub.ReasonOfDisallowed, "should be cleared")
			}

			//ChosenCopyId Check
			assert.Equalf(t, step.result.brq.ChosenCopyId, bor.DataOrImage.ChosenCopyId, "chosen id is not correct")

			//Renew times check
			assert.Equalf(t, step.result.renewTimes, bor.DataOrImage.RenewedTimes, "renew times is not correct")
		}

	})

	t.Run("normal reverted, no toggling", func(t *testing.T) {

		env := newWorkflowEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPri.CopyKeeperMap = map[string]Keeper{
			"zzh-book-001 b1": {User: "kpuser1"},
		}

		//move to last initially
		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_KEEPER_CONFIRMED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
		}

		env.td.ABookPub.IsAllowedToBorrow = false
		env.td.ABookPub.ManuallyDisallowed = true

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_REQUESTED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1", backward: true})
		}
		assert.Equalf(t, false, env.td.ABookPub.IsAllowedToBorrow, "still should not be allowed to borrow")
		assert.Equalf(t, true, env.td.ABookPub.ManuallyDisallowed, "still should be true")
	})

	t.Run("revert through keeper_confirmed, restoring another keeper", func(t *testing.T) {

		env := newWorkflowEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 2,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
				"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPri.CopyKeeperMap = map[string]Keeper{
			"zzh-book-001 b1": {User: "kpuser1"},
			"zzh-book-001 b2": {User: "kpuser2"},
		}

		oldKp2Post := env.realbrUpdPosts[env.td.Keeper2Id_botId]

		//move to last initially
		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_KEEPER_CONFIRMED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
		}

		env.createdPid[env.td.Keeper2Id_botId] = model.NewId()
		for _, status := range []string{
			STATUS_CONFIRMED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1", backward: true})
		}

		newKp2Post := env.realbrUpdPosts[env.td.Keeper2Id_botId]
		assert.NotEqualf(t, newKp2Post.Id, oldKp2Post.Id, "2 keeper2'sid should not be equal")

		masterPost := env.realbrUpdPosts[env.td.BorChannelId]

		var br Borrow
		json.Unmarshal([]byte(masterPost.Message), &br)
		assert.Containsf(t, br.RelationKeys.Keepers, newKp2Post.Id, "new keeper2 should be in master's relation")

	})
}

func TestWorkflowLock(t *testing.T) {
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

		wgall.Add(3)

		go func() {

			performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
			startNew <- struct{}{}
			wgall.Done()
		}()

		go func() {
			<-start

			performNext(t, env, STATUS_CONFIRMED, true, performNextOption{
				chosen:       "zzh-book-001 b1",
				errorMessage: "Failed to lock"})

			end <- struct{}{}
			wgall.Done()
		}()

		go func() {
			<-startNew

			performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})

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

			performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})

			wait.Done()

		}()

		go func() {
			env.td.block1 <- struct{}{}

			performNext(t, env, STATUS_CONFIRMED, true, performNextOption{})

			assert.Equalf(t, 0, len(env.realbrUpdPosts), "should have no br update")
			assert.Equalf(t, 0, len(env.td.RealBookPostUpd), "should have no book update")

			env.td.block0 <- struct{}{}

			wait.Done()

		}()

		wait.Wait()

	})

	t.Run("lock with same role", func(*testing.T) {

		env := newWorkflowEnv(injectOpt{
			bookInjectOpt: &bookInjectOptions{
				keepersAsLibworkers: true,
			},
		})

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})

	})
}

func TestWorkflowRollback(t *testing.T) {

	logSwitch = false
	_ = fmt.Println

	t.Run("rollback borrow requests", func(t *testing.T) {

		env := newWorkflowEnv(injectOpt{
			ifUpdErrCtrl: true,
		})

		td := env.td

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

			performNext(t, env, STATUS_CONFIRMED, true, performNextOption{})

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
		performNext(t, env, STATUS_CONFIRMED, true, performNextOption{})

		for _, oldpost := range oldPosts {
			var oldBorrow Borrow
			var newBorrow Borrow

			json.Unmarshal([]byte(env.realbrUpdPosts[oldpost.ChannelId].Message), &newBorrow)
			json.Unmarshal([]byte(oldpost.Message), &oldBorrow)
			assert.Equalf(t, oldBorrow, newBorrow, "all updates to br should be rollback")
		}
	})

	t.Run("rollback create", func(t *testing.T) {

		env := newWorkflowEnv()

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
		performNext(t, env, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})

		var oldDelPosts map[string]string
		DeepCopy(&oldDelPosts, &env.realbrDelPosts)

		var oldPosts map[string]*model.Post
		DeepCopy(&oldPosts, &env.realbrUpdPosts)

		env.td.updateBorrowErr[env.td.BorChannelId] = true
		newKeeper2PostId := model.NewId()
		env.createdPid[env.td.Keeper2Id_botId] = newKeeper2PostId
		env.chidByCreatedPid[newKeeper2PostId] = env.td.Keeper2Id_botId
		performNext(t, env, STATUS_CONFIRMED, true, performNextOption{backward: true})

		newDelPosts := env.realbrDelPosts
		newPosts := env.realbrUpdPosts

		assert.Equal(t, newDelPosts[env.td.Keeper2Id_botId], newPosts[env.td.Keeper2Id_botId].Id, "create id and delete id should be same")

	})

	t.Run("rollback delete", func(t *testing.T) {

		env := newWorkflowEnv()

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})

		var oldPosts map[string]*model.Post
		DeepCopy(&oldPosts, &env.realbrUpdPosts)

		env.td.updateBorrowErr[env.td.BorChannelId] = true
		env.createdPid_1[env.td.Keeper2Id_botId] = env.createdPid[env.td.Keeper2Id_botId]
		newKeeper2PostId := model.NewId()
		env.createdPid[env.td.Keeper2Id_botId] = newKeeper2PostId
		env.chidByCreatedPid[newKeeper2PostId] = env.td.Keeper2Id_botId
		performNext(t, env, STATUS_KEEPER_CONFIRMED, true, performNextOption{chosen: "zzh-book-001 b1"})

		newPosts := env.realbrUpdPosts
		newCreatedPosts := env.realbrPosts

		assert.NotEqualf(t, newCreatedPosts[env.td.Keeper2Id_botId].Id, oldPosts[env.td.Keeper2Id_botId].Id, "should create a new keeper post")

		var newKeeper2Borrow Borrow
		json.Unmarshal([]byte(newPosts[env.td.Keeper2Id_botId].Message), &newKeeper2Borrow)
		var oldKeeper2Borrow Borrow
		json.Unmarshal([]byte(oldPosts[env.td.Keeper2Id_botId].Message), &oldKeeper2Borrow)

		assert.Equal(t, newKeeper2Borrow, oldKeeper2Borrow, "rollbacked keeper borrow should be same")

		var oldMasterBorrow Borrow
		json.Unmarshal([]byte(oldPosts[env.td.BorChannelId].Message), &oldMasterBorrow)

		var newMasterBorrow Borrow
		json.Unmarshal([]byte(newPosts[env.td.BorChannelId].Message), &newMasterBorrow)

		assert.Equal(t, 2, len(newMasterBorrow.RelationKeys.Keepers), "2 keepers")
		assert.Contains(t, newMasterBorrow.RelationKeys.Keepers, env.createdPid[env.td.Keeper1Id_botId], "keeper1 should be in master")
		assert.Contains(t, newMasterBorrow.RelationKeys.Keepers, env.createdPid[env.td.Keeper2Id_botId], "new keeper2 should be in master")

		newMasterBorrow.RelationKeys.Keepers = nil
		oldMasterBorrow.RelationKeys.Keepers = nil
		assert.Equal(t, newMasterBorrow, oldMasterBorrow, "the rest master borrow should be same")

	})

}

func TestWorkflowBorrowRestrict(t *testing.T) {
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
							Terms:     TAG_PREFIX_BORROWER + td.BorrowUser,
							IsHashtag: true,
							InChannels: []string{
								plugin.borrowChannel.Name,
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
							Terms:     TAG_PREFIX_BORROWER + td.BorrowUser,
							IsHashtag: true,
							InChannels: []string{
								plugin.borrowChannel.Name,
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
		assert.Equalf(t, env.plugin.i18n.GetText(ErrBorrowingLimited.Error()), res.Error, "should be error")
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
		assert.Equalf(t, env.plugin.i18n.GetText(ErrNoStock.Error()), res.Error, "should be error")

		postPub := env.td.RealBookPostUpd[env.td.BookChIdPub]
		var pub BookPublic
		json.Unmarshal([]byte(postPub.Message), &pub)
		assert.Equalf(t, false, pub.IsAllowedToBorrow, "should be unavailable")
	})

}

func TestWorkflowRenewTimes(t *testing.T) {

	t.Run("renew until max times", func(t *testing.T) {

		env := newWorkflowEnv()
		env.plugin.maxRenewTimes = 1

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_KEEPER_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
		}

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		assert.Equalf(t, 1, bor.DataOrImage.RenewedTimes, "renewed times should be 1")

		var oldPosts map[string]*model.Post
		DeepCopy(&oldPosts, &env.realbrUpdPosts)

		performNext(t, env, STATUS_RENEW_REQUESTED, true, performNextOption{
			chosen:       "zzh-book-001 b1",
			errorMessage: ErrRenewLimited.Error()})

		assert.Equalf(t, oldPosts, env.realbrUpdPosts, "should be no updated")

	})

	t.Run("reset confirmed step at second renew time", func(t *testing.T) {

		env := newWorkflowEnv()
		env.plugin.maxRenewTimes = 2

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_KEEPER_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
			STATUS_RENEW_REQUESTED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
		}

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		confirmedStep := bor.DataOrImage.Worflow[_getIndexByStatus(STATUS_RENEW_CONFIRMED, env.td.EmptyWorkflow)]
		assert.Equalf(t, false, confirmedStep.Completed, "completed should be cleard")
		assert.Equalf(t, int64(0), confirmedStep.ActionDate, "action date should be cleared")

	})

	t.Run("backward movement, renew times should not be updated", func(t *testing.T) {

		env := newWorkflowEnv()

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_KEEPER_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
			STATUS_RETURN_REQUESTED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
		}

		performNext(t, env, STATUS_RENEW_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1", backward: true})

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		assert.Equalf(t, 1, bor.DataOrImage.RenewedTimes, "backward, renew times should not be updated.")

	})

}

func TestWorkflowJump(t *testing.T) {
	//------------------------------
	//Jump doesn't mean revert
	//just make a possibility
	//and test the worflow clearning logic
	//------------------------------

	t.Run("forward cyclic jump", func(t *testing.T) {

		env := newWorkflowEnv()

		var bor Borrow
		postBor := env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		workflow := bor.DataOrImage.Worflow
		reqStepOld := workflow[_getIndexByStatus(STATUS_REQUESTED, workflow)]

		for _, status := range []string{
			STATUS_CONFIRMED,
			STATUS_KEEPER_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_RENEW_REQUESTED,
			STATUS_RENEW_CONFIRMED,
			STATUS_RETURN_REQUESTED,
			STATUS_RETURN_CONFIRMED,
			STATUS_RETURNED,
			STATUS_REQUESTED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1"})
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
			STATUS_KEEPER_CONFIRMED,
			STATUS_DELIVIED,
			STATUS_REQUESTED,
		} {
			performNext(t, env, status, false, performNextOption{chosen: "zzh-book-001 b1", backward: true})
		}

		postBor = env.realbrUpdPosts[env.td.BorChannelId]
		json.Unmarshal([]byte(postBor.Message), &bor)
		workflow = bor.DataOrImage.Worflow
		reqStepNew := workflow[_getIndexByStatus(STATUS_REQUESTED, workflow)]

		assert.Equalf(t, reqStepOld, reqStepNew, "the new back returned step should be equal")
	})
}

type updatedBorrowCallback struct {
	master    func(br *Borrow)
	libworker func(br *Borrow)
	borrower  func(br *Borrow)
	keeper1   func(br *Borrow)
	keeper2   func(br *Borrow)
}

func getUpdatedBorrows(env *workflowEnv, cb updatedBorrowCallback) {

	if cb.master != nil {
		var borrow Borrow
		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.BorChannelId].Message), &borrow)
		cb.master(&borrow)
	}

	if cb.libworker != nil {
		var borrow Borrow
		json.Unmarshal([]byte(env.realbrUpdPosts[env.worker_botId].Message), &borrow)
		cb.libworker(&borrow)
	}

	if cb.borrower != nil {
		var borrow Borrow
		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.BorId_botId].Message), &borrow)
		cb.borrower(&borrow)
	}

	if cb.keeper1 != nil {
		var borrow Borrow
		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.Keeper1Id_botId].Message), &borrow)
		cb.borrower(&borrow)
	}

	if cb.keeper2 != nil {
		var borrow Borrow
		json.Unmarshal([]byte(env.realbrUpdPosts[env.td.Keeper2Id_botId].Message), &borrow)
		cb.borrower(&borrow)
	}
}
func TestWorkflowBorrowDelete(t *testing.T) {
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

	performDelete := func(env *workflowEnv, status string, assertError bool) {
		wfrJson, _ := json.Marshal(WorkflowRequest{
			ActorUser:     getActor(env, status),
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

	t.Run("all steps test.", func(t *testing.T) {

		env := newEnv()

		performDelete(env.workflowEnv, STATUS_REQUESTED, false)

		for _, step := range []struct {
			status      string
			assertError bool
		}{
			{
				STATUS_CONFIRMED,
				false,
			},
			{
				STATUS_KEEPER_CONFIRMED,
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
			performNext(t, env.workflowEnv, step.status, false, performNextOption{chosen: "zzh-book-001 b1"})

			performDelete(env.workflowEnv, step.status, step.assertError)
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
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		performNext(t, env.workflowEnv, STATUS_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		inv := getInv(env.workflowEnv)
		assert.Equalf(t, 1, inv.Stock, "stock")
		assert.Equalf(t, "", env.td.ABookPub.ReasonOfDisallowed, "reason should be set")
		performDelete(env.workflowEnv, STATUS_CONFIRMED, false)
		inv = getInv(env.workflowEnv)
		assert.Equalf(t, 1, inv.Stock, "stock")
		assert.Equalf(t, true, env.td.ABookPub.IsAllowedToBorrow, "stock should be available")
		assert.Equalf(t, "", env.td.ABookPub.ReasonOfDisallowed, "reason should be cleared")
	})

	t.Run("delete keeper confirmed", func(t *testing.T) {

		getInv := func(env *workflowEnv) *BookInventory {
			invPost := env.td.RealBookPostUpd[env.td.BookChIdInv]
			inv := &BookInventory{}
			json.Unmarshal([]byte(invPost.Message), inv)
			return inv
		}

		env := newEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		performNext(t, env.workflowEnv, STATUS_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		performNext(t, env.workflowEnv, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		performDelete(env.workflowEnv, STATUS_KEEPER_CONFIRMED, false)

		inv := getInv(env.workflowEnv)
		assert.Equalf(t, 1, inv.Stock, "stock")
		assert.Equalf(t, 0, inv.TransmitOut, "stock")
		assert.Equalf(t, true, env.td.ABookPub.IsAllowedToBorrow, "stock should be available")
		assert.Equalf(t, "", env.td.ABookPub.ReasonOfDisallowed, "reason should be cleared")
		assert.Equalf(t, COPY_STATUS_INSTOCK, inv.Copies["zzh-book-001 b1"].Status, "should return to instock")
		//no need for deletion
		// getUpdatedBorrows(env.workflowEnv, updatedBorrowCallback{
		// 	master: func(br *Borrow) {
		// 		assert.Emptyf(t, br.DataOrImage.ChosenCopyId, "master's chosen id should empty")
		// 	},
		// 	libworker: func(br *Borrow) {
		// 		assert.Emptyf(t, br.DataOrImage.ChosenCopyId, "libworker's chosen id should empty")
		// 	},
		// 	borrower: func(br *Borrow) {
		// 		assert.Emptyf(t, br.DataOrImage.ChosenCopyId, "borrower's chosen id should empty")
		// 	},
		// 	keeper1: func(br *Borrow) {
		// 		assert.Emptyf(t, br.DataOrImage.ChosenCopyId, "keeper1's chosen id should empty")
		// 	},
		// 	keeper2: func(br *Borrow) {
		// 		assert.Emptyf(t, br.DataOrImage.ChosenCopyId, "keeper2's chosen id should empty")
		// 	},
		// })
	})

	t.Run("delete, not toggling", func(t *testing.T) {

		env := newEnv()
		env.td.ABookInv = &BookInventory{
			Stock: 1,
			Copies: BookCopies{
				"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			},
		}
		env.td.ABookPub.IsAllowedToBorrow = false
		env.td.ABookPub.ManuallyDisallowed = true
		performNext(t, env.workflowEnv, STATUS_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		performNext(t, env.workflowEnv, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		performDelete(env.workflowEnv, STATUS_KEEPER_CONFIRMED, false)
		assert.Equalf(t, false, env.td.ABookPub.IsAllowedToBorrow, "stock should be still not available")
		assert.Equalf(t, true, env.td.ABookPub.ManuallyDisallowed, "manually disallowed should be changed")
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
				performDelete(env.workflowEnv, STATUS_REQUESTED, true)
			} else {
				performDelete(env.workflowEnv, STATUS_REQUESTED, false)
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
		performDelete(env.workflowEnv, STATUS_REQUESTED, false)
		seqLen := len(env.realbrDelPostsSeq)
		assert.Equalf(t, env.createdPid[env.td.BorChannelId], env.realbrDelPostsSeq[seqLen-1], "last should be borrow channel")
		assert.Equalf(t, env.createdPid[env.worker_botId], env.realbrDelPostsSeq[seqLen-2], "last second should be borrow channel")

	})

}

func TestMultiRoles(t *testing.T) {

	t.Run("keeper confirmed", func(t *testing.T) {

		env := newWorkflowEnv(injectOpt{
			bookInjectOpt: &bookInjectOptions{
				keepersAsLibworkers: true,
			},
		})

		getUpdatedBorrows(env, updatedBorrowCallback{
			master: func(br *Borrow) {
				assert.Containsf(t, br.RelationKeys.Keepers, env.createdPid[env.worker_botId], "should contains work post id")
			},
		})

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
		//the actor keeper won't be the worker, but the keeper(worker) shouldn't be deleted
		performNext(t, env, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})
		assert.Equalf(t, 0, len(env.realbrDelPosts[env.worker_botId]), "keeper should not be deleted.")

		getUpdatedBorrows(env, updatedBorrowCallback{
			master: func(br *Borrow) {
				assert.Equalf(t, 1, len(br.RelationKeys.Keepers), "keeper relation should be deleted")
				assert.NotContainsf(t, br.RelationKeys.Keepers, env.createdPid[env.worker_botId], "should not contains work post id")
			},
		})

	})

	t.Run("keeper confirmed back", func(t *testing.T) {

		env := newWorkflowEnv(injectOpt{
			bookInjectOpt: &bookInjectOptions{
				keepersAsLibworkers: true,
			},
		})

		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{})
		//the actor keeper won't be the worker, but the keeper(worker) shouldn't be deleted
		performNext(t, env, STATUS_KEEPER_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1"})

		getUpdatedBorrows(env, updatedBorrowCallback{
			master: func(br *Borrow) {
				assert.NotContainsf(t, br.RelationKeys.Keepers, env.createdPid[env.worker_botId], "should not contains work post id")
			},
		})
		env.realbrPosts[env.worker_botId] = nil
		performNext(t, env, STATUS_CONFIRMED, false, performNextOption{chosen: "zzh-book-001 b1", backward: true})

		assert.Emptyf(t, env.realbrPosts[env.worker_botId], "should not create keeper")

		getUpdatedBorrows(env, updatedBorrowCallback{
			master: func(br *Borrow) {
				assert.Equalf(t, 2, len(br.RelationKeys.Keepers), "keeper relation should be appended")
				assert.Containsf(t, br.RelationKeys.Keepers, env.createdPid[env.worker_botId], "should contains work post id")
			},
		})

	})
}
