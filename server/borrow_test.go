package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	// "github.com/stretchr/testify/require"
)

func TestHandleBorrow(t *testing.T) {
	_ = fmt.Println

	bookPostId := model.NewId()

	botId := model.NewId()
	borChannelId := model.NewId()
	borTeamId := model.NewId()

	aBook := &Book{
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
		KeeperUsers:       []string{"kpuser1", "kpuser2"},
		KeeperNames:       []string{"kpname1", "kpname2"},
		IsAllowedToBorrow: true,
		Tags:              []string{},
	}
	aBookJson, _ := json.MarshalIndent(aBook, "", "")
	borrowUser := "bor"

	reqKey := BorrowRequestKey{
		BookPostId:   bookPostId,
		BorrowerUser: borrowUser,
	}
	reqKeyJson, _ := json.Marshal(reqKey)

	borId := model.NewId()
	worker1Id := model.NewId()
	worker2Id := model.NewId()
	keeper1Id := model.NewId()
	keeper2Id := model.NewId()
	borId_botId := model.NewId()
	worker1Id_botId := model.NewId()
	worker2Id_botId := model.NewId()
	keeper1Id_botId := model.NewId()
	keeper2Id_botId := model.NewId()

	apiMockCommon := func() *plugintest.API {
		api := &plugintest.API{}

		api.On("GetPost", bookPostId).Return(&model.Post{
			Message: string(aBookJson),
		}, nil)

		api.On("GetUserByUsername", "bor").Return(&model.User{
			Id:        borId,
			LastName:  "book",
			FirstName: "bor",
		}, nil)

		api.On("GetUserByUsername", "worker1").Return(&model.User{
			Id:        worker1Id,
			LastName:  "wk",
			FirstName: "name1",
		}, nil)

		api.On("GetUserByUsername", "worker2").Return(&model.User{
			Id:        worker2Id,
			LastName:  "wk",
			FirstName: "name2",
		}, nil)

		api.On("GetUserByUsername", "kpuser1").Return(&model.User{
			Id:        keeper1Id,
			LastName:  "kp",
			FirstName: "name1",
		}, nil)

		api.On("GetUserByUsername", "kpuser2").Return(&model.User{
			Id:        keeper2Id,
			LastName:  "kp",
			FirstName: "name2",
		}, nil)

		api.On("GetDirectChannel", borId, botId).Return(&model.Channel{
			Id: borId_botId,
		}, nil)

		api.On("GetDirectChannel", worker1Id, botId).Return(&model.Channel{
			Id: worker1Id_botId,
		}, nil)

		api.On("GetDirectChannel", worker2Id, botId).Return(&model.Channel{
			Id: worker2Id_botId,
		}, nil)

		api.On("GetDirectChannel", keeper1Id, botId).Return(&model.Channel{
			Id: keeper1Id_botId,
		}, nil)

		api.On("GetDirectChannel", keeper2Id, botId).Return(&model.Channel{
			Id: keeper2Id_botId,
		}, nil)

		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).Return()
		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).Return()
		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).Return()
		api.On("LogError",
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("[]string"),
			mock.AnythingOfType("string"),
			mock.AnythingOfType("string")).Return()
		api.On("DeletePost", mock.AnythingOfType("string")).Return(nil)

		return api

	}

	newMockPlugin := func() *Plugin {
		return &Plugin{
			botID: botId,
			borrowChannel: &model.Channel{
				Id: borChannelId,
			},
			team: &model.Team{
				Id: borTeamId,
			},
		}
	}

	matchPost := func(channelId string) func(*model.Post) bool {
		return func(post *model.Post) bool {
			return post.ChannelId == channelId
		}
	}

	matchPostById := func(PostId string) func(*model.Post) bool {
		return func(post *model.Post) bool {
			return post.Id == PostId
		}
	}
	t.Run("_makeBorrowRequest", func(t *testing.T) {
		api := apiMockCommon()
		plugin := newMockPlugin()
		plugin.SetAPI(api)

		br, _ := plugin._makeBorrowRequest(&reqKey, borrowUser)
		assert.Equal(t, aBook.Id, br.BookId, "BookId")
		assert.Equal(t, aBook.Name, br.BookName, "BookName")
		assert.Equal(t, aBook.Author, br.Author, "Author")
		assert.Equal(t, borrowUser, br.BorrowerUser, "BorrowerUser")
		assert.Equal(t, "bookbor", br.BorrowerName, "BorrowerName")
		assert.Contains(t, aBook.LibworkerUsers, br.LibworkerUser, "LibworkerUser")
		assert.Contains(t, aBook.LibworkerNames, br.LibworkerName, "LibworkerName")
		assert.Equal(t, aBook.KeeperUsers, br.KeeperUsers, "KeeperUsers")
		assert.Equal(t, aBook.KeeperNames, br.KeeperNames, "KeeperNames")

		testTime := time.Now().Add(-10 * time.Second).Unix()

		assert.Greater(t, br.RequestDate, int64(testTime), "RequestDate")
		assert.Equal(t, WORKFLOW_BORROW, br.WorkflowType, "WorkflowType")
		assert.Equal(t, []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED}, br.Worflow, "WorkflowType")
		assert.Equal(t, STATUS_REQUESTED, br.Status, "Status")
		assert.Equal(t, []string{
			"#STATUS_EQ_" + STATUS_REQUESTED,
			"#BORROWERUSER_EQ_" + borrowUser,
			"#LIBWORKERUSER_EQ_" + br.LibworkerUser,
			"#KEEPERUSER_EQ_" + aBook.KeeperUsers[0],
			"#KEEPERUSER_EQ_" + aBook.KeeperUsers[1],
		}, br.Tags, "Tags")

	})

	t.Run("serveHttp_borrow", func(t *testing.T) {
		api := apiMockCommon()
		plugin := newMockPlugin()
		plugin.SetAPI(api)

		realbrPosts := map[string]*model.Post{}
		realbrUpdPosts := map[string]*model.Post{}
		var realbr *Borrow

		runfn := func(args mock.Arguments) {
			realbrPost := args.Get(0).(*model.Post)
			realbrPosts[realbrPost.ChannelId] = realbrPost
		}

		runfnUpd := func(args mock.Arguments) {
			realbrPost := args.Get(0).(*model.Post)
			realbrUpdPosts[realbrPost.ChannelId] = realbrPost
		}

		createdPid := map[string]string{
			plugin.borrowChannel.Id: model.NewId(),
			borId_botId:             model.NewId(),
			worker1Id_botId:         model.NewId(),
			worker2Id_botId:         model.NewId(),
			keeper1Id_botId:         model.NewId(),
			keeper2Id_botId:         model.NewId(),
		}

		api.On("CreatePost", mock.MatchedBy(matchPost(plugin.borrowChannel.Id))).Run(runfn).
			Return(&model.Post{
				Id:        createdPid[plugin.borrowChannel.Id],
				ChannelId: plugin.borrowChannel.Id,
				UserId:    botId,
				Type:      "custom_borrow_type",
			}, nil)
		api.On("CreatePost", mock.MatchedBy(matchPost(borId_botId))).Run(runfn).
			Return(&model.Post{
				Id:        createdPid[borId_botId],
				ChannelId: borId_botId,
				UserId:    botId,
				Type:      "custom_borrow_type",
			}, nil)
		api.On("CreatePost", mock.MatchedBy(matchPost(worker1Id_botId))).Run(runfn).
			Return(&model.Post{
				Id:        createdPid[worker1Id_botId],
				ChannelId: worker1Id_botId,
				UserId:    botId,
				Type:      "custom_borrow_type",
			}, nil)
		api.On("CreatePost", mock.MatchedBy(matchPost(worker2Id_botId))).Run(runfn).
			Return(&model.Post{
				Id:        createdPid[worker2Id_botId],
				ChannelId: worker2Id_botId,
				UserId:    botId,
				Type:      "custom_borrow_type",
			}, nil)
		api.On("CreatePost", mock.MatchedBy(matchPost(keeper1Id_botId))).Run(runfn).
			Return(&model.Post{
				Id:        createdPid[keeper1Id_botId],
				ChannelId: keeper1Id_botId,
				UserId:    botId,
				Type:      "custom_borrow_type",
			}, nil)
		api.On("CreatePost", mock.MatchedBy(matchPost(keeper2Id_botId))).Run(runfn).
			Return(&model.Post{
				Id:        createdPid[keeper2Id_botId],
				ChannelId: keeper2Id_botId,
				UserId:    botId,
				Type:      "custom_borrow_type",
			}, nil)

		api.On("UpdatePost", mock.MatchedBy(matchPost(plugin.borrowChannel.Id))).Run(runfnUpd).
			Return(&model.Post{Id: createdPid[plugin.borrowChannel.Id]}, nil)
		api.On("UpdatePost", mock.MatchedBy(matchPost(borId_botId))).Run(runfnUpd).
			Return(&model.Post{Id: createdPid[borId_botId]}, nil)
		api.On("UpdatePost", mock.MatchedBy(matchPost(worker1Id_botId))).Run(runfnUpd).
			Return(&model.Post{Id: createdPid[worker1Id_botId]}, nil)
		api.On("UpdatePost", mock.MatchedBy(matchPost(worker2Id_botId))).Run(runfnUpd).
			Return(&model.Post{Id: createdPid[worker2Id_botId]}, nil)
		api.On("UpdatePost", mock.MatchedBy(matchPost(keeper1Id_botId))).Run(runfnUpd).
			Return(&model.Post{Id: createdPid[keeper1Id_botId]}, nil)
		api.On("UpdatePost", mock.MatchedBy(matchPost(keeper2Id_botId))).Run(runfnUpd).
			Return(&model.Post{Id: createdPid[keeper2Id_botId]}, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/borrow", bytes.NewReader(reqKeyJson))
		plugin.ServeHTTP(nil, w, r)

		//------- Verfication -------
		realbrMsg := realbrPosts[plugin.borrowChannel.Id].Message
		json.Unmarshal([]byte(realbrMsg), &realbr)

		realwk := realbr.DataOrImage.LibworkerUser
		realReqDate := realbr.DataOrImage.RequestDate

		//make sure we have the same library worker
		br, _ := plugin._makeBorrowRequest(&reqKey, borrowUser)
		for br.LibworkerUser != realwk {
			br, _ = plugin._makeBorrowRequest(&reqKey, borrowUser)
		}
		br.RequestDate = realReqDate

		var dirWkChId string
		if br.LibworkerUser == "worker1" {
			dirWkChId = worker1Id_botId
		} else {
			dirWkChId = worker2Id_botId
		}

		for _, role := range []struct {
			role         string
			channelId    string
			borrower     string
			borrowerName string
			keeperUsers  []string
			keeperNames  []string
			workflow     []string
			tags         []string
		}{
			{
				role:         MASTER,
				channelId:    plugin.borrowChannel.Id,
				borrower:     br.BorrowerUser,
				borrowerName: br.BorrowerName,
				keeperUsers:  []string{"kpuser1", "kpuser2"},
				keeperNames:  []string{"kpname1", "kpname2"},
				workflow:     []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
				tags: []string{
					"#STATUS_EQ_REQUESTED",
					"#BORROWERUSER_EQ_" + br.BorrowerUser,
					"#LIBWORKERUSER_EQ_" + br.LibworkerUser,
					"#KEEPERUSER_EQ_" + br.KeeperUsers[0],
					"#KEEPERUSER_EQ_" + br.KeeperUsers[1],
				},
			},
			{
				role:        BORROWER,
				channelId:   borId_botId,
				borrower:     br.BorrowerUser,
				borrowerName: br.BorrowerName,
				keeperUsers: []string{},
				keeperNames: []string{},
				workflow:    []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
				tags: []string{
					"#STATUS_EQ_REQUESTED",
					"#BORROWERUSER_EQ_" + br.BorrowerUser,
					"#LIBWORKERUSER_EQ_" + br.LibworkerUser,
				},
			},
			{
				role:        LIBWORKER,
				channelId:   dirWkChId,
				borrower:     br.BorrowerUser,
				borrowerName: br.BorrowerName,
				keeperUsers: []string{"kpuser1", "kpuser2"},
				keeperNames: []string{"kpname1", "kpname2"},
				workflow:    []string{STATUS_REQUESTED, STATUS_CONFIRMED, STATUS_DELIVIED},
				tags: []string{
					"#STATUS_EQ_REQUESTED",
					"#BORROWERUSER_EQ_" + br.BorrowerUser,
					"#LIBWORKERUSER_EQ_" + br.LibworkerUser,
					"#KEEPERUSER_EQ_" + br.KeeperUsers[0],
					"#KEEPERUSER_EQ_" + br.KeeperUsers[1],
				},
			},
			{
				role:        KEEPER,
				channelId:   keeper1Id_botId,
				borrower:     "",
				borrowerName: "",
				keeperUsers: []string{"kpuser1", "kpuser2"},
				keeperNames: []string{"kpname1", "kpname2"},
				workflow:    []string{STATUS_REQUESTED, STATUS_CONFIRMED},
				tags: []string{
					"#STATUS_EQ_REQUESTED",
					"#LIBWORKERUSER_EQ_" + br.LibworkerUser,
					"#KEEPERUSER_EQ_" + br.KeeperUsers[0],
					"#KEEPERUSER_EQ_" + br.KeeperUsers[1],
				},
			},
			{
				role:      KEEPER,
				channelId: keeper2Id_botId,
				borrower:     "",
				borrowerName: "",
				keeperUsers: []string{"kpuser1", "kpuser2"},
				keeperNames: []string{"kpname1", "kpname2"},
				workflow:    []string{STATUS_REQUESTED, STATUS_CONFIRMED},
				tags: []string{
					"#STATUS_EQ_REQUESTED",
					"#LIBWORKERUSER_EQ_" + br.LibworkerUser,
					"#KEEPERUSER_EQ_" + br.KeeperUsers[0],
					"#KEEPERUSER_EQ_" + br.KeeperUsers[1],
				},
			},
		} {

			// we have to distribute every library work to a borrow request
			// because the distribution is random
			borrowExp := &Borrow{
				DataOrImage:  br,
				Role:         []string{role.role},
				RelationKeys: RelationKeys{},
			}
                        borrowExp.DataOrImage.BorrowerUser = role.borrower
                        borrowExp.DataOrImage.BorrowerName = role.borrowerName
                        borrowExp.DataOrImage.KeeperUsers = role.keeperUsers
                        borrowExp.DataOrImage.KeeperNames = role.keeperNames
                        borrowExp.DataOrImage.Worflow = role.workflow
                        borrowExp.DataOrImage.Tags = role.tags

			borrowExpJson, _ := json.MarshalIndent(borrowExp, "", "")

			expPost := &model.Post{
				UserId:    plugin.botID,
				ChannelId: role.channelId,
				Message:   string(borrowExpJson),
				Type:      "custom_borrow_type",
			}

			//prfer to use assert.Equal because the test result is clearer
			assert.Equal(t, expPost, realbrPosts[role.channelId])

			borrowExp.RelationKeys.Book = bookPostId
			if role.role == MASTER {
				borrowExp.RelationKeys.Borrower = createdPid[borId_botId]
				borrowExp.RelationKeys.Libworker = createdPid[dirWkChId]
				borrowExp.RelationKeys.Keepers = []string{
					createdPid[keeper1Id_botId],
					createdPid[keeper2Id_botId],
				}
				sort.Strings(borrowExp.RelationKeys.Keepers)

			} else {
				borrowExp.RelationKeys.Master = createdPid[plugin.borrowChannel.Id]
			}

			borrowExpJson, _ = json.MarshalIndent(borrowExp, "", "")
			expPost = &model.Post{
				Id:        createdPid[role.channelId],
				UserId:    plugin.botID,
				ChannelId: role.channelId,
				Message:   string(borrowExpJson),
				Type:      "custom_borrow_type",
			}
			// fmt.Printf("*********** role: %v\n", role.role)
			assert.Equal(t, expPost, realbrUpdPosts[role.channelId])
		}

	})

	t.Run("rollback", func(t *testing.T) {
		//the order must be same as the logic

		createdPid := map[string]string{
			borChannelId:    model.NewId(),
			borId_botId:     model.NewId(),
			worker1Id_botId: model.NewId(),
			worker2Id_botId: model.NewId(),
			keeper1Id_botId: model.NewId(),
			keeper2Id_botId: model.NewId(),
		}
		tests := []struct {
			role       string
			channelId  []string
			createdPid []string
			stage      string
		}{
			{
				role:       MASTER,
				channelId:  []string{borChannelId},
				createdPid: []string{createdPid[borChannelId]},
				stage:      "CREATE",
			},
			{
				role:       BORROWER,
				channelId:  []string{borId_botId},
				createdPid: []string{createdPid[borId_botId]},
				stage:      "CREATE",
			},
			{
				role:       LIBWORKER,
				channelId:  []string{worker1Id_botId, worker2Id_botId},
				createdPid: []string{createdPid[worker1Id_botId], createdPid[worker2Id_botId]},
				stage:      "CREATE",
			},
			{
				role:       KEEPER,
				channelId:  []string{keeper1Id_botId},
				createdPid: []string{createdPid[keeper1Id_botId]},
				stage:      "CREATE",
			},
			{
				role:       KEEPER,
				channelId:  []string{keeper2Id_botId},
				createdPid: []string{createdPid[keeper2Id_botId]},
				stage:      "CREATE",
			},
			{
				role:       MASTER,
				channelId:  []string{borChannelId},
				createdPid: []string{createdPid[borChannelId]},
				stage:      "UPDATE",
			},
			{
				role:       BORROWER,
				channelId:  []string{borId_botId},
				createdPid: []string{createdPid[borId_botId]},
				stage:      "UPDATE",
			},
			{
				role:       LIBWORKER,
				channelId:  []string{worker1Id_botId, worker2Id_botId},
				createdPid: []string{createdPid[worker1Id_botId], createdPid[worker2Id_botId]},
				stage:      "UPDATE",
			},
			{
				role:       KEEPER,
				channelId:  []string{keeper1Id_botId},
				createdPid: []string{createdPid[keeper1Id_botId]},
				stage:      "UPDATE",
			},
			{
				role:       KEEPER,
				channelId:  []string{keeper2Id_botId},
				createdPid: []string{createdPid[keeper2Id_botId]},
				stage:      "UPDATE",
			},
		}

		for _, test := range tests {
			// fmt.Printf("*** %v\n", test.role)

			api := apiMockCommon()
			plugin := newMockPlugin()
			plugin.SetAPI(api)

			for i, chid := range test.channelId {
				if test.stage == "CREATE" {

					api.On("CreatePost", mock.MatchedBy(matchPost(chid))).
						Return(nil, &model.AppError{})
				} else {

					api.On("UpdatePost", mock.MatchedBy(matchPostById(test.createdPid[i]))).
						Return(nil, &model.AppError{})
				}
			}

			savedCreatedId := map[string]string{}

			api.On("CreatePost", mock.MatchedBy(matchPost(borChannelId))).
				Return(&model.Post{Id: createdPid[borChannelId]}, nil).
				Run(func(args mock.Arguments) {
					savedCreatedId[MASTER] = createdPid[borChannelId]
				})
			api.On("CreatePost", mock.MatchedBy(matchPost(borId_botId))).
				Return(&model.Post{Id: createdPid[borId_botId]}, nil).
				Run(func(args mock.Arguments) {
					savedCreatedId[BORROWER] = createdPid[borId_botId]
				})
			api.On("CreatePost", mock.MatchedBy(matchPost(worker1Id_botId))).
				Return(&model.Post{Id: createdPid[worker1Id_botId]}, nil).
				Run(func(args mock.Arguments) {
					savedCreatedId[LIBWORKER+"1"] = createdPid[worker1Id_botId]
				})
			api.On("CreatePost", mock.MatchedBy(matchPost(worker2Id_botId))).
				Return(&model.Post{Id: createdPid[worker2Id_botId]}, nil).
				Run(func(args mock.Arguments) {
					savedCreatedId[LIBWORKER+"2"] = createdPid[worker2Id_botId]
				})
			api.On("CreatePost", mock.MatchedBy(matchPost(keeper1Id_botId))).
				Return(&model.Post{Id: createdPid[keeper1Id_botId]}, nil).
				Run(func(args mock.Arguments) {
					savedCreatedId[KEEPER+"1"] = createdPid[keeper1Id_botId]
				})
			api.On("CreatePost", mock.MatchedBy(matchPost(keeper2Id_botId))).
				Return(&model.Post{Id: createdPid[keeper2Id_botId]}, nil).
				Run(func(args mock.Arguments) {
					savedCreatedId[KEEPER+"2"] = createdPid[keeper2Id_botId]
				})
			api.On("UpdatePost", mock.MatchedBy(matchPostById(createdPid[borChannelId]))).
				Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.MatchedBy(matchPostById(createdPid[borId_botId]))).
				Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.MatchedBy(matchPostById(createdPid[worker1Id_botId]))).
				Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.MatchedBy(matchPostById(createdPid[worker2Id_botId]))).
				Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.MatchedBy(matchPostById(createdPid[keeper1Id_botId]))).
				Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.MatchedBy(matchPostById(createdPid[keeper2Id_botId]))).
				Return(&model.Post{}, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/borrow", bytes.NewReader(reqKeyJson))
			plugin.ServeHTTP(nil, w, r)

			result := w.Result()
			var resultObj *Result
			json.NewDecoder(result.Body).Decode(&resultObj)
			assert.NotEmpty(t, resultObj.Error, "should be error")

			for role, cId := range savedCreatedId {
				_ = role
				// fmt.Printf("****** %v\n", role)
				api.AssertCalled(t, "DeletePost", cId)
			}

		}
	})

	t.Run("duplicated_users_bor_worker", func(t *testing.T) {

		for _, test := range []struct {
			borrower      string
			borId_botId   string
			worker        string
			workerNm      string
			worker_botId  string
			keepers       []string
			keepersNm     []string
			keepers_botId []string
		}{
			{
				borrower:      "worker1",
				borId_botId:   worker1Id_botId,
				worker:        "worker1",
				workerNm:      "wkname1",
				worker_botId:  worker1Id_botId,
				keepers:       []string{"kpuser1", "kpuser2"},
				keepersNm:     []string{"kpname1", "kpname2"},
				keepers_botId: []string{keeper1Id_botId, keeper2Id_botId},
			},
			{
				borrower:      "kpuser1",
				borId_botId:   keeper1Id_botId,
				worker:        "worker1",
				workerNm:      "wkname1",
				worker_botId:  worker1Id_botId,
				keepers:       []string{"kpuser1", "kpuser2"},
				keepersNm:     []string{"kpname1", "kpname2"},
				keepers_botId: []string{keeper1Id_botId, keeper2Id_botId},
			},
			{
				borrower:      "kpuser2",
				borId_botId:   keeper2Id_botId,
				worker:        "worker1",
				workerNm:      "wkname1",
				worker_botId:  worker1Id_botId,
				keepers:       []string{"kpuser2"},
				keepersNm:     []string{"kpname2"},
				keepers_botId: []string{keeper2Id_botId},
			},
			{
				borrower:      "bor",
				borId_botId:   borId_botId,
				worker:        "kpuser1",
				workerNm:      "kpname1",
				worker_botId:  keeper1Id_botId,
				keepers:       []string{"kpuser1", "kpuser2"},
				keepersNm:     []string{"kpname1", "kpname2"},
				keepers_botId: []string{keeper1Id_botId, keeper2Id_botId},
			},
			{
				borrower:      "worker1",
				borId_botId:   worker1Id_botId,
				worker:        "worker1",
				workerNm:      "wkname1",
				worker_botId:  worker1Id_botId,
				keepers:       []string{"worker1"},
				keepersNm:     []string{"wkname1"},
				keepers_botId: []string{worker1Id_botId},
			},
		} {

			api := apiMockCommon()
			plugin := newMockPlugin()
			plugin.SetAPI(api)

			thisBookPostId := model.NewId()
			thisBook := *aBook
			thisBook.LibworkerUsers = []string{test.worker}
			thisBook.LibworkerNames = []string{test.workerNm}
			thisBook.KeeperUsers = test.keepers
			thisBook.KeeperNames = test.keepersNm

			borrowUser := test.borrower

			reqKey := BorrowRequestKey{
				BookPostId:   thisBookPostId,
				BorrowerUser: borrowUser,
			}
			reqKeyJson, _ := json.Marshal(reqKey)

			realbrPosts := map[string][]*model.Post{}
			realbrUpdPosts := map[string][]*model.Post{}

			runfn := func(args mock.Arguments) {
				realbrPost := args.Get(0).(*model.Post)
				rbp, ok := realbrPosts[realbrPost.ChannelId]
				if !ok {
					rbp = []*model.Post{}
				}
				rbp = append(rbp, realbrPost)
				realbrPosts[realbrPost.ChannelId] = rbp
			}

			runfnUpd := func(args mock.Arguments) {
				realbrPost := args.Get(0).(*model.Post)
				rbp, ok := realbrUpdPosts[realbrPost.ChannelId]
				if !ok {
					rbp = []*model.Post{}
				}
				rbp = append(rbp, realbrPost)
				realbrUpdPosts[realbrPost.ChannelId] = rbp
			}

			thisBookJson, _ := json.MarshalIndent(thisBook, "", "")

			api.On("GetPost", thisBookPostId).Return(&model.Post{
				Message: string(thisBookJson),
			}, nil)

			api.On("CreatePost", mock.MatchedBy(matchPost(test.borId_botId))).
				Return(&model.Post{ChannelId: test.borId_botId}, nil).Run(runfn)
			api.On("UpdatePost", mock.MatchedBy(matchPost(test.borId_botId))).
				Return(&model.Post{ChannelId: test.borId_botId}, nil).Run(runfnUpd)

			api.On("CreatePost", mock.MatchedBy(matchPost(test.worker_botId))).
				Return(&model.Post{ChannelId: test.worker_botId}, nil).Run(runfn)
			api.On("UpdatePost", mock.MatchedBy(matchPost(test.worker_botId))).
				Return(&model.Post{ChannelId: test.worker_botId}, nil).Run(runfnUpd)

			for i := range test.keepers {
				api.On("CreatePost", mock.MatchedBy(matchPost(test.keepers_botId[i]))).
					Return(&model.Post{ChannelId: test.keepers_botId[i]}, nil).Run(runfn)
				api.On("UpdatePost", mock.MatchedBy(matchPost(test.keepers_botId[i]))).
					Return(&model.Post{ChannelId: test.keepers_botId[i]}, nil).Run(runfnUpd)
			}

			api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/borrow", bytes.NewReader(reqKeyJson))
			plugin.ServeHTTP(nil, w, r)

			assert.Equalf(t, 1, len(realbrPosts[test.borId_botId]), "post to borrower: %v should be 1 time", test.borrower)
			assert.Equalf(t, 1, len(realbrUpdPosts[test.borId_botId]), "update to borrower: %v should be 1 time", test.borrower)

			assert.Equalf(t, 1, len(realbrPosts[test.worker_botId]), "post to worker: %v should be 1 time", test.worker)
			assert.Equalf(t, 1, len(realbrUpdPosts[test.worker_botId]), "update to worker: %v should be 1 time", test.worker)

			for i := range test.keepers {
				assert.Equalf(t, 1, len(realbrPosts[test.keepers_botId[i]]), "post to keeper: %v should be 1 time", test.keepers[i])
				assert.Equalf(t, 1, len(realbrUpdPosts[test.keepers_botId[i]]), "update to keep: %v should be 1 time", test.keepers[i])
			}
		}

	})

}
