package main

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"net/http"
	"net/url"
	"sync"
)

const (
	PLUGIN_ID = "com.github.ericzzh.mattermost-plugin-bookslibrary"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	botID string

	team *model.Team

	borrowChannel *model.Channel

	booksChannel    *model.Channel
	booksPriChannel *model.Channel
	booksInvChannel *model.Channel

	borrowTimes int

	maxRenewTimes int
	expiredDays   int
        
        i18n *i18n
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
	  	http.Redirect(w, r, *p.API.GetConfig().ServiceSettings.SiteURL+"/login?redirect_to="+url.QueryEscape(r.RequestURI), http.StatusFound)
	    // http.Error(w, "Not authorized", http.StatusUnauthorized)
      return
	}
	switch r.URL.Path {
	case "/borrow":
		p.handleBorrowRequest(c, w, r)
	case "/workflow":
		p.handleWorkflowRequest(c, w, r)
	case "/books":
		p.handleBooksRequest(c, w, r)
	case "/config":
		p.handleConfigRequest(c, w, r)
	default:
		http.NotFound(w, r)
	}
}
