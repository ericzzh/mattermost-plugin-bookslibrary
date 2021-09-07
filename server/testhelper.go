package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	// "github.com/stretchr/testify/require"
)

type TestData struct {
	ABook           *Book
	BookPostId      string
	BotId           string
	BorChannelId    string
	BorTeamId       string
	ABookJson       []byte
	BorrowUser      string
	ReqKey          BorrowRequestKey
	ReqKeyJson      []byte
	BorId           string
	Worker1Id       string
	Worker2Id       string
	Keeper1Id       string
	Keeper2Id       string
	BorId_botId     string
	Worker1Id_botId string
	Worker2Id_botId string
	Keeper1Id_botId string
	Keeper2Id_botId string
	ApiMockCommon   func() *plugintest.API
	NewMockPlugin   func() *Plugin
        MatchPostByChannel   func(string) func(*model.Post) bool
        MatchPostById   func(string) func(*model.Post) bool
}

func NewTestData() TestData {
	td := TestData{}

	td.BookPostId = model.NewId()

	td.BotId = model.NewId()
	td.BorChannelId = model.NewId()
	td.BorTeamId = model.NewId()

	td.ABook = &Book{
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
	td.ABookJson, _ = json.MarshalIndent(td.ABook, "", "")
	td.BorrowUser = "bor"

	td.ReqKey = BorrowRequestKey{
		BookPostId:   td.BookPostId,
		BorrowerUser: td.BorrowUser,
	}
	td.ReqKeyJson, _ = json.Marshal(td.ReqKey)

	td.BorId = model.NewId()
	td.Worker1Id = model.NewId()
	td.Worker2Id = model.NewId()
	td.Keeper1Id = model.NewId()
	td.Keeper2Id = model.NewId()
	td.BorId_botId = model.NewId()
	td.Worker1Id_botId = model.NewId()
	td.Worker2Id_botId = model.NewId()
	td.Keeper1Id_botId = model.NewId()
	td.Keeper2Id_botId = model.NewId()

	td.ApiMockCommon = func() *plugintest.API {
		api := &plugintest.API{}

		api.On("GetPost", td.BookPostId).Return(&model.Post{
			Message: string(td.ABookJson),
		}, nil)

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

	td.NewMockPlugin = func() *Plugin {
		return &Plugin{
			botID: td.BotId,
			borrowChannel: &model.Channel{
				Id: td.BorChannelId,
			},
			team: &model.Team{
				Id: td.BorTeamId,
			},
		}
	}

	td.MatchPostByChannel = func(channelId string) func(*model.Post) bool {
		return func(post *model.Post) bool {
			return post.ChannelId == channelId
		}
	}

	td.MatchPostById = func(PostId string) func(*model.Post) bool {
		return func(post *model.Post) bool {
			return post.Id == PostId
		}
	}
	return td
}

func GenerateBorrowRequest(td TestData, plugin *Plugin, api *plugintest.API) (map[string]*model.Post,map[string]*model.Post,map[string]string) {

	realbrPosts := map[string]*model.Post{}
	realbrUpdPosts := map[string]*model.Post{}
	matchPost := td.MatchPostByChannel

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
		td.BorId_botId:          model.NewId(),
		td.Worker1Id_botId:      model.NewId(),
		td.Worker2Id_botId:      model.NewId(),
		td.Keeper1Id_botId:      model.NewId(),
		td.Keeper2Id_botId:      model.NewId(),
	}

	api.On("CreatePost", mock.MatchedBy(matchPost(plugin.borrowChannel.Id))).Run(runfn).
		Return(&model.Post{
			Id:        createdPid[plugin.borrowChannel.Id],
			ChannelId: plugin.borrowChannel.Id,
			UserId:    td.BotId,
			Type:      "custom_borrow_type",
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.BorId_botId))).Run(runfn).
		Return(&model.Post{
			Id:        createdPid[td.BorId_botId],
			ChannelId: td.BorId_botId,
			UserId:    td.BotId,
			Type:      "custom_borrow_type",
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Worker1Id_botId))).Run(runfn).
		Return(&model.Post{
			Id:        createdPid[td.Worker1Id_botId],
			ChannelId: td.Worker1Id_botId,
			UserId:    td.BotId,
			Type:      "custom_borrow_type",
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Worker2Id_botId))).Run(runfn).
		Return(&model.Post{
			Id:        createdPid[td.Worker2Id_botId],
			ChannelId: td.Worker2Id_botId,
			UserId:    td.BotId,
			Type:      "custom_borrow_type",
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Keeper1Id_botId))).Run(runfn).
		Return(&model.Post{
			Id:        createdPid[td.Keeper1Id_botId],
			ChannelId: td.Keeper1Id_botId,
			UserId:    td.BotId,
			Type:      "custom_borrow_type",
		}, nil)
	api.On("CreatePost", mock.MatchedBy(matchPost(td.Keeper2Id_botId))).Run(runfn).
		Return(&model.Post{
			Id:        createdPid[td.Keeper2Id_botId],
			ChannelId: td.Keeper2Id_botId,
			UserId:    td.BotId,
			Type:      "custom_borrow_type",
		}, nil)

	api.On("UpdatePost", mock.MatchedBy(matchPost(plugin.borrowChannel.Id))).Run(runfnUpd).
		Return(&model.Post{Id: createdPid[plugin.borrowChannel.Id]}, nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.BorId_botId))).Run(runfnUpd).
		Return(&model.Post{Id: createdPid[td.BorId_botId]}, nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Worker1Id_botId))).Run(runfnUpd).
		Return(&model.Post{Id: createdPid[td.Worker1Id_botId]}, nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Worker2Id_botId))).Run(runfnUpd).
		Return(&model.Post{Id: createdPid[td.Worker2Id_botId]}, nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Keeper1Id_botId))).Run(runfnUpd).
		Return(&model.Post{Id: createdPid[td.Keeper1Id_botId]}, nil)
	api.On("UpdatePost", mock.MatchedBy(matchPost(td.Keeper2Id_botId))).Run(runfnUpd).
		Return(&model.Post{Id: createdPid[td.Keeper2Id_botId]}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/borrow", bytes.NewReader(td.ReqKeyJson))
	plugin.ServeHTTP(nil, w, r)


        return realbrPosts,realbrUpdPosts,createdPid
}
