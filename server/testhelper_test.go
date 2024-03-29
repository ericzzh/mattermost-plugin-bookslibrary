package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	// "github.com/stretchr/testify/require"
)

var logSwitch bool

type mockapiOptons struct {
	excludeBookUpdAPI bool
}
type TestData struct {
	ABook              *Book
	ABookPub           *BookPublic
	ABookPri           *BookPrivate
	ABookInvInjected   *BookInventory
	ABookInv           *BookInventory
	BookPostIdPub      string
	BookPostIdPri      string
	BookPostIdInv      string
	BookChIdPub        string
	BookChIdPri        string
	BookChIdInv        string
	BookPidToChid      map[string]string
	RealBookPostUpd    map[string]*model.Post
	RealBookPostDel    map[string]string
	BotId              string
	BorChannelId       string
	BorTeamId          string
	ABookJson          []byte
	BorrowUser         string
	ReqKey             BorrowRequestKey
	ReqKeyJson         []byte
	BorId              string
	Worker1Id          string
	Worker2Id          string
	Keeper1Id          string
	Keeper2Id          string
	BorId_botId        string
	Worker1Id_botId    string
	Worker2Id_botId    string
	Keeper1Id_botId    string
	Keeper2Id_botId    string
	EmptyWorkflow      []Step
	ApiMockCommon      func(...mockapiOptons) *plugintest.API
	NewMockPlugin      func() *Plugin
	MatchPostByChannel func(string) func(*model.Post) bool
	MatchPostById      func(string) func(*model.Post) bool
	block0             chan struct{}
	block1             chan struct{}
	updateBookErr      bool
	updateBorrowErr    map[string]bool
}
type bookInjectOptions struct {
	keepersAsLibworkers bool
}

func NewTestData(bookInject ...bookInjectOptions) *TestData {
	var inject bookInjectOptions
	if bookInject != nil {
		inject = bookInject[0]
	}

	_ = fmt.Printf
	td := &TestData{}

	td.BookPostIdPub = model.NewId()
	td.BookPostIdPri = model.NewId()
	td.BookPostIdInv = model.NewId()

	td.BookChIdPub = model.NewId()
	td.BookChIdPri = model.NewId()
	td.BookChIdInv = model.NewId()

	td.BookPidToChid = map[string]string{
		td.BookPostIdPub: td.BookChIdPub,
		td.BookPostIdPri: td.BookChIdPri,
		td.BookPostIdInv: td.BookChIdInv,
	}

	td.BotId = model.NewId()
	td.BorChannelId = model.NewId()
	td.BorTeamId = model.NewId()

	td.ABookPub = &BookPublic{
		Id:                "zzh-book-001",
		Name:              "a test book",
		NameEn:            "a test book",
		Category1:         "C1",
		Category2:         "C2",
		Category3:         "C3",
		Author:            "zzh",
		AuthorEn:          "zzh",
		Translator:        "eric",
		TranslatorEn:      "eric",
		Publisher:         "pub1",
		PublisherEn:       "pub1En",
		PublishDate:       "20200821",
		LibworkerUsers:    []string{"worker1", "worker2"},
		LibworkerNames:    []string{"wkname1", "wkname2"},
		IsAllowedToBorrow: true,
		Tags:              []string{},
		MatchId:           model.NewId(),
		Relations: Relations{
			REL_BOOK_PRIVATE:   td.BookPostIdPri,
			REL_BOOK_INVENTORY: td.BookPostIdInv,
		},
	}

	keeperUsers := []string{"kpuser1", "kpuser2"}
	keeperInfos := KeeperInfoMap{
		"kpuser1": {"kpname1"},
		"kpuser2": {"kpname2"},
	}

	var copyKeeperMap map[string]Keeper
	copyKeeperMap = map[string]Keeper{
		"zzh-book-001 b1": {User: "kpuser1"},
		"zzh-book-001 b2": {User: "kpuser1"},
		"zzh-book-001 b3": {User: "kpuser2"},
	}

	if inject.keepersAsLibworkers {
		keeperUsers = td.ABookPub.LibworkerUsers
		keeperInfos = KeeperInfoMap{}
		for i, user := range keeperUsers {
			keeperInfos[user] = KeeperInfo{td.ABookPub.LibworkerNames[i]}
		}
		copyKeeperMap = map[string]Keeper{
			"zzh-book-001 b1": {User: td.ABookPub.LibworkerUsers[0]},
			"zzh-book-001 b2": {User: td.ABookPub.LibworkerUsers[0]},
			"zzh-book-001 b3": {User: td.ABookPub.LibworkerUsers[1]},
		}
	}

	td.ABookPri = &BookPrivate{
		Id:            "zzh-book-001",
		Name:          "a test book",
		KeeperUsers:   keeperUsers,
		KeeperInfos:   keeperInfos,
		CopyKeeperMap: copyKeeperMap,
		Relations: Relations{
			REL_BOOK_PUBLIC: td.BookPostIdPub,
		},
	}

	td.ABookInv = &BookInventory{
		Id:    "zzh-book-001",
		Name:  "a test book",
		Stock: 3,
		Copies: BookCopies{
			"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
		},
		Relations: Relations{
			REL_BOOK_PUBLIC: td.BookPostIdPub,
		},
	}

	td.ABook = &Book{
		td.ABookPub,
		td.ABookPri,
		td.ABookInv,
		nil,
	}

	td.BorrowUser = "bor"

	td.ReqKey = BorrowRequestKey{
		BookPostId:   td.BookPostIdPub,
		BorrowerUser: td.BorrowUser,
	}
	td.ReqKeyJson, _ = json.Marshal(td.ReqKey)

	td.BorId = model.NewId()
	td.Worker1Id = model.NewId()
	td.Worker2Id = model.NewId()
	td.Keeper1Id = model.NewId()
	td.Keeper2Id = model.NewId()
	if inject.keepersAsLibworkers {
		td.Keeper1Id = td.Worker1Id
		td.Keeper2Id = td.Worker2Id
	}
	td.BorId_botId = model.NewId()
	td.Worker1Id_botId = model.NewId()
	td.Worker2Id_botId = model.NewId()
	td.Keeper1Id_botId = model.NewId()
	td.Keeper2Id_botId = model.NewId()
	if inject.keepersAsLibworkers {
		td.Keeper1Id_botId = td.Worker1Id_botId
		td.Keeper2Id_botId = td.Worker2Id_botId
	}

	td.ApiMockCommon = func(options ...mockapiOptons) *plugintest.API {
		var option mockapiOptons
		if options != nil {
			option = options[0]
		}
		api := &plugintest.API{}

		api.On("GetUserByUsername", "bor").Return(&model.User{
			Id:        td.BorId,
			LastName:  "book",
			FirstName: "bor",
		}, nil)

		api.On("GetUserByUsername", "worker1").Return(&model.User{
			Id:        td.Worker1Id,
			LastName:  "wk",
			FirstName: "name1",
		}, nil)

		api.On("GetUserByUsername", "worker2").Return(&model.User{
			Id:        td.Worker2Id,
			LastName:  "wk",
			FirstName: "name2",
		}, nil)

		api.On("GetUserByUsername", "kpuser1").Return(&model.User{
			Id:        td.Keeper1Id,
			LastName:  "kp",
			FirstName: "name1",
		}, nil)

		api.On("GetUserByUsername", "kpuser2").Return(&model.User{
			Id:        td.Keeper2Id,
			LastName:  "kp",
			FirstName: "name2",
		}, nil)

		api.On("GetDirectChannel", td.BorId, td.BotId).Return(&model.Channel{
			Id: td.BorId_botId,
		}, nil)

		api.On("GetDirectChannel", td.Worker1Id, td.BotId).Return(&model.Channel{
			Id: td.Worker1Id_botId,
		}, nil)

		api.On("GetDirectChannel", td.Worker2Id, td.BotId).Return(&model.Channel{
			Id: td.Worker2Id_botId,
		}, nil)

		api.On("GetDirectChannel", td.Keeper1Id, td.BotId).Return(&model.Channel{
			Id: td.Keeper1Id_botId,
		}, nil)

		api.On("GetDirectChannel", td.Keeper2Id, td.BotId).Return(&model.Channel{
			Id: td.Keeper2Id_botId,
		}, nil)

		api.On("GetChannelMember", td.BorChannelId, mock.AnythingOfType("string")).Return(
			func(channelid string, userid string) *model.ChannelMember {
				if userid == td.Worker1Id {
					return &model.ChannelMember{}
				}
				return nil
			}, func(channelid string, userid string) *model.AppError {
				if userid == td.Worker1Id {
					return nil
				}
				return model.NewAppError("GetChannelMember", "app.channel.get_member.missing.app_error", nil, "", http.StatusNotFound)
			})

		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).
			Return().
			Run(func(args mock.Arguments) {
				if logSwitch {
					fmt.Printf("LOG ERROR: %v, %v, %v\n",
						args.String(0),
						args.String(1),
						args.String(2),
					)
				}
			})
		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).
			Return().
			Run(func(args mock.Arguments) {

				if logSwitch {
					fmt.Printf("LOG ERROR: %v, %v, %v, %v, %v\n",
						args.String(0),
						args.String(1),
						args.String(2),
						args.String(3),
						args.String(4),
					)
				}
			})

		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).
			Return().
			Run(func(args mock.Arguments) {
				if logSwitch {
					fmt.Printf("LOG ERROR: %v, %v, %v, %v, %v, %v, %v\n",
						args.String(0),
						args.String(1),
						args.String(2),
						args.String(3),
						args.String(4),
						args.String(5),
						args.String(6),
					)
				}
			})

		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("[]string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).
			Return().
			Run(func(args mock.Arguments) {
				if logSwitch {
					fmt.Printf("LOG ERROR: %v, %v, %v, %v, %v, %v, %v\n",
						args.String(0),
						args.String(1),
						args.String(2),
						args.String(3),
						args.Get(4).([]string),
						args.String(5),
						args.String(6),
					)
				}
			})
		api.On("LogInfo",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything).Return()
		api.On("LogInfo",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything).Return()
		// if option.includeDeleteAnything {
		// 	api.On("DeletePost", mock.AnythingOfType("string")).Return(nil)
		// }:noh

		//------------------------------
		//Books Mock
		//------------------------------
		var once sync.Once
		api.On("GetPost", td.BookPostIdPub).Return(
			func(id string) *model.Post {
				bookPubJson, _ := json.Marshal(td.ABookPub)
				if td.block0 != nil {
					once.Do(func() {
						if td.block1 != nil {
							td.block1 <- struct{}{}
						}
						td.block0 <- struct{}{}
					})
				}
				return &model.Post{
					Id:        td.BookPostIdPub,
					ChannelId: td.BookChIdPub,
					Message:   string(bookPubJson),
				}
			}, nil)

		api.On("GetPost", td.BookPostIdPri).Return(
			func(id string) *model.Post {
				bookPriJson, _ := json.Marshal(td.ABookPri)
				return &model.Post{
					Id:        td.BookPostIdPri,
					ChannelId: td.BookChIdPri,
					Message:   string(bookPriJson),
				}
			}, nil)

		api.On("GetPost", td.BookPostIdInv).Return(
			func(id string) *model.Post {

				bookInvJson, _ := json.Marshal(td.ABookInv)

				if td.ABookInvInjected != nil {
					bookInvJsonInj, _ := json.Marshal(td.ABookInvInjected)
					return &model.Post{
						Id:        td.BookPostIdInv,
						ChannelId: td.BookChIdInv,
						Message:   string(bookInvJsonInj),
					}
				}

				return &model.Post{
					Id:        td.BookPostIdInv,
					ChannelId: td.BookChIdInv,
					Message:   string(bookInvJson),
				}
			}, nil)
		if !option.excludeBookUpdAPI {

			pluginConfig := td.NewMockPlugin()
			td.RealBookPostUpd = map[string]*model.Post{}

			for _, ch := range []struct {
				pid  string
				chid string
			}{
				{
					td.BookPostIdPub,
					pluginConfig.booksChannel.Id,
				},
				{

					td.BookPostIdPri,
					pluginConfig.booksPriChannel.Id,
				},
				{
					td.BookPostIdInv,
					pluginConfig.booksInvChannel.Id,
				},
			} {
				api.On("CreatePost", mock.MatchedBy(td.MatchPostByChannel(ch.chid))).Return(
					func(post *model.Post) *model.Post {
						return &model.Post{
							Id: ch.pid,
						}
					}, nil)
				api.On("UpdatePost", mock.MatchedBy(td.MatchPostByChannel(ch.chid))).Return(
					func(post *model.Post) *model.Post {
						chid := td.BookPidToChid[post.Id]
						td.RealBookPostUpd[chid] = post
						switch chid {
						case td.BookChIdPub:
							pub := &BookPublic{}
							json.Unmarshal([]byte(post.Message), pub)
							td.ABookPub = pub
						case td.BookChIdPri:
							pri := &BookPrivate{}
							json.Unmarshal([]byte(post.Message), pri)
							td.ABookPri = pri
						case td.BookChIdInv:
							inv := &BookInventory{}
							json.Unmarshal([]byte(post.Message), inv)
							td.ABookInv = inv
						}

						return &model.Post{}
					}, func(post *model.Post) *model.AppError {
						if td.updateBookErr {
							return &model.AppError{}
						}
						return nil
					})

				//because chid:pid is 1:1, so we can use pid directly
				api.On("DeletePost", ch.pid).Return(
					func(id string) *model.AppError {
						td.RealBookPostDel[td.BookPidToChid[id]] = id
						return nil
					})
			}
		}

		return api

	}

	td.NewMockPlugin = func() *Plugin {
		i18n, _ := NewI18n("zh")
		return &Plugin{
			botID: td.BotId,
			booksChannel: &model.Channel{
				Id: td.BookChIdPub,
			},
			booksPriChannel: &model.Channel{
				Id: td.BookChIdPri,
			},
			booksInvChannel: &model.Channel{
				Id: td.BookChIdInv,
			},
			borrowChannel: &model.Channel{
				Id: td.BorChannelId,
			},
			team: &model.Team{
				Id: td.BorTeamId,
			},
			borrowTimes:   2,
			maxRenewTimes: 2,
			expiredDays:   30,
			i18n:          i18n,
		}
	}

	td.MatchPostByChannel = func(channelId string) func(*model.Post) bool {
		return func(post *model.Post) bool {
			return post.ChannelId == channelId && post.RootId == ""
		}
	}

	td.MatchPostById = func(PostId string) func(*model.Post) bool {
		return func(post *model.Post) bool {
			return post.Id == PostId && post.RootId == ""
		}
	}

	td.updateBorrowErr = map[string]bool{}
	return td
}

type InjectOptions struct {
	updatePost  func()
	searchPosts func()
}

type ReturnedInfo struct {
	RealbrPost       map[string]*model.Post
	RealbrUpdPosts   map[string]*model.Post
	CreatedPid       map[string]string
	ChidByCreatedPid map[string]string
	HttpResponse     *httptest.ResponseRecorder
}

func GenerateBorrowRequest(td *TestData, plugin *Plugin, api *plugintest.API, injects ...InjectOptions) func() ReturnedInfo {

	var injectOpt InjectOptions
	if injects != nil {
		injectOpt = injects[0]
	}

	realbrPosts := map[string]*model.Post{}
	realbrUpdPosts := map[string]*model.Post{}
	matchPost := td.MatchPostByChannel

	runfn := func(args mock.Arguments) {
		realbrPost := args.Get(0).(*model.Post)
		realbrPosts[realbrPost.ChannelId] = realbrPost
	}

	runfnUpd := func(args mock.Arguments) {
		realbrPost := args.Get(0).(*model.Post)
		// fmt.Printf("****IN RunfnUpd %v\n", realbrPost)
		realbrUpdPosts[realbrPost.ChannelId] = realbrPost
	}

	createdPid := map[string]string{
		plugin.borrowChannel.Id: model.NewId(),
		td.BorId_botId:          model.NewId(),
		td.Worker1Id_botId:      model.NewId(),
		td.Worker2Id_botId:      model.NewId(),
		td.Keeper1Id_botId:      model.NewId(),
		td.Keeper2Id_botId:      model.NewId(),
	}

	chidByCreatedPid := map[string]string{}
	for chid, pid := range createdPid {
		chidByCreatedPid[pid] = chid
	}

	api.On("CreatePost", mock.MatchedBy(matchPost(plugin.borrowChannel.Id))).Run(runfn).
		Return(func(post *model.Post) *model.Post {
			realbrPost := &model.Post{
				Id:        createdPid[plugin.borrowChannel.Id],
				ChannelId: plugin.borrowChannel.Id,
				UserId:    td.BotId,
				Type:      "custom_borrow_type",
				Message:   post.Message,
			}

			var createdPost model.Post
			DeepCopy(&createdPost, realbrPost)

			realbrPosts[plugin.borrowChannel.Id] = &createdPost

			return realbrPost
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.BorId_botId))).Run(runfn).
		Return(func(post *model.Post) *model.Post {
			realbrPost := &model.Post{
				Id:        createdPid[td.BorId_botId],
				ChannelId: td.BorId_botId,
				UserId:    td.BotId,
				Type:      "custom_borrow_type",
				Message:   post.Message,
			}
			var createdPost model.Post
			DeepCopy(&createdPost, realbrPost)
			realbrPosts[td.BorId_botId] = &createdPost
			return realbrPost
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Worker1Id_botId))).Run(runfn).
		Return(func(post *model.Post) *model.Post {

			realbrPost := &model.Post{
				Id:        createdPid[td.Worker1Id_botId],
				ChannelId: td.Worker1Id_botId,
				UserId:    td.BotId,
				Type:      "custom_borrow_type",
				Message:   post.Message,
			}
			var createdPost model.Post
			DeepCopy(&createdPost, realbrPost)
			realbrPosts[td.Worker1Id_botId] = &createdPost
			return realbrPost
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Worker2Id_botId))).Run(runfn).
		Return(func(post *model.Post) *model.Post {
			realbrPost := &model.Post{
				Id:        createdPid[td.Worker2Id_botId],
				ChannelId: td.Worker2Id_botId,
				UserId:    td.BotId,
				Type:      "custom_borrow_type",
				Message:   post.Message,
			}

			var createdPost model.Post
			DeepCopy(&createdPost, realbrPost)
			realbrPosts[td.Worker2Id_botId] = &createdPost
			return realbrPost
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Keeper1Id_botId))).Run(runfn).
		Return(func(post *model.Post) *model.Post {
			realbrPost := &model.Post{
				Id:        createdPid[td.Keeper1Id_botId],
				ChannelId: td.Keeper1Id_botId,
				UserId:    td.BotId,
				Type:      "custom_borrow_type",
				Message:   post.Message,
			}
			var createdPost model.Post
			DeepCopy(&createdPost, realbrPost)
			realbrPosts[td.Keeper1Id_botId] = &createdPost
			return realbrPost
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Keeper2Id_botId))).Run(runfn).
		Return(func(post *model.Post) *model.Post {
			realbrPost := &model.Post{
				Id:        createdPid[td.Keeper2Id_botId],
				ChannelId: td.Keeper2Id_botId,
				UserId:    td.BotId,
				Type:      "custom_borrow_type",
				Message:   post.Message,
			}
			var createdPost model.Post
			DeepCopy(&createdPost, realbrPost)
			realbrPosts[td.Keeper2Id_botId] = &createdPost
			return realbrPost
		}, nil)

	if injectOpt.updatePost != nil {
		injectOpt.updatePost()
	}

	api.On("UpdatePost", mock.MatchedBy(matchPost(plugin.borrowChannel.Id))).Run(runfnUpd).
		Return(
			func(post *model.Post) *model.Post {
				realbrUpdPosts[plugin.borrowChannel.Id] = post
				return post
			}, func(post *model.Post) *model.AppError {
				if _, ok := td.updateBorrowErr[td.BorChannelId]; ok {
					return &model.AppError{}
				}
				return nil

			})
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.BorId_botId))).Run(runfnUpd).
		Return(
			func(post *model.Post) *model.Post {
				realbrUpdPosts[td.BorId_botId] = post
				return post
			},
			nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Worker1Id_botId))).Run(runfnUpd).
		Return(
			func(post *model.Post) *model.Post {
				realbrUpdPosts[td.Worker1Id_botId] = post
				return post
			},
			nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Worker2Id_botId))).Run(runfnUpd).
		Return(
			func(post *model.Post) *model.Post {
				realbrUpdPosts[td.Worker2Id_botId] = post
				return post
			},
			nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Keeper1Id_botId))).Run(runfnUpd).
		Return(
			func(post *model.Post) *model.Post {
				realbrUpdPosts[td.Keeper1Id_botId] = post
				return post
			},
			nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Keeper2Id_botId))).Run(runfnUpd).
		Return(
			func(post *model.Post) *model.Post {
				realbrUpdPosts[td.Keeper2Id_botId] = post
				return post
			},
			nil)

	if injectOpt.searchPosts != nil {
		injectOpt.searchPosts()
	} else {
		api.On("SearchPostsInTeam", plugin.team.Id, mock.AnythingOfType("[]*model.SearchParams")).
			Return([]*model.Post{}, nil)

	}

	w := httptest.NewRecorder()
	reqkey := td.ReqKey
	reqkeyJson, _ := json.Marshal(reqkey)
	r := httptest.NewRequest("POST", "/borrow", bytes.NewReader(reqkeyJson))
	plugin.ServeHTTP(nil, w, r)

	return func() ReturnedInfo {

		return ReturnedInfo{
			RealbrPost:       realbrPosts,
			RealbrUpdPosts:   realbrUpdPosts,
			CreatedPid:       createdPid,
			ChidByCreatedPid: chidByCreatedPid,
			HttpResponse:     w,
		}

	}

}

func _getIndexByStatus(status string, workflow []Step) int {

	for i, step := range workflow {
		if step.Status == status {
			return i
		}
	}

	return -1

}

func _completeStep(status string, workflow []Step) []Step {

	for i := range workflow {
		stepPtr := &workflow[i]
		if stepPtr.Status == status {
			stepPtr.ActionDate = 1
			stepPtr.Completed = true

			return workflow

		}
	}

	return nil
}

func _getUserByRole(role string, td *TestData, worker string) string {
	switch role {
	case BORROWER:
		return td.BorrowUser
	case LIBWORKER:
		return worker
	case KEEPER:
		return td.ABook.KeeperUsers[0]
	}

	return ""
}

func _checkBookMessageResult(t *testing.T, w *httptest.ResponseRecorder, ifErr bool, expMessages map[string]BooksMessage) {
	result := w.Result()
	var resultObj *Result
	json.NewDecoder(result.Body).Decode(&resultObj)
	require.NotEqual(t, result.StatusCode, 404, "should find this service")
	if ifErr {
		require.NotEmpty(t, resultObj.Error, "should be error")
	} else {
		require.Empty(t, resultObj.Error, "should not be error")
	}

	//check result
	for k, msg := range expMessages {
		bodyJson, ok := resultObj.Messages[k]
		require.Equalf(t, ok, true, "book id %v should exist in result.", k)
		var body BooksMessage
		json.Unmarshal([]byte(bodyJson), &body)
		assert.Equalf(t, body.PostId, msg.PostId, "public post id.")
		assert.Equalf(t, body.Status, msg.Status, "status.")
		if msg.Message != "" {
			// assert.Containsf(t, body.Message, msg.Message, "should contain expected message")
			assert.Equalf(t, msg.Message, body.Message, "should contain expected message")
		}
	}

}

func _initPassed(workflow []Step, fromStep Step, toStep Step, passed map[string]struct{}, params ..._initPassedParams) {

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
		_initPassed(workflow, nextStep, toStep, passed, _initPassedParams{
			tempPassed: tempPassed,
		})
	}

}

type _assertAfterwordStepsClearedParams struct {
	passed map[string]struct{}
}

func _assertAfterwordStepsCleared(t *testing.T, status string, wf []Step, params ..._assertAfterwordStepsClearedParams) {

	index := _getIndexByStatus(status, wf)

	if params == nil {
		passed := map[string]struct{}{}
		_initPassed(wf, wf[0], wf[index], passed)
		param := _assertAfterwordStepsClearedParams{
			passed: passed,
		}
		_assertAfterwordStepsCleared(t, status, wf, param)
		return
	}

	param := params[0]
	step := wf[index]

	if step.NextStepIndex == nil {
		return
	}

	param.passed[step.Status] = struct{}{}

	for _, i := range step.NextStepIndex {
		nextStep := &wf[i]
		if _, ok := param.passed[nextStep.Status]; ok {
			return
		}
		assert.Equalf(t, int64(0), nextStep.ActionDate, "action date should be cleared, status %v, next status: %v", status, nextStep.Status)
		assert.Equalf(t, false, nextStep.Completed, "completed should be false, status %v, next status: %v", status, nextStep.Status)
		_assertAfterwordStepsCleared(t, wf[i].Status, wf, param)
	}

}
