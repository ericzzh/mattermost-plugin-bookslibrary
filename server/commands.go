package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"

	// "strconv"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

const (
	commandPostTestBook   = "post_test_book"
	commandPostTestBorrow = "post_test_borrow"
)

func (p *Plugin) registerCommands() error {

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          commandPostTestBook,
		AutoComplete:     true,
		AutoCompleteDesc: "Post a test book.",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandPostTestBook)
	}

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          commandPostTestBorrow,
		AutoComplete:     true,
		AutoCompleteDesc: "Post test borrow.",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandPostTestBook)
	}
	return nil
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	switch trigger {
	case commandPostTestBook:
		return p.executePostBook(args), nil
	case commandPostTestBorrow:
		return p.executePostBorrow(args), nil
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

	if len(argsarr) < 3 {
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "no borrow count.",
		}
	}

	count, _ := strconv.Atoi(argsarr[1])

	userName := argsarr[2]
	bookPostId := argsarr[3]

	borReq := fmt.Sprintf(`{
          "book_post_id":"%v",
          "borrower_user":"%v"
        }`, bookPostId, userName)

	for i := 0; i < count; i++ {
		w := httptest.NewRecorder()
		r, _ := http.NewRequest("POST", "/borrow", bytes.NewReader([]byte(borReq)))
		p.ServeHTTP(nil, w, r)

		res := new(Result)
		json.NewDecoder(w.Result().Body).Decode(&res)
		if res.Error != "" {

			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         fmt.Sprintf("error.  Message:%v", res.Messages),
			}
		}
	}

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         fmt.Sprintf("Succ."),
	}

}
