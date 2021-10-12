package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	// "github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	// "github.com/pkg/errors"
)

func (p *Plugin) handleConfigRequest(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	config := Config{
		MaxRenewTimes: p.maxRenewTimes,
		ExpiredDays:   p.expiredDays,
	}

	data, err := json.Marshal(config)
	if err != nil {
		p.API.LogError("mashal config error", "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "mashal config error",
		})

		w.Write(resp)
		return
	}

	msg := Messages{}
	msg["data"] = string(data)

	res := Result{
		Error:    "",
		Messages: msg,
	}

	resJson, err := json.Marshal(res)
	if err != nil {

		p.API.LogError("mashal result error", "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "mashal result error",
		})

		w.Write(resp)
		return
	}

	w.Write(resJson)

}
