package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	// "strconv"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

const (
	commandRoot         = "books_library"
	commandPostBook     = "post_book"
	commandPostBorrow   = "post_borrow"
	commandPostTestBook = "post_test_book"
)

func (p *Plugin) registerCommands() error {
	// if err := p.API.RegisterCommand(&model.Command{
	// 	Trigger:          commandRoot,
	// 	AutoComplete:     true,
	// 	AutoCompleteDesc: "Books library commands",
	// 	AutocompleteData: getCommandRoot(),
	// }); err != nil {
	// 	return errors.Wrapf(err, "failed to register %s command", commandRoot)
	// }

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          commandPostBook,
		AutoComplete:     true,
		AutoCompleteDesc: "Post a book.",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandPostBook)
	}

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          commandPostTestBook,
		AutoComplete:     true,
		AutoCompleteDesc: "Post test book.",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandPostTestBook)
	}
	return nil
}

// func getCommandRoot() *model.AutocompleteData {
//
// 	command := model.NewAutocompleteData(commandRoot, "", "Books library commands")
//
// 	postbook := model.NewAutocompleteData(commandPostBook, "", "Post a book from json input.")
// 	command.AddCommand(postbook)
//
// 	postborrow := model.NewAutocompleteData(commandPostBorrow, "", "Post a borrow record from json input")
// 	command.AddCommand(postborrow)
//
//         return command
//
// }

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	switch trigger {
	case commandPostBook:
		return p.executePostBook(args), nil
	case commandPostBorrow:
		return p.executePostBorrow(args), nil
	case commandPostTestBook:
		return p.executePostTestBook(args), nil
	default:
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         fmt.Sprintf("Unknown command: " + args.Command),
		}, nil
	}
}

func (p *Plugin) executePostBook(args *model.CommandArgs) *model.CommandResponse {
	argsarr := strings.Fields(args.Command)

	if len(argsarr) < 1 {
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "no file args.",
		}
	}

	path := filepath.Join("plugins", PLUGIN_ID, "assets", argsarr[1])
	booksJsonStr, err := ioutil.ReadFile(path)
	if err != nil {
		wd, _ := os.Getwd()
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         fmt.Sprintf("Failed to load books. path:%v, pwd:%v, err:%v", path, wd, err),
		}
	}


	messages, err := p._uploadBooks(string(booksJsonStr))
	if err != nil {
		// p.API.LogError("Failded uplolad.", "json", brqJson)
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         fmt.Sprintf("Message:%v. Error:%v. Json:%v", messages, err, string(booksJsonStr)),
		}
	}

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         fmt.Sprintf("Succ.  Message:%v", messages),
	}

}

func (p *Plugin) executePostBorrow(args *model.CommandArgs) *model.CommandResponse {
	argsarr := strings.Fields(args.Command)

	var borrowStr string
	if len(argsarr) > 1 {
		borrowStr = argsarr[1]
		var borrowData BorrowRequest
		if err := json.Unmarshal([]byte(borrowStr), &borrowData); err != nil {
			p.API.LogError("Failed to convert from a borrow json string.", "err", err.Error())
			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         "Failed to convert from a borrow json string.",
			}
		}

	} else {

		if borrow_data_bytes, err := json.Marshal(BorrowRequest{}); err != nil {
			p.API.LogError("Failed to initialize a borrow record.", "err", err.Error())
			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         "Failed to initialize a borrow record.",
			}
		} else {
			borrowStr = string(borrow_data_bytes)
		}

	}

	if _, appErr := p.API.CreatePost(&model.Post{
		UserId:    p.botID,
		ChannelId: p.borrowChannel.Id,
		Message:   borrowStr,
		Type:      "custom_borrow_type",
	}); appErr != nil {
		p.API.LogError("Failed to post a borrow record.", "err", appErr.Error())
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "Failed to post a borrow record.",
		}
	}
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         "Posted a borrow record.",
	}
}

func (p *Plugin) executePostTestBook(args *model.CommandArgs) *model.CommandResponse {
	// 	argsarr := strings.Fields(args.Command)
	//
	// 	var counts_str string
	// 	var counts int
	// 	var err error
	//
	// 	if len(argsarr) > 1 {
	// 		counts_str = argsarr[1]
	// 		if counts, err = strconv.Atoi(counts_str); err != nil {
	// 			p.API.LogError("Failed to convert count to number", "err", err.Error())
	// 			return &model.CommandResponse{
	// 				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
	// 				Text:         "Failed to convert from a book json string.",
	// 			}
	// 		}
	//
	// 	} else {
	//
	// 		counts = 100
	//
	// 	}
	//
	// 	for i := 0; i < counts; i++ {
	// 		var bookDataStr string
	// 		if book_data_bytes, err := json.MarshalIndent(Book{
	// 			BookPublic{
	// 				Name:      fmt.Sprintf("Book-%d", i),
	// 				Id:        strconv.Itoa(i),
	// 				Category1: "C1",
	// 				Category2: "C2",
	// 				Category3: "C3",
	// 			},
	// 			BookPrivate{},
	// 		}, "", ""); err != nil {
	// 			p.API.LogError("Failed to initialize a book.", "err", err.Error())
	// 			return &model.CommandResponse{
	// 				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
	// 				Text:         "Failed to initialize a book.",
	// 			}
	// 		} else {
	// 			bookDataStr = string(book_data_bytes)
	// 		}
	//
	// 		if _, appErr := p.API.CreatePost(&model.Post{
	// 			UserId:    p.botID,
	// 			ChannelId: p.booksChannel.Id,
	// 			Message:   bookDataStr,
	// 			Type:      "custom_book_type",
	// 		}); appErr != nil {
	// 			p.API.LogError("Failed to post a book", "err", appErr.Error())
	// 			return &model.CommandResponse{
	// 				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
	// 				Text:         "Failed to post a book.",
	// 			}
	// 		}
	//
	// 		// _ = p.API.SendEphemeralPost(args.UserId, &model.Post{
	// 		// 	UserId:    p.botID,
	// 		// 	ChannelId: p.booksChannel.Id,
	// 		// 	Message:   bookDataStr,
	// 		// 	Props: model.StringInterface{
	// 		// 		"type": "custom_book_type",
	// 		// 	},
	// 		// })
	// 	}
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         "Posted test books.[link](http://localhost:8065/bookslibrary/pl/5wcfjp5jubrid8f58cc171fg7a)",
	}
}
