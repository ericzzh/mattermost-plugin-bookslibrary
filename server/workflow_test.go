package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	// "github.com/mattermost/mattermost-server/v5/model"
)

func TestHandleWorkflow(t *testing.T) {

	td := NewTestData()

	api := td.ApiMockCommon()
	plugin := td.NewMockPlugin()
	plugin.SetAPI(api)

	realbrPosts, realbrUpdPosts, createdPid := GenerateBorrowRequest(td, plugin, api)

	var master *Borrow
	json.Unmarshal([]byte(realbrPosts[td.BorChannelId].Message), master)

	worker := master.DataOrImage.LibworkerUser
	var worker_botId string
	if worker == "worker1" {
		worker_botId = td.Worker1Id_botId
	} else {
		worker_botId = td.Worker2Id_botId
	}

	for _, channelId := range []string{
		td.BorChannelId,
		td.BorId_botId,
		worker_botId,
		td.Keeper1Id_botId,
		td.Keeper2Id_botId,
	} {
		api.On("GetPost", td.MatchPostById(createdPid[channelId])).
			Return(realbrUpdPosts[createdPid[channelId]])
	}

	t.Run("normal", func(t *testing.T) {

		wfr := WorkflowRequest{
			MasterPostKey: createdPid[td.BorChannelId],
			ActUser:       worker,
			MoveToStatus:  STATUS_CONFIRMED,
		}

                wfrJson, _ := json.Marshal(wfr)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/workflow", bytes.NewReader(wfrJson))
		plugin.ServeHTTP(nil, w, r)

	})

}
