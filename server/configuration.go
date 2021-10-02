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
	BooksPrivateChannelName   string
	BooksInventoryChannelName string
	BorrowWorkflowChannelName string
	BorrowLimit               int
	InitialAdmin              string
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

	// initial admin is must

	// ensure team
	team, appErr := p.ensureTeam(configuration.TeamName)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure team.")
	}
	p.team = team

	// ensure books public channel
	bkchannel, appErr := p.ensureChannel(team.Id, configuration.BooksChannelName, "Books", "Channel for books library.", model.CHANNEL_OPEN)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure channel")
	}
	p.booksChannel = bkchannel

	// ensure books private channel
	bkprichannel, appErr := p.ensureChannel(team.Id, configuration.BooksPrivateChannelName,
		"Books Private", "Channel for books private infomation.", model.CHANNEL_PRIVATE)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure channel")
	}
	p.booksPriChannel = bkprichannel

	// ensure books inventory channel
	bkinvchannel, appErr := p.ensureChannel(team.Id, configuration.BooksInventoryChannelName,
		"Books Inventory", "Channel for books inventory infomation.", model.CHANNEL_PRIVATE)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure channel")
	}
	p.booksInvChannel = bkinvchannel

	// ensure workflow
	bchannel, appErr := p.ensureChannel(team.Id, configuration.BorrowWorkflowChannelName, "Borrows", "Channel for borrowing workflow.", model.CHANNEL_PRIVATE)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to ensure channel")
	}
	p.borrowChannel = bchannel

	p.borrowTimes = configuration.BorrowLimit

	// assign initial admin
	if configuration.InitialAdmin != "" {
		admin, appErr := p.API.GetUserByUsername(configuration.InitialAdmin)
		if appErr != nil {
			return errors.Wrap(appErr, "failed to find initial admin")
		}

		if appErr := p.ensureMemberInTeam(team, admin); appErr != nil {
			return errors.Wrap(appErr, "failed to add member to books team")
		}

                err := p.ensureMemberInChannel(bkchannel, admin)
		if err != nil {
			return errors.Wrap(err, "failed to assign inital user to book channel")
		}

		err = p.ensureMemberInChannel(bkprichannel, admin)
		if err != nil {
			return errors.Wrap(err, "failed to assign inital user to book channel(private)")
		}

		err = p.ensureMemberInChannel(bkinvchannel, admin)
		if err != nil {
			return errors.Wrap(err, "failed to assign inital user to book channel(inventory)")
		}

		err = p.ensureMemberInChannel(bchannel, admin)
		if err != nil {
			return errors.Wrap(err, "failed to assign inital user to borrow channel")
		}
	}
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
		TeamId:      teamid,
		Name:        name,
		DisplayName: displayName,
		Purpose:     purpose,
		Type:        chtype,
	})

	if appErr != nil {
		return nil, errors.Wrapf(appErr, "faield to create channel with name: %s", name)
	}
	return channel, nil
}

func (p *Plugin) ensureMemberInTeam(team *model.Team, admin *model.User) error {

	_, appErr := p.API.GetTeamMember(team.Id, admin.Id)

	if appErr != nil && appErr.Id != "app.team.get_member.missing.app_error" {
		return errors.Wrapf(appErr, "failed to get team member")
	}

	if appErr == nil {
		return nil
	}

	_, appErr = p.API.CreateTeamMember(team.Id, admin.Id)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to assign inital user")
	}

	return nil
}

func (p *Plugin) ensureMemberInChannel(channel *model.Channel, admin *model.User) error {

	_, appErr := p.API.GetChannelMember(channel.Id, admin.Id)

	if appErr != nil && appErr.Id != "app.team.get_member.missing.app_error" {
		return errors.Wrapf(appErr, "failed to get channel member")
	}

	if appErr == nil {
		return nil
	}

	_, appErr = p.API.AddChannelMember(channel.Id, admin.Id)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to assign inital user to channel")
	}

	return nil
}
