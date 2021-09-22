package main

import (
	// "fmt"
	"reflect"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type configuration struct {
	TeamName                  string
	BooksChannelName          string
	BorrowWorkflowChannelName string
        BorrowLimit               int
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)

	// ensure book library bot

	botID, ensureBotError := p.Helpers.EnsureBot(&model.Bot{
		Username:    "bookslibrary",
		DisplayName: "Books Library Bot",
		Description: "A bot account created by books library plugin.",
	})
	if ensureBotError != nil {
		return errors.Wrap(ensureBotError, "failed to ensure books libary bot.")
	}

	p.botID = botID

	team, appErr := p.ensureTeam(configuration.TeamName)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure team.")
	}
	p.team = team

	bkchannel, appErr := p.ensureChannel(team.Id, configuration.BooksChannelName,"Books", "Channel for books library.", model.CHANNEL_OPEN)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure channel")
	}
	p.booksChannel = bkchannel

	bchannel, appErr := p.ensureChannel(team.Id, configuration.BorrowWorkflowChannelName, "Borrows","Channel for borrowing workflow.", model.CHANNEL_PRIVATE)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure channel")
	}
	p.borrowChannel = bchannel

        p.borrowTimes = configuration.BorrowLimit

	return nil
}

func (p *Plugin) ensureTeam(name string) (*model.Team, error) {

	// fmt.Printf("********* book library debug.. config team %s", name)

	team, appErr := p.API.GetTeamByName(name)

	if appErr != nil && appErr.Id != "app.team.get_by_name.missing.app_error" {
		return nil, errors.Wrapf(appErr, "failed to get Team by name:%s", name)
	}

	if appErr == nil {
		return team, nil
	}

	team, appErr = p.API.CreateTeam(&model.Team{
		Name:        name,
                DisplayName: "Books Library",
		Description: "Books Library",
		Type:        model.TEAM_INVITE,
	})

	if appErr != nil {
		return nil, errors.Wrapf(appErr, "failed to create team with name:%s", name)
	}

	return team, nil

}

func (p *Plugin) ensureChannel(teamid, name, displayName, purpose, chtype string) (*model.Channel, error) {
	channel, appErr := p.API.GetChannelByName(teamid, name, false)

	if appErr != nil && appErr.Id != "app.channel.get_by_name.missing.app_error" {
		return nil, errors.Wrapf(appErr, "failed to get channel by name: %s", name)
	}

	if appErr == nil {
		return channel, nil
	}
	channel, appErr = p.API.CreateChannel(&model.Channel{
		TeamId:  teamid,
		Name:    name,
                DisplayName: displayName,
  		Purpose: purpose,
		Type:    chtype,
	})

	if appErr != nil {
		return nil, errors.Wrapf(appErr, "faield to create channel with name: %s", name)
	}
	return channel, nil
}
