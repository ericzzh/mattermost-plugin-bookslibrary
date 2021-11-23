package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sort"
	"testing"
	// "time"

	"github.com/mattermost/mattermost-server/v5/model"
	// "github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	// "github.com/stretchr/testify/require"
)

func TestBorrowHandle(t *testing.T) {
	_ = fmt.Println

	td := NewTestData()

	bookPostId := td.BookPostIdPub

	borChannelId := td.BorChannelId

	aBook := td.ABook
	borrowUser := td.BorrowUser

	reqKey := td.ReqKey
	reqKeyJson, _ := json.Marshal(reqKey)

	borId_botId := td.BorId_botId
	worker1Id_botId := td.Worker1Id_botId
	worker2Id_botId := td.Worker2Id_botId
	keeper1Id_botId := td.Keeper1Id_botId
	keeper2Id_botId := td.Keeper2Id_botId

	apiMockCommon := td.ApiMockCommon

	newMockPlugin := td.NewMockPlugin

	matchPost := td.MatchPostByChannel
	matchPostById := td.MatchPostById

	t.Run("_makeBorrowRequest", func(t *testing.T) {
		otherData := otherRequestData{
			processTime: GetNowTime(),
		}
		api := apiMockCommon()
		plugin := newMockPlugin()
		plugin.SetAPI(api)

		br, _ := plugin._makeBorrowRequest(&reqKey, borrowUser, []string{MASTER}, nil, otherData)
		assert.Equal(t, aBook.BookPublic.Id, br.BookId, "BookId")
		assert.Equal(t, aBook.BookPublic.Name, br.BookName, "BookName")
		assert.Equal(t, aBook.Author, br.Author, "Author")
		assert.Equal(t, borrowUser, br.BorrowerUser, "BorrowerUser")
		assert.Equal(t, "bookbor", br.BorrowerName, "BorrowerName")
		assert.Contains(t, aBook.LibworkerUsers, br.LibworkerUser, "LibworkerUser")
		assert.Contains(t, aBook.LibworkerNames, br.LibworkerName, "LibworkerName")
		assert.Equal(t, aBook.KeeperUsers, br.KeeperUsers, "KeeperUsers")
		assert.Equal(t, aBook.KeeperNames, br.KeeperNames, "KeeperNames")

		assert.Equal(t, 0, br.StepIndex, "StepIndex")

		// testTime := time.Now().Add(-10 * time.Second).Unix()
		assert.Equal(t, br.Worflow[0].ActionDate, otherData.processTime, "RequestDate")

		wf := plugin._createWFTemplate(0)
		br.Worflow[0].ActionDate = 0
		assert.Equal(t, wf, br.Worflow, "Workflow")

		assert.Equal(t, []string{
			TAG_PREFIX_BORROWER + borrowUser,
			TAG_PREFIX_LIBWORKER + br.LibworkerUser,
			TAG_PREFIX_KEEPER + aBook.KeeperUsers[0],
			TAG_PREFIX_KEEPER + aBook.KeeperUsers[1],
			TAG_PREFIX_STATUS + STATUS_REQUESTED,
		}, br.Tags, "Tags")

	})

	t.Run("serveHttp_borrow", func(t *testing.T) {
		api := apiMockCommon()
		plugin := newMockPlugin()
		plugin.SetAPI(api)

		var realbr *Borrow
		getCurrentPosts := GenerateBorrowRequest(td, plugin, api)
		returnedInfo := getCurrentPosts()

		realbrPosts := returnedInfo.RealbrPost
		realbrUpdPosts := returnedInfo.RealbrUpdPosts
		createdPid := returnedInfo.CreatedPid

		//------- Verfication -------
		realbrMsg := realbrUpdPosts[plugin.borrowChannel.Id].Message
		json.Unmarshal([]byte(realbrMsg), &realbr)

		realwk := realbr.DataOrImage.LibworkerUser

		var dirWkChId, realwkName string
		if realwk == "worker1" {
			dirWkChId = worker1Id_botId
			realwkName = "wkname1"
		} else {
			dirWkChId = worker2Id_botId
			realwkName = "wkname2"
		}

		for _, role := range []struct {
			role         string
			channelId    string
			borrower     string
			borrowerName string
			worker       string
			workerName   string
			keeperUsers  []string
			keeperNames  []string
			workflow     []Step
			tags         []string
		}{
			{
				role:         MASTER,
				channelId:    plugin.borrowChannel.Id,
				borrower:     borrowUser,
				borrowerName: "bookbor",
				worker:       realwk,
				workerName:   realwkName,
				keeperUsers:  []string{"kpuser1", "kpuser2"},
				keeperNames:  []string{"kpname1", "kpname2"},
				workflow:     plugin._createWFTemplate(0),
				tags: []string{
					TAG_PREFIX_BORROWER + borrowUser,
					TAG_PREFIX_LIBWORKER + realwk,
					TAG_PREFIX_KEEPER + "kpuser1",
					TAG_PREFIX_KEEPER + "kpuser2",
					TAG_PREFIX_STATUS + STATUS_REQUESTED,
				},
			},
			{
				role:         BORROWER,
				channelId:    borId_botId,
				borrower:     borrowUser,
				borrowerName: "bookbor",
				worker:       realwk,
				workerName:   realwkName,
				keeperUsers:  []string{},
				keeperNames:  []string{},
				workflow:     plugin._createWFTemplate(0),
				tags: []string{
					TAG_PREFIX_BORROWER + borrowUser,
					TAG_PREFIX_LIBWORKER + realwk,
					TAG_PREFIX_STATUS + STATUS_REQUESTED,
				},
			},
			{
				role:         LIBWORKER,
				channelId:    dirWkChId,
				borrower:     borrowUser,
				borrowerName: "bookbor",
				worker:       realwk,
				workerName:   realwkName,
				keeperUsers:  []string{"kpuser1", "kpuser2"},
				keeperNames:  []string{"kpname1", "kpname2"},
				workflow:     plugin._createWFTemplate(0),
				tags: []string{
					TAG_PREFIX_BORROWER + borrowUser,
					TAG_PREFIX_LIBWORKER + realwk,
					TAG_PREFIX_KEEPER + "kpuser1",
					TAG_PREFIX_KEEPER + "kpuser2",
					TAG_PREFIX_STATUS + STATUS_REQUESTED,
				},
			},
			{
				role:         KEEPER,
				channelId:    keeper1Id_botId,
				borrower:     "",
				borrowerName: "",
				worker:       realwk,
				workerName:   realwkName,
				keeperUsers:  []string{"kpuser1", "kpuser2"},
				keeperNames:  []string{"kpname1", "kpname2"},
				workflow:     plugin._createWFTemplate(0),
				tags: []string{
					TAG_PREFIX_LIBWORKER + realwk,
					TAG_PREFIX_KEEPER + "kpuser1",
					TAG_PREFIX_KEEPER + "kpuser2",
					TAG_PREFIX_STATUS + STATUS_REQUESTED,
				},
			},
			{
				role:         KEEPER,
				channelId:    keeper2Id_botId,
				borrower:     "",
				borrowerName: "",
				worker:       realwk,
				workerName:   realwkName,
				keeperUsers:  []string{"kpuser1", "kpuser2"},
				keeperNames:  []string{"kpname1", "kpname2"},
				workflow:     plugin._createWFTemplate(0),
				tags: []string{
					TAG_PREFIX_LIBWORKER + realwk,
					TAG_PREFIX_KEEPER + "kpuser1",
					TAG_PREFIX_KEEPER + "kpuser2",
					TAG_PREFIX_STATUS + STATUS_REQUESTED,
				},
			},
		} {
			var thisRealBr Borrow
			_ = json.Unmarshal([]byte(realbrUpdPosts[role.channelId].Message), &thisRealBr)

			role.workflow[0].ActionDate = thisRealBr.DataOrImage.Worflow[0].ActionDate
			br := &BorrowRequest{
				BookPostId:    bookPostId,
				BookId:        aBook.BookPublic.Id,
				BookName:      aBook.BookPublic.Name,
				Author:        aBook.Author,
				BorrowerUser:  role.borrower,
				BorrowerName:  role.borrowerName,
				LibworkerUser: role.worker,
				LibworkerName: role.workerName,
				KeeperUsers:   role.keeperUsers,
				KeeperNames:   role.keeperNames,
				Worflow:       role.workflow,
				StepIndex:     0,
				Tags:          role.tags,
			}

			// we have to distribute every library work to a borrow request
			// because the distribution is random
			borrowExp := &Borrow{
				DataOrImage:  br,
				Role:         []string{role.role},
				RelationKeys: RelationKeys{},
			}

			borrowExpJson, _ := json.Marshal(borrowExp)

			expPost := &model.Post{
				Id:        createdPid[role.channelId],
				UserId:    plugin.botID,
				ChannelId: role.channelId,
				Message:   "",
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

			borrowExpJson, _ = json.MarshalIndent(borrowExp, "", "  ")
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

			api.On("SearchPostsInTeam", plugin.team.Id, mock.AnythingOfType("[]*model.SearchParams")).
				Return([]*model.Post{}, nil)
			api.On("DeletePost", mock.AnythingOfType("string")).Return(nil)

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

	t.Run("multi_roles", func(t *testing.T) {

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

			thisBookJson, _ := json.Marshal(thisBook)

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
			api.On("SearchPostsInTeam", plugin.team.Id, mock.AnythingOfType("[]*model.SearchParams")).
				Return([]*model.Post{}, nil)

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

func TestBorrowForChange(t *testing.T) {

	t.Run("toggle to disallowed as no-stock", func(t *testing.T) {
		td := NewTestData()
		td.ABookInv.Stock = 0

		api := td.ApiMockCommon()
		plugin := td.NewMockPlugin()
		plugin.SetAPI(api)

		GenerateBorrowRequest(td, plugin, api)

		assert.Equalf(t, false, td.ABookPub.IsAllowedToBorrow, "should not allowed")
		assert.Equalf(t, "无库存", td.ABookPub.ReasonOfDisallowed, "reason should be the text of no-stock")

	})
}
