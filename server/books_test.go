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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBooks(t *testing.T) {
	_ = fmt.Println

	td := NewTestData()
	plugin := td.NewMockPlugin()
	api := td.ApiMockCommon(
		mockapiOptons{
			excludeBookUpdAPI: true,
		})
	plugin.SetAPI(api)

	type bookPids map[string]string

	booksPids := []bookPids{
		{
			"pub_id": model.NewId(),
			"pri_id": model.NewId(),
			"inv_id": model.NewId(),
		},
		{
			"pub_id": model.NewId(),
			"pri_id": model.NewId(),
			"inv_id": model.NewId(),
		},
	}

	findIndexById := func(id string) int {
		for i, v := range booksPids {
			for _, x := range v {
				if x == id {
					return i
				}
			}
		}

		return -1
	}

	booksChidByPid := map[string]string{}

	for _, bookids := range booksPids {
		for chtype, id := range bookids {
			switch chtype {
			case "pub_id":
				booksChidByPid[id] = td.BookChIdPub
			case "pri_id":
				booksChidByPid[id] = td.BookChIdPri
			case "inv_id":
				booksChidByPid[id] = td.BookChIdInv
			}
		}
	}

	someBooksUpl := Books{
		{
			&BookPublic{
				Id:                 "zzh-book-001",
				Name:               "a test book",
				NameEn:             "a test book",
				Category1:          "C1",
				Category2:          "C2",
				Category3:          "C3",
				Author:             "zzh",
				AuthorEn:           "zzh",
				Translator:         "eric",
				TranslatorEn:       "eric",
				Publisher:          "pub1",
				PublisherEn:        "pub1En",
				PublishDate:        "20200821",
				LibworkerUsers:     []string{"worker1", "worker2"},
				IsAllowedToBorrow:  true,
				ManuallyDisallowed: false,
			},
			&BookPrivate{
				KeeperUsers: []string{"kpuser1", "kpuser2"},
				CopyKeeperMap: map[string]Keeper{
					"zzh-book-001 b1": {User: "kpuser1"},
					"zzh-book-001 b2": {User: "kpuser1"},
					"zzh-book-001 b3": {User: "kpuser1"},
					"zzh-book-001 b4": {User: "kpuser1"},
					"zzh-book-001 b5": {User: "kpuser2"},
					"zzh-book-001 b6": {User: "kpuser2"},
					"zzh-book-001 b7": {User: "kpuser2"},
				},
			},
			&BookInventory{
				Stock: 7,
				Copies: BookCopies{
					"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-001 b4": BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-001 b5": BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-001 b6": BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
				},
			},
			nil,
		},
		{
			&BookPublic{
				Id:                "zzh-book-002",
				Name:              "a second test book",
				NameEn:            "a second test book",
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
			},
			&BookPrivate{
				KeeperUsers: []string{"kpuser1", "kpuser2"},
				KeeperInfos: KeeperInfoMap{
					"kpuser1": {"kpname1"},
					"kpuser2": {"kpname2"},
				},
				CopyKeeperMap: map[string]Keeper{
					"zzh-book-002 b1":  {User: "kpuser1"},
					"zzh-book-002 b2":  {User: "kpuser1"},
					"zzh-book-002 b3":  {User: "kpuser1"},
					"zzh-book-002 b4":  {User: "kpuser1"},
					"zzh-book-002 b5":  {User: "kpuser1"},
					"zzh-book-002 b6":  {User: "kpuser2"},
					"zzh-book-002 b7":  {User: "kpuser2"},
					"zzh-book-002 b8":  {User: "kpuser2"},
					"zzh-book-002 b9":  {User: "kpuser2"},
					"zzh-book-002 b10": {User: "kpuser2"},
				},
			},
			&BookInventory{
				Stock: 10,
				Copies: BookCopies{
					"zzh-book-002 b1":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b2":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b3":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b4":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b5":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b6":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b7":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b8":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b9":  BookCopy{Status: COPY_STATUS_INSTOCK},
					"zzh-book-002 b10": BookCopy{Status: COPY_STATUS_INSTOCK},
				},
			},
			nil,
		},
	}

	booksJson, _ := json.Marshal(someBooksUpl)

	req := BooksRequest{
		Action:  BOOKS_ACTION_UPLOAD,
		ActUser: td.ABook.LibworkerNames[0],
		Body:    string(booksJson),
	}

	reqJson, _ := json.Marshal(req)

	sendARequestAndCheck := func(t *testing.T, bq BooksRequest, assertError bool, expMessages map[string]BooksMessage) {

		reqJson, _ := json.Marshal(bq)
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
		// fmt.Println(string(reqJson))
		plugin.ServeHTTP(nil, w, r)
		_checkBookMessageResult(t, w, assertError, expMessages)
	}

	// fmt.Println(string(reqJson))

	resetLastUpdated := ""

	resetSomeBooksInDB := func() Books {
		resetLastUpdated = model.NewId() 
		return Books{
			{
				&BookPublic{
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
					Tags: []string{
						TAG_PREFIX_ID + "zzh-book-001",
						TAG_PREFIX_C1 + "C1",
						TAG_PREFIX_C2 + "C2",
						TAG_PREFIX_C3 + "C3",
					},
					Relations: Relations{
						REL_BOOK_PRIVATE:   booksPids[0]["pri_id"],
						REL_BOOK_INVENTORY: booksPids[0]["inv_id"],
					},
					MatchId: resetLastUpdated,
				},
				&BookPrivate{
					Id:          "zzh-book-001",
					Name:        "a test book",
					KeeperUsers: []string{"kpuser1", "kpuser2"},
					KeeperInfos: KeeperInfoMap{
						"kpuser1": {"kpname1"},
						"kpuser2": {"kpname2"},
					},
					CopyKeeperMap: map[string]Keeper{
						"zzh-book-001 b1": {User: "kpuser1"},
						"zzh-book-001 b2": {User: "kpuser1"},
						"zzh-book-001 b3": {User: "kpuser1"},
						"zzh-book-001 b4": {User: "kpuser1"},
						"zzh-book-001 b5": {User: "kpuser2"},
						"zzh-book-001 b6": {User: "kpuser2"},
						"zzh-book-001 b7": {User: "kpuser2"},
					},
					Relations: Relations{
						REL_BOOK_PUBLIC: booksPids[0]["pub_id"],
					},
				},
				&BookInventory{
					Id:          "zzh-book-001",
					Name:        "a test book",
					Stock:       3,
					TransmitOut: 2,
					Lending:     1,
					TransmitIn:  1,
					Copies: BookCopies{
						"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_TRANSIN},
						"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_LENDING},
						"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_TRANSOUT},
						"zzh-book-001 b4": BookCopy{Status: COPY_STATUS_TRANSOUT},
						"zzh-book-001 b5": BookCopy{Status: COPY_STATUS_INSTOCK},
						"zzh-book-001 b6": BookCopy{Status: COPY_STATUS_INSTOCK},
						"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
					},
					Relations: Relations{
						REL_BOOK_PUBLIC: booksPids[0]["pub_id"],
					},
				},
				nil,
			},
			{
				&BookPublic{
					Id:                "zzh-book-002",
					Name:              "a second test book",
					NameEn:            "a second test book",
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
					Tags: []string{
						TAG_PREFIX_ID + "zzh-book-002",
						TAG_PREFIX_C1 + "C1",
						TAG_PREFIX_C2 + "C2",
						TAG_PREFIX_C3 + "C3",
					},
					Relations: Relations{
						REL_BOOK_PRIVATE:   booksPids[1]["pri_id"],
						REL_BOOK_INVENTORY: booksPids[1]["inv_id"],
					},
					MatchId: resetLastUpdated,
				},
				&BookPrivate{
					Id:          "zzh-book-002",
					Name:        "a second test book",
					KeeperUsers: []string{"kpuser1", "kpuser2"},
					KeeperInfos: KeeperInfoMap{
						"kpuser1": {"kpname1"},
						"kpuser2": {"kpname2"},
					},
					CopyKeeperMap: map[string]Keeper{
						"zzh-book-002 b1":  {User: "kpuser1"},
						"zzh-book-002 b2":  {User: "kpuser1"},
						"zzh-book-002 b3":  {User: "kpuser1"},
						"zzh-book-002 b4":  {User: "kpuser1"},
						"zzh-book-002 b5":  {User: "kpuser1"},
						"zzh-book-002 b6":  {User: "kpuser2"},
						"zzh-book-002 b7":  {User: "kpuser2"},
						"zzh-book-002 b8":  {User: "kpuser2"},
						"zzh-book-002 b9":  {User: "kpuser2"},
						"zzh-book-002 b10": {User: "kpuser2"},
					},
					Relations: Relations{
						REL_BOOK_PUBLIC: booksPids[1]["pub_id"],
					},
				},
				&BookInventory{
					Id:          "zzh-book-002",
					Name:        "a second test book",
					Stock:       4,
					TransmitOut: 3,
					Lending:     2,
					TransmitIn:  1,
					Copies: BookCopies{
						"zzh-book-002 b1":  BookCopy{Status: COPY_STATUS_TRANSIN},
						"zzh-book-002 b2":  BookCopy{Status: COPY_STATUS_LENDING},
						"zzh-book-002 b3":  BookCopy{Status: COPY_STATUS_LENDING},
						"zzh-book-002 b4":  BookCopy{Status: COPY_STATUS_TRANSOUT},
						"zzh-book-002 b5":  BookCopy{Status: COPY_STATUS_TRANSOUT},
						"zzh-book-002 b6":  BookCopy{Status: COPY_STATUS_TRANSOUT},
						"zzh-book-002 b7":  BookCopy{Status: COPY_STATUS_INSTOCK},
						"zzh-book-002 b8":  BookCopy{Status: COPY_STATUS_INSTOCK},
						"zzh-book-002 b9":  BookCopy{Status: COPY_STATUS_INSTOCK},
						"zzh-book-002 b10": BookCopy{Status: COPY_STATUS_INSTOCK},
					},
					Relations: Relations{
						REL_BOOK_PUBLIC: booksPids[1]["pub_id"],
					},
				},
				nil,
			},
		}

	}

	someBooksInDB := resetSomeBooksInDB()

	type mockChannel struct {
		postIdType   string
		chid         string
		createdCount int
		updatedCount int
		deletedCount int
		result       []*model.Post
		deletedIds   map[string]bool
	}

	initMockChannel := func() []mockChannel {
		return []mockChannel{
			{
				"pub_id",
				td.BookChIdPub,
				0,
				0,
				0,
				[]*model.Post{},
				map[string]bool{},
			},
			{
				"pri_id",
				td.BookChIdPri,
				0,
				0,
				0,
				[]*model.Post{},
				map[string]bool{},
			},
			{
				"inv_id",
				td.BookChIdInv,
				0,
				0,
				0,
				[]*model.Post{},
				map[string]bool{},
			},
		}
	}

	mockChannels := initMockChannel()

	// 	findMockChannel := func(postIdType string) *mockChannel {
	//
	// 		for _, channel := range mockChannels {
	// 			if channel.postIdType == postIdType {
	// 				return &channel
	// 			}
	// 		}
	// 		return nil
	// 	}

	// findMCByChType := func(chtype string) *mockChannel {
	// 	for _, ch := range mockChannels {
	// 		if ch.postIdType == chtype {
	// 			return &ch
	// 		}
	// 	}
	// 	return nil
	// }
	resetMockChannels := func(mc []mockChannel) {
		resetTo := initMockChannel()
		for i := range mc {
			mc[i] = resetTo[i]
		}
	}

	type errControl struct {
		created  bool
		update   bool
		delete   bool
		get      bool
		notfound bool
	}

	type errControls map[string]errControl

	initErrControl := func() []errControls {
		return []errControls{
			{
				td.BookChIdPub: errControl{},
				td.BookChIdPri: errControl{},
				td.BookChIdInv: errControl{},
			},
			{
				td.BookChIdPub: errControl{},
				td.BookChIdPri: errControl{},
				td.BookChIdInv: errControl{},
			},
		}
	}

	errctrls := initErrControl()

	var block0 chan struct{}
	var block1 chan struct{}
	var once sync.Once

	api.On("GetPost", mock.AnythingOfType("string")).Return(
		func(id string) *model.Post {
			booksPartById := map[string]interface{}{
				booksPids[0]["pub_id"]: someBooksInDB[0].BookPublic,
				booksPids[0]["pri_id"]: someBooksInDB[0].BookPrivate,
				booksPids[0]["inv_id"]: someBooksInDB[0].BookInventory,
				booksPids[1]["pub_id"]: someBooksInDB[1].BookPublic,
				booksPids[1]["pri_id"]: someBooksInDB[1].BookPrivate,
				booksPids[1]["inv_id"]: someBooksInDB[1].BookInventory,
			}
			if block0 != nil {
				once.Do(func() {
					if block1 != nil {
						block1 <- struct{}{}
					}
					block0 <- struct{}{}
				})
			}
			post := &model.Post{
				Id:     id,
				UserId: plugin.botID,
				Type:   "custom_book_type",
			}
			switch part := booksPartById[id].(type) {
			case *BookPublic:
				j, _ := json.MarshalIndent(part, "", "  ")
				post.ChannelId = td.BookChIdPub
				post.Message = string(j)
			case *BookPrivate:
				j, _ := json.MarshalIndent(part, "", "  ")
				post.ChannelId = td.BookChIdPri
				post.Message = string(j)
			case *BookInventory:
				j, _ := json.MarshalIndent(part, "", "  ")
				post.ChannelId = td.BookChIdInv
				post.Message = string(j)
			}
			return post
		},
		func(id string) *model.AppError {
			index := findIndexById(id)
			chid := booksChidByPid[id]
			if errctrls[index][chid].notfound {
				return model.NewAppError("GetSinglePost", "app.post.get.app_error", nil, "", http.StatusNotFound)

			}
			return nil

		})
	for i := range mockChannels {
		mockChannelPtr := &mockChannels[i]

		api.On("CreatePost", mock.MatchedBy(td.MatchPostByChannel(mockChannelPtr.chid))).Return(
			func(post *model.Post) *model.Post {
				assert.Equalf(t, plugin.botID, post.UserId, "should be bot id")
				if booksChidByPid[post.Id] == td.BookChIdPub {
					assert.Equalf(t, "custom_book_type", post.Type, "should be custom_book_type")
				}
				if !errctrls[mockChannelPtr.createdCount][mockChannelPtr.chid].created {
					post.Id = booksPids[mockChannelPtr.createdCount][mockChannelPtr.postIdType]
					return post
				}
				return nil
			},
			func(post *model.Post) *model.AppError {
				//increaing count must be placed here, because error func is the lastest
				if errctrls[mockChannelPtr.createdCount][mockChannelPtr.chid].created {
					return &model.AppError{}
				}
				mockChannelPtr.createdCount++
				return nil

			})

		api.On("UpdatePost", mock.MatchedBy(td.MatchPostByChannel(mockChannelPtr.chid))).Return(
			func(post *model.Post) *model.Post {
				index := findIndexById(post.Id)
				if !errctrls[index][mockChannelPtr.chid].update {
					mockChannelPtr.result = append(mockChannelPtr.result, post)
					switch mockChannelPtr.postIdType {
					case "pub_id":
						var pub BookPublic
						json.Unmarshal([]byte(post.Message), &pub)
						*someBooksInDB[index].BookPublic = pub
					case "pri_id":
						var pri BookPrivate
						json.Unmarshal([]byte(post.Message), &pri)
						*someBooksInDB[index].BookPrivate = pri
					case "inv_id":
						var inv BookInventory
						json.Unmarshal([]byte(post.Message), &inv)
						*someBooksInDB[index].BookInventory = inv

					}
					return post
				}
				return nil
			},
			func(post *model.Post) *model.AppError {
				index := findIndexById(post.Id)
				if errctrls[index][mockChannelPtr.chid].update {
					return &model.AppError{}
				}
				mockChannelPtr.updatedCount++
				return nil

			})

		api.On("DeletePost",
			booksPids[mockChannelPtr.createdCount][mockChannelPtr.postIdType]).Return(
			func(id string) *model.AppError {
				index := findIndexById(id)
				if errctrls[index][mockChannelPtr.chid].delete {
					mockChannelPtr.deletedCount++
					return &model.AppError{}
				}
				mockChannelPtr.deletedCount++
				mockChannelPtr.deletedIds[id] = true
				return nil
			},
		)
	}

	assertPublic := func(t *testing.T, expPub BookPublic, actPub BookPublic, last string) {
		assert.NotEmpty(t, actPub.MatchId)
		actPub.MatchId = ""
		expPub.MatchId = ""
		assert.Equalf(t, expPub, actPub, "public part")

	}

	t.Run("create_normal", func(t *testing.T) {

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
		// fmt.Println(string(reqJson))
		plugin.ServeHTTP(nil, w, r)

		// validate messages
		_checkBookMessageResult(t, w, false, map[string]BooksMessage{
			"zzh-book-001": {
				PostId: booksPids[0]["pub_id"],
				Status: BOOK_UPLOAD_SUCC,
			},
			"zzh-book-002": {
				PostId: booksPids[1]["pub_id"],
				Status: BOOK_UPLOAD_SUCC,
			},
		})

		var expectBooks Books
		DeepCopy(&expectBooks, &someBooksInDB)

		// validate create
		for i, somebook := range expectBooks {
			for _, mockChannel := range mockChannels {
				msg := mockChannel.result[i].Message
				postType := mockChannel.result[i].Type

				switch mockChannel.postIdType {
				case "pub_id":
					assert.Equal(t, "custom_book_type", postType, "post.type should be BookType")
					bookpub := new(BookPublic)
					json.Unmarshal([]byte(msg), bookpub)
					assertPublic(t, *somebook.BookPublic, *bookpub, resetLastUpdated)
				case "pri_id":
					assert.Equal(t, "custom_book_private_type", postType, "post.type should be BookPrivateType")
					bookpri := new(BookPrivate)
					json.Unmarshal([]byte(msg), bookpri)
					assert.Equalf(t, somebook.BookPrivate, bookpri, "private part")
				case "inv_id":
					assert.Equal(t, "custom_book_inventory_type", postType, "post.type should be BookInventoryType")
					bookinv := new(BookInventory)
					json.Unmarshal([]byte(msg), bookinv)
					//initial expection status
					somebook.Stock = someBooksUpl[i].Stock
					somebook.TransmitOut = 0
					somebook.TransmitIn = 0
					somebook.Lending = 0
					somebook.Copies = someBooksUpl[i].Copies

					assert.Equalf(t, somebook.BookInventory, bookinv, "inventory part")
				}
			}
		}
	})

	t.Run("create_error", func(t *testing.T) {

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
		someBooksInDB = resetSomeBooksInDB()
		resetMockChannels(mockChannels)
		// mockChannels = initMockChannel()

		errctrls = []errControls{
			{
				td.BookChIdPub: errControl{},
				td.BookChIdPri: errControl{},
				td.BookChIdInv: errControl{},
			},
			{
				td.BookChIdPub: {created: true},
				td.BookChIdPri: errControl{},
				td.BookChIdInv: errControl{},
			},
		}

		plugin.ServeHTTP(nil, w, r)

		// validate messages
		_checkBookMessageResult(t, w, true, map[string]BooksMessage{
			"zzh-book-001": {
				PostId: booksPids[0]["pub_id"],
				Status: BOOK_UPLOAD_SUCC,
			},
			"zzh-book-002": {
				PostId: "",
				Status: BOOK_UPLOAD_ERROR,
			},
		})

	})

	t.Run("create_rollback", func(t *testing.T) {

		type delcnt map[string]int

		for i, test := range []struct {
			erc    errControls
			delcnt delcnt
		}{
			{
				errControls{
					td.BookChIdPub: errControl{created: true},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{},
				},
				delcnt{
					td.BookChIdPub: 0,
					td.BookChIdPri: 0,
					td.BookChIdInv: 0,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{created: true},
					td.BookChIdInv: errControl{},
				},
				delcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 0,
					td.BookChIdInv: 0,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{created: true},
				},
				delcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 1,
					td.BookChIdInv: 0,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{update: true},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{},
				},
				delcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 1,
					td.BookChIdInv: 1,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{update: true},
					td.BookChIdInv: errControl{},
				},
				delcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 1,
					td.BookChIdInv: 1,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{update: true},
				},
				delcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 1,
					td.BookChIdInv: 1,
				},
			},
		} {
			_ = i

			booksJson, _ := json.Marshal(Books{someBooksUpl[0]})

			req := BooksRequest{
				Action:  BOOKS_ACTION_UPLOAD,
				ActUser: td.ABook.LibworkerNames[0],
				Body:    string(booksJson),
			}

			reqJson, _ := json.Marshal(req)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

			//check result
			// mockChannels = initMockChannel()
			resetMockChannels(mockChannels)
			someBooksInDB = resetSomeBooksInDB()

			errctrls = []errControls{test.erc}
			plugin.ServeHTTP(nil, w, r)

			// validate messages
			_checkBookMessageResult(t, w, true, map[string]BooksMessage{
				"zzh-book-001": {
					PostId: "",
					Status: BOOK_UPLOAD_ERROR,
				},
			})

			for _, mockChannel := range mockChannels {
				assert.Equalf(t, test.delcnt[mockChannel.chid], mockChannel.deletedCount, mockChannel.postIdType)
			}
		}

	})

	t.Run("update_normal", func(t *testing.T) {

		someBooksInDB = resetSomeBooksInDB()
		var theseBooksUpl Books
		//there is map in, so have to use deepcopy
		DeepCopy(&theseBooksUpl, &someBooksUpl)
		theseBooksUpl[0].BookPublic.Author = "new Author"
		//don't update isAllowedToBorrow when updating
		theseBooksUpl[0].BookPublic.IsAllowedToBorrow = false
		theseBooksUpl[0].BookPrivate.KeeperUsers = []string{td.ABook.KeeperUsers[1]}
		theseBooksUpl[0].BookInventory.Stock = 8
		theseBooksUpl[0].BookInventory.Copies["zzh-book-001 b8"] = BookCopy{Status: COPY_STATUS_INSTOCK}

		theseBooksUpl[1].BookPublic.Author = "new Author 2"
		//don't update isAllowedToBorrow when updating
		theseBooksUpl[1].BookPublic.IsAllowedToBorrow = false
		theseBooksUpl[1].BookPrivate.KeeperUsers = []string{td.ABook.KeeperUsers[0]}
		theseBooksUpl[1].BookInventory.Stock = 6
		delete(theseBooksUpl[1].Copies, "zzh-book-002 b7")
		delete(theseBooksUpl[1].Copies, "zzh-book-002 b8")
		delete(theseBooksUpl[1].Copies, "zzh-book-002 b9")
		delete(theseBooksUpl[1].Copies, "zzh-book-002 b10")

		var expectBooks Books
		DeepCopy(&expectBooks, &someBooksInDB)
		expectBooks[0].BookPublic.Author = "new Author"
		expectBooks[0].BookPrivate.KeeperUsers = []string{td.ABook.KeeperUsers[1]}
		expectBooks[0].BookPrivate.KeeperInfos = KeeperInfoMap{
			td.ABook.KeeperUsers[1]: td.ABook.KeeperInfos[td.ABook.KeeperUsers[1]],
		}
		expectBooks[0].BookInventory.Stock = 4
		expectBooks[0].BookInventory.TransmitOut = 2
		expectBooks[0].BookInventory.Lending = 1
		expectBooks[0].BookInventory.TransmitIn = 1
		expectBooks[0].BookInventory.Copies = BookCopies{
			"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_TRANSIN},
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_LENDING},
			"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_TRANSOUT},
			"zzh-book-001 b4": BookCopy{Status: COPY_STATUS_TRANSOUT},
			"zzh-book-001 b5": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b6": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b8": BookCopy{Status: COPY_STATUS_INSTOCK},
		}

		expectBooks[1].BookPublic.Author = "new Author 2"
		expectBooks[1].BookPrivate.KeeperUsers = []string{td.ABook.KeeperUsers[0]}
		expectBooks[1].BookPrivate.KeeperInfos = KeeperInfoMap{
			td.ABook.KeeperUsers[0]: td.ABook.KeeperInfos[td.ABook.KeeperUsers[0]],
		}
		expectBooks[1].BookInventory.Stock = 0
		expectBooks[1].BookInventory.TransmitOut = 3
		expectBooks[1].BookInventory.Lending = 2
		expectBooks[1].BookInventory.TransmitIn = 1
		expectBooks[1].BookInventory.Copies = BookCopies{
			"zzh-book-002 b1": BookCopy{Status: COPY_STATUS_TRANSIN},
			"zzh-book-002 b2": BookCopy{Status: COPY_STATUS_LENDING},
			"zzh-book-002 b3": BookCopy{Status: COPY_STATUS_LENDING},
			"zzh-book-002 b4": BookCopy{Status: COPY_STATUS_TRANSOUT},
			"zzh-book-002 b5": BookCopy{Status: COPY_STATUS_TRANSOUT},
			"zzh-book-002 b6": BookCopy{Status: COPY_STATUS_TRANSOUT},
		}

		for _, test := range []struct {
			postids []string
			created map[string]int
			updated map[string]int
		}{
			{
				[]string{booksPids[0]["pub_id"], booksPids[1]["pub_id"]},
				map[string]int{
					"pub_id": 0,
					"pri_id": 0,
					"inv_id": 0,
				},
				map[string]int{
					"pub_id": 2,
					"pri_id": 2,
					"inv_id": 2,
				},
			},
		} {
			resetMockChannels(mockChannels)
			someBooksInDB = resetSomeBooksInDB()

			theseBooksUpl[0].Upload = &Upload{
				Post_id: test.postids[0],
			}

			theseBooksUpl[1].Upload = &Upload{
				Post_id: test.postids[1],
			}

			booksJson, _ := json.Marshal(theseBooksUpl)
			req := BooksRequest{
				Action:  BOOKS_ACTION_UPLOAD,
				ActUser: td.ABook.LibworkerNames[0],
				Body:    string(booksJson),
			}

			reqJson, _ := json.Marshal(req)

			// fmt.Println(string(reqJson))

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

			someBooksInDB[0].BookPublic.IsAllowedToBorrow = false
			someBooksInDB[1].BookPublic.IsAllowedToBorrow = true

			errctrls = initErrControl()

			plugin.ServeHTTP(nil, w, r)

			// validate messages
			_checkBookMessageResult(t, w, false, map[string]BooksMessage{
				"zzh-book-001": {
					PostId: booksPids[0]["pub_id"],
					Status: BOOK_UPLOAD_SUCC,
				},
				"zzh-book-002": {
					PostId: booksPids[1]["pub_id"],
					Status: BOOK_UPLOAD_SUCC,
				},
			})

			for _, mockChannel := range mockChannels {
				switch mockChannel.postIdType {
				case "pub_id":
					assert.Equalf(t, test.created["pub_id"], mockChannel.createdCount, "no creation of pub_id")
					assert.Equalf(t, test.updated["pub_id"], mockChannel.updatedCount, "no updated of pub_id")
				case "pri_id":
					assert.Equalf(t, test.created["pri_id"], mockChannel.createdCount, "no creation of pri_id")
					assert.Equalf(t, test.updated["pri_id"], mockChannel.updatedCount, "no updated of pri_id")
				case "inv_id":
					assert.Equalf(t, test.created["inv_id"], mockChannel.createdCount, "no creation of inv_id")
					assert.Equalf(t, test.updated["inv_id"], mockChannel.updatedCount, "no updated of inv_id")
				}
			}

			//Setting expection
			for i, thisbook := range expectBooks {
				for _, mockChannel := range mockChannels {
					msg := mockChannel.result[i].Message

					switch mockChannel.postIdType {
					case "pub_id":
						var bookpub *BookPublic
						json.Unmarshal([]byte(msg), &bookpub)
						assertPublic(t, *thisbook.BookPublic, *bookpub, resetLastUpdated)
					case "pri_id":
						var bookpri *BookPrivate
						json.Unmarshal([]byte(msg), &bookpri)
						assert.Equalf(t, thisbook.BookPrivate, bookpri, "private part %v", i)
					case "inv_id":
						var bookinv *BookInventory
						json.Unmarshal([]byte(msg), &bookinv)
						assert.Equalf(t, thisbook.BookInventory, bookinv, "inventory part %v", i)
					}
				}
			}

		}

	})

	t.Run("update_borrow_allowed_force", func(t *testing.T) {
		someBooksInDB = resetSomeBooksInDB()
		someBooksInDB[0].BookPublic.IsAllowedToBorrow = true
		someBooksInDB[0].BookPublic.ManuallyDisallowed = false
		someBooksInDB[1].BookPublic.IsAllowedToBorrow = false
		someBooksInDB[1].BookPublic.ReasonOfDisallowed = "some reason"
		someBooksInDB[1].BookPublic.ManuallyDisallowed = true

		resetMockChannels(mockChannels)

		var theseBooksUpl Books
		DeepCopy(&theseBooksUpl, &someBooksUpl)
		theseBooksUpl[0].BookPublic.IsAllowedToBorrow = false
		theseBooksUpl[0].Upload = &Upload{
			Post_id:              booksPids[0]["pub_id"],
			UpdIsAllowedToBorrow: true,
		}

		theseBooksUpl[1].BookPublic.IsAllowedToBorrow = true
		theseBooksUpl[1].BookPublic.ReasonOfDisallowed = "some reason"
		theseBooksUpl[1].Upload = &Upload{
			Post_id:              booksPids[1]["pub_id"],
			UpdIsAllowedToBorrow: true,
		}

		var expectBooks Books
		DeepCopy(&expectBooks, &someBooksInDB)
		expectBooks[0].BookPublic.IsAllowedToBorrow = false
		expectBooks[0].BookPublic.ManuallyDisallowed = true
		expectBooks[1].BookPublic.IsAllowedToBorrow = true
		expectBooks[1].BookPublic.ReasonOfDisallowed = ""
		expectBooks[1].BookPublic.ManuallyDisallowed = false

		booksJson, _ := json.Marshal(theseBooksUpl)
		req := BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerNames[0],
			Body:    string(booksJson),
		}

		reqJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

		errctrls = initErrControl()

		plugin.ServeHTTP(nil, w, r)

		// validate messages
		_checkBookMessageResult(t, w, false, map[string]BooksMessage{
			"zzh-book-001": {
				PostId: booksPids[0]["pub_id"],
				Status: BOOK_UPLOAD_SUCC,
			},
			"zzh-book-002": {
				PostId: booksPids[1]["pub_id"],
				Status: BOOK_UPLOAD_SUCC,
			},
		})

	NEXTBOOK:
		for i, thisbook := range expectBooks {
			for _, mockChannel := range mockChannels {
				msg := mockChannel.result[i].Message

				switch mockChannel.postIdType {
				case "pub_id":
					var bookpub *BookPublic
					json.Unmarshal([]byte(msg), &bookpub)
					assertPublic(t, *thisbook.BookPublic, *bookpub, resetLastUpdated)
					continue NEXTBOOK
				default:
				}
			}
		}
	})

	t.Run("update_pub_only", func(t *testing.T) {
		resetMockChannels(mockChannels)
		someBooksInDB = resetSomeBooksInDB()
		errctrls = initErrControl()

		var thisBookUpl Book
		//there is map in, so have to use deepcopy
		DeepCopy(&thisBookUpl, &someBooksUpl[0])

		thisBookUpl.BookPublic.Author = "New Author"

		thisBookUpl.BookPrivate = nil
		thisBookUpl.BookInventory = nil
		thisBookUpl.Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
		}

		var theseBooksUpl Books
		theseBooksUpl = append(theseBooksUpl, thisBookUpl)

		booksJson, _ := json.Marshal(theseBooksUpl)

		req := BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerNames[0],
			Body:    string(booksJson),
		}

		reqJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

		plugin.ServeHTTP(nil, w, r)

		for _, mockChannel := range mockChannels {
			switch mockChannel.postIdType {
			case "pub_id":
				assert.Equalf(t, 1, mockChannel.updatedCount, "no updated of pub_id")
			case "pri_id":
				assert.Equalf(t, 0, mockChannel.updatedCount, "no updated of pri_id")
			case "inv_id":
				assert.Equalf(t, 0, mockChannel.updatedCount, "no updated of inv_id")
			}
		}

	})

	t.Run("update_lock", func(t *testing.T) {
		someBooksInDB = resetSomeBooksInDB()
		theseBooks := Books{
			someBooksUpl[0],
		}

		theseBooks[0].Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
		}

		booksJson, _ := json.Marshal(theseBooks)

		req := BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerNames[0],
			Body:    string(booksJson),
		}

		reqJson, _ := json.Marshal(req)

		block0 = make(chan struct{})
		block1 = make(chan struct{})
		defer func() {
			close(block0)
			close(block1)
			block0 = nil
			block1 = nil
		}()

		var wait sync.WaitGroup
		wait.Add(2)

		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		go func() {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
			plugin.ServeHTTP(nil, w, r)
			// validate messages
			_checkBookMessageResult(t, w, false, map[string]BooksMessage{
				"zzh-book-001": {
					PostId: booksPids[0]["pub_id"],
					Status: BOOK_UPLOAD_SUCC,
				},
			})
			wait.Done()
		}()

		go func() {
			<-block1
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
			plugin.ServeHTTP(nil, w, r)

			// validate messages
			_checkBookMessageResult(t, w, true, map[string]BooksMessage{
				"zzh-book-001": {
					PostId: booksPids[0]["pub_id"],
					Status: BOOK_UPLOAD_ERROR,
				},
			})
			for _, mockChannel := range mockChannels {
				assert.Equalf(t, 0, mockChannel.createdCount, "channel &v's created count should be 0", mockChannel.chid)
				assert.Equalf(t, 0, mockChannel.updatedCount, "channel &v's updated count should be 0", mockChannel.chid)
			}

			<-block0
			wait.Done()
		}()

		wait.Wait()

	})

	t.Run("update_stale", func(t *testing.T) {

		var thisBookUpl Book
		//there is map in, so have to use deepcopy
		DeepCopy(&thisBookUpl, &someBooksUpl[0])

		thisBookUpl.BookPublic.Author = "New Author"

		thisBookUpl.BookPrivate = nil
		thisBookUpl.BookInventory = nil
		thisBookUpl.Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
			Etag:    "123456",
		}

		var theseBooksUpl Books
		theseBooksUpl = append(theseBooksUpl, thisBookUpl)

		booksJson, _ := json.Marshal(theseBooksUpl)

		// 		req := BooksRequest{
		// 			Action:  BOOKS_ACTION_UPLOAD,
		// 			ActUser: td.ABook.LibworkerNames[0],
		// 			Body:    string(booksJson),
		// 		}
		//
		// 		reqJson, _ := json.Marshal(req)
		//
		// 		w := httptest.NewRecorder()
		// 		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
		//
		// 		resetMockChannels(mockChannels)
		// 		someBooksInDB = resetSomeBooksInDB()
		// 		errctrls = initErrControl()
		//
		// 		plugin.ServeHTTP(nil, w, r)
		//
		// 		_checkBookMessageResult(t, w, true, map[string]BooksMessage{
		// 			"zzh-book-001": {
		// 				PostId:  booksPids[0]["pub_id"],
		// 				Status:  BOOK_UPLOAD_ERROR,
		// 				Message: plugin.i18n.GetText("system-locking"),
		// 			},
		// 		})

		sendARequestAndCheck(t, BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerUsers[0],
			Body:    string(booksJson),
		}, true, map[string]BooksMessage{
			"zzh-book-001": {
				PostId:  booksPids[0]["pub_id"],
				Status:  BOOK_UPLOAD_ERROR,
				Message: plugin.i18n.GetText("system-locking"),
			},
		})
	})

	t.Run("update_rollback", func(t *testing.T) {

		var theseBooksUpl Books
		DeepCopy(&theseBooksUpl, &someBooksUpl)

		theseBooksUpl = Books{theseBooksUpl[0]}
		theseBooksUpl[0].BookPublic.Author = "new Author"
		theseBooksUpl[0].BookPrivate.KeeperUsers = []string{td.ABook.KeeperUsers[1]}
		theseBooksUpl[0].BookInventory.Stock = 10
		theseBooksUpl[0].Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
		}

		type updcnt map[string]int

		for _, test := range []struct {
			erc    errControls
			updcnt updcnt
		}{
			{
				errControls{
					td.BookChIdPub: errControl{update: true},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{},
				},
				updcnt{
					td.BookChIdPub: 0,
					td.BookChIdPri: 0,
					td.BookChIdInv: 0,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{update: true},
					td.BookChIdInv: errControl{},
				},
				updcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 0,
					td.BookChIdInv: 0,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{update: true},
				},
				updcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 1,
					td.BookChIdInv: 0,
				},
			},
		} {
			//check result
			// mockChannels = initMockChannel()
			resetMockChannels(mockChannels)
			someBooksInDB = resetSomeBooksInDB()

			booksJson, _ := json.Marshal(theseBooksUpl)

			req := BooksRequest{
				Action:  BOOKS_ACTION_UPLOAD,
				ActUser: td.ABook.LibworkerNames[0],
				Body:    string(booksJson),
			}

			reqJson, _ := json.Marshal(req)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

			errctrls = []errControls{test.erc}
			plugin.ServeHTTP(nil, w, r)

			// Validation
			_checkBookMessageResult(t, w, true, map[string]BooksMessage{
				"zzh-book-001": {
					PostId: booksPids[0]["pub_id"],
					Status: BOOK_UPLOAD_ERROR,
				},
			})

			for _, mockChannel := range mockChannels {
				assert.Equalf(t, 0, mockChannel.createdCount, "create count should be zero")

				if test.updcnt[mockChannel.chid] == 0 {
					assert.Equalf(t, 0, mockChannel.updatedCount, "update count should be zero in channel:%v", mockChannel.postIdType)
				}
				if test.updcnt[mockChannel.chid] == 1 {
					post := mockChannel.result[len(mockChannel.result)-1]

					switch mockChannel.postIdType {
					case "pub_id":
						var pub BookPublic
						json.Unmarshal([]byte(post.Message), &pub)
						assert.Equalf(t, someBooksInDB[0].BookPublic, &pub, "pub should be rollbacked")
					case "pri_id":
						var pri BookPrivate
						json.Unmarshal([]byte(post.Message), &pri)
						assert.Equalf(t, someBooksInDB[0].BookPrivate, &pri, "pri should be rollbacked")
					case "inv_id":
						var inv BookInventory
						json.Unmarshal([]byte(post.Message), &inv)
						assert.Equalf(t, someBooksInDB[0].BookInventory, &inv, "inv should be rollbacked")
					}
				}
			}
		}
	})

	t.Run("update stock to be avialble", func(t *testing.T) {

		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		someBooksInDB = resetSomeBooksInDB()
		someBooksInDB[0].BookPublic.IsAllowedToBorrow = false
		someBooksInDB[0].BookPublic.ManuallyDisallowed = false
		someBooksInDB[0].BookInventory.Stock = 0
		someBooksInDB[0].BookInventory.Lending = 1
		someBooksInDB[0].BookInventory.TransmitOut = 0
		someBooksInDB[0].BookInventory.TransmitIn = 0
		someBooksInDB[0].BookInventory.Copies = BookCopies{
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_LENDING},
		}

		someBooksInDB[1].BookPublic.IsAllowedToBorrow = false
		someBooksInDB[1].BookPublic.ManuallyDisallowed = true
		someBooksInDB[1].BookInventory.Stock = 0
		someBooksInDB[1].BookInventory.Lending = 1
		someBooksInDB[1].BookInventory.TransmitOut = 0
		someBooksInDB[1].BookInventory.TransmitIn = 0
		someBooksInDB[1].BookInventory.Copies = BookCopies{
			"zzh-book-002 b3": BookCopy{Status: COPY_STATUS_LENDING},
		}

		var theseBooksUpl Books
		DeepCopy(&theseBooksUpl, &someBooksUpl)
		theseBooksUpl[0].BookInventory.Stock = 2
		theseBooksUpl[0].BookInventory.Copies = BookCopies{
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
		}
		theseBooksUpl[0].Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
		}
		theseBooksUpl[1].BookInventory.Stock = 2
		theseBooksUpl[1].BookInventory.Copies = BookCopies{
			"zzh-book-002 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-002 b4": BookCopy{Status: COPY_STATUS_INSTOCK},
		}
		theseBooksUpl[1].Upload = &Upload{
			Post_id: booksPids[1]["pub_id"],
		}

		var expectBooks Books
		DeepCopy(&expectBooks, &someBooksInDB)
		expectBooks[0].BookPublic.IsAllowedToBorrow = true
		expectBooks[0].BookPublic.ManuallyDisallowed = false
		expectBooks[0].BookInventory.Stock = 1
		expectBooks[0].BookInventory.Lending = 1
		expectBooks[0].BookInventory.Copies = BookCopies{
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_LENDING},
			"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
		}

		expectBooks[1].BookPublic.IsAllowedToBorrow = false
		expectBooks[1].BookPublic.ManuallyDisallowed = true
		expectBooks[1].BookInventory.Stock = 1
		expectBooks[1].BookInventory.Lending = 1
		expectBooks[1].BookInventory.Copies = BookCopies{
			"zzh-book-002 b3": BookCopy{Status: COPY_STATUS_LENDING},
			"zzh-book-002 b4": BookCopy{Status: COPY_STATUS_INSTOCK},
		}

		booksJson, _ := json.Marshal(theseBooksUpl)

		req := BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerNames[0],
			Body:    string(booksJson),
		}

		reqJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

		plugin.ServeHTTP(nil, w, r)

		assertPublic(t, *expectBooks[0].BookPublic, *someBooksInDB[0].BookPublic, resetLastUpdated)
		// assert.Equalf(t, expectBooks[0].BookPublic, someBooksInDB[0].BookPublic, "public part")
		// assert.Equalf(t, expectBooks[1].BookPublic, someBooksInDB[1].BookPublic, "public part")
		assertPublic(t, *expectBooks[1].BookPublic, *someBooksInDB[1].BookPublic, resetLastUpdated)
		assert.Equalf(t, expectBooks[0].BookInventory, someBooksInDB[0].BookInventory, "inventory part")
		assert.Equalf(t, expectBooks[1].BookInventory, someBooksInDB[1].BookInventory, "inventory part")

	})

	t.Run("update_stock_error_delete_not_stock", func(t *testing.T) {

		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		someBooksInDB = resetSomeBooksInDB()
		someBooksInDB[0].BookInventory.Stock = 0
		someBooksInDB[0].BookInventory.Lending = 1
		someBooksInDB[0].BookInventory.TransmitOut = 0
		someBooksInDB[0].BookInventory.TransmitIn = 0
		someBooksInDB[0].BookInventory.Copies = BookCopies{
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_LENDING},
		}

		var theseBooksUpl Books
		DeepCopy(&theseBooksUpl, &someBooksUpl)
		theseBooksUpl[0].BookInventory.Stock = 1
		theseBooksUpl[0].BookInventory.Copies = BookCopies{
			"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
		}
		theseBooksUpl[0].Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
		}

		booksJson, _ := json.Marshal(Books{theseBooksUpl[0]})

		req := BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerNames[0],
			Body:    string(booksJson),
		}

		reqJson, _ := json.Marshal(req)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

		plugin.ServeHTTP(nil, w, r)

		_checkBookMessageResult(t, w, true, map[string]BooksMessage{
			"zzh-book-001": {
				PostId:  booksPids[0]["pub_id"],
				Status:  BOOK_UPLOAD_ERROR,
				Message: fmt.Sprintf("cannot delete copy %v with status %v is not InStock", "zzh-book-001 b2", COPY_STATUS_LENDING),
			},
		})

	})

	t.Run("delete_normal", func(t *testing.T) {
		someBooksInDB = resetSomeBooksInDB()

		someBooksInDB[0].BookInventory.Stock = 7
		someBooksInDB[0].BookInventory.TransmitIn = 0
		someBooksInDB[0].BookInventory.TransmitOut = 0
		someBooksInDB[0].BookInventory.Lending = 0
		someBooksInDB[0].BookInventory.Copies = BookCopies{
			"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b4": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b5": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b6": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
		}
		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		theseBooksUpl := someBooksUpl
		theseBooksUpl[0].Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
			Delete:  true,
		}

		booksJson, _ := json.Marshal(theseBooksUpl)

		req := BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerNames[0],
			Body:    string(booksJson),
		}

		reqJson, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
		plugin.ServeHTTP(nil, w, r)

		// Validation
		_checkBookMessageResult(t, w, false, map[string]BooksMessage{
			"zzh-book-001": {
				PostId: booksPids[0]["pub_id"],
				Status: BOOK_UPLOAD_SUCC,
			},
		})

		for _, mockChannel := range mockChannels {
			assert.Equalf(t, 1, mockChannel.createdCount, "create count should be 1")
			assert.Equalf(t, 1, mockChannel.updatedCount, "update count should be 1")
			assert.Equalf(t, 1, mockChannel.deletedCount, "delete count should be 1")
			switch mockChannel.postIdType {
			case "pub_id":
				assert.Equalf(t, true, mockChannel.deletedIds[booksPids[0]["pub_id"]], "index 0's pub_id should be deleted")
			case "pri_id":
				assert.Equalf(t, true, mockChannel.deletedIds[booksPids[0]["pri_id"]], "index 0's pri_id should be deleted")
			case "inv_id":
				assert.Equalf(t, true, mockChannel.deletedIds[booksPids[0]["inv_id"]], "index 0's inv_id should be deleted")
			}
		}

	})

	t.Run("delete error(not all books were returned)", func(t *testing.T) {
		someBooksInDB = resetSomeBooksInDB()
		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		theseBooksUpl := someBooksUpl
		theseBooksUpl[0].Upload = &Upload{
			Post_id: booksPids[0]["pub_id"],
			Delete:  true,
		}

		booksJson, _ := json.Marshal(theseBooksUpl)

		req := BooksRequest{
			Action:  BOOKS_ACTION_UPLOAD,
			ActUser: td.ABook.LibworkerNames[0],
			Body:    string(booksJson),
		}

		reqJson, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))
		plugin.ServeHTTP(nil, w, r)

		// Validation
		_checkBookMessageResult(t, w, true, map[string]BooksMessage{
			"zzh-book-001": {
				PostId: booksPids[0]["pub_id"],
				Status: BOOK_UPLOAD_ERROR,
			},
		})

		for _, mockChannel := range mockChannels {
			assert.Equalf(t, 0, mockChannel.deletedCount, "delete count should be 0")
		}

	})

	t.Run("delete empty", func(t *testing.T) {

		someBooksInDB = resetSomeBooksInDB()

		someBooksInDB[0].Stock = 7
		someBooksInDB[0].TransmitOut = 0
		someBooksInDB[0].Lending = 0
		someBooksInDB[0].TransmitIn = 0
		someBooksInDB[0].Copies = BookCopies{
			"zzh-book-001 b1": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b2": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b3": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b4": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b5": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b6": BookCopy{Status: COPY_STATUS_INSTOCK},
			"zzh-book-001 b7": BookCopy{Status: COPY_STATUS_INSTOCK},
		}

		type delcnt map[string]int

		for _, test := range []struct {
			erc    errControls
			delcnt delcnt
		}{
			{
				errControls{
					td.BookChIdPub: errControl{notfound: true},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{},
				},
				delcnt{
					td.BookChIdPub: 0,
					td.BookChIdPri: 0,
					td.BookChIdInv: 0,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{notfound: true},
					td.BookChIdInv: errControl{},
				},
				delcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 0,
					td.BookChIdInv: 1,
				},
			},
			{
				errControls{
					td.BookChIdPub: errControl{},
					td.BookChIdPri: errControl{},
					td.BookChIdInv: errControl{notfound: true},
				},
				delcnt{
					td.BookChIdPub: 1,
					td.BookChIdPri: 1,
					td.BookChIdInv: 0,
				},
			},
		} {
			var theseBooksUpl Books
			DeepCopy(&theseBooksUpl, &someBooksUpl)
			theseBooksUpl[0].Upload = &Upload{
				Post_id: booksPids[0]["pub_id"],
				Delete:  true,
			}

			booksJson, _ := json.Marshal(Books{theseBooksUpl[0]})
			theseBooksUpl[0].Upload = &Upload{
				Post_id: booksPids[0]["pub_id"],
				Delete:  true,
			}

			req := BooksRequest{
				Action:  BOOKS_ACTION_UPLOAD,
				ActUser: td.ABook.LibworkerNames[0],
				Body:    string(booksJson),
			}

			reqJson, _ := json.Marshal(req)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/books", bytes.NewReader(reqJson))

			//check result
			resetMockChannels(mockChannels)

			errctrls = []errControls{test.erc}
			plugin.ServeHTTP(nil, w, r)

			// validate messages
			if test.erc[td.BookChIdPub].notfound {
				_checkBookMessageResult(t, w, true, map[string]BooksMessage{
					"zzh-book-001": {
						PostId: theseBooksUpl[0].Upload.Post_id,
						Status: BOOK_UPLOAD_ERROR,
					},
				})
			} else {

				_checkBookMessageResult(t, w, false, map[string]BooksMessage{
					"zzh-book-001": {
						PostId: theseBooksUpl[0].Upload.Post_id,
						Status: BOOK_UPLOAD_SUCC,
					},
				})
			}

			for _, mockChannel := range mockChannels {
				assert.Equalf(t, test.delcnt[mockChannel.chid], mockChannel.deletedCount, mockChannel.postIdType)
			}
		}

	})

	t.Run("fetch inv", func(t *testing.T) {
		someBooksInDB = resetSomeBooksInDB()
		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		key := fmt.Sprintf(`[
                  {
                    "post_id": "%v"
                  }
                ]`, booksPids[0]["pub_id"])

		var b1 Book
		DeepCopy(&b1, someBooksInDB[0])

		copyKeeperMap := map[string]Keeper{}
		bookCopies := BookCopies{}
		for copyid, keeper := range b1.CopyKeeperMap {
			if keeper.User == td.ABook.KeeperUsers[0] {
				copyKeeperMap[copyid] = keeper
				bookCopies[copyid] = b1.Copies[copyid]
			}
		}

		b1.BookPrivate = &BookPrivate{
			KeeperInfos: KeeperInfoMap{
				td.ABook.KeeperUsers[0]: td.ABook.KeeperInfos[td.ABook.KeeperUsers[0]],
			},
			CopyKeeperMap: copyKeeperMap,
		}

		b1.BookInventory = &BookInventory{
			Copies: bookCopies,
		}

		result, _ := json.Marshal(b1)

		sendARequestAndCheck(t, BooksRequest{
			Action:  BOOKS_ACTION_FETCH_INV_KEEPER,
			ActUser: td.ABook.KeeperUsers[0],
			Body:    key,
		}, false, map[string]BooksMessage{
			"zzh-book-001": {
				booksPids[0]["pub_id"],
				BOOK_ACTION_SUCC,
				string(result),
			},
		})
	})

	t.Run("fetch inv by supervisor", func(t *testing.T) {
		someBooksInDB = resetSomeBooksInDB()
		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		key := fmt.Sprintf(`[
                  {
                    "post_id": "%v"
                  }
                ]`, booksPids[0]["pub_id"])

		var b1 Book
		DeepCopy(&b1, someBooksInDB[0])

		result, _ := json.Marshal(b1)

		sendARequestAndCheck(t, BooksRequest{
			Action:  BOOKS_ACTION_FETCH_INV_KEEPER,
			ActUser: td.ABook.LibworkerUsers[0],
			Body:    key,
		}, false, map[string]BooksMessage{
			"zzh-book-001": {
				booksPids[0]["pub_id"],
				BOOK_ACTION_SUCC,
				string(result),
			},
		})
	})

	t.Run("fetch inv not by supervisor", func(t *testing.T) {
		someBooksInDB = resetSomeBooksInDB()
		resetMockChannels(mockChannels)
		errctrls = initErrControl()

		key := fmt.Sprintf(`[
                  {
                    "post_id": "%v"
                  }
                ]`, booksPids[0]["pub_id"])

		var b1 Book
		DeepCopy(&b1, someBooksInDB[0])

		copyKeeperMap := map[string]Keeper{}
		bookCopies := BookCopies{}

		b1.BookPrivate = &BookPrivate{
			KeeperInfos:   KeeperInfoMap{},
			CopyKeeperMap: copyKeeperMap,
		}

		b1.BookInventory = &BookInventory{
			Copies: bookCopies,
		}

		result, _ := json.Marshal(b1)

		sendARequestAndCheck(t, BooksRequest{
			Action:  BOOKS_ACTION_FETCH_INV_KEEPER,
			ActUser: td.ABook.LibworkerUsers[1],
			Body:    key,
		}, false, map[string]BooksMessage{
			"zzh-book-001": {
				booksPids[0]["pub_id"],
				BOOK_ACTION_SUCC,
				string(result),
			},
		})
	})
}
