package main

import (
	"fmt"
	// "time"

	semver "github.com/blang/semver/v4"
	// "github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
	// "github.com/mattermost/mattermost-plugin-api/cluster"
)

const minimumServerVersion = "5.30.0"

func (p *Plugin) OnActivate() error {
	if err := p.checkServerVersion(); err != nil {
		return err
	}
	if err := p.OnConfigurationChange(); err != nil {
		return err
	}

	// conf := p.getConfiguration()
        
	if err := p.registerCommands(); err != nil {
		return errors.Wrap(err, "failed to register commands")
	}

	return nil
}

func (p *Plugin) checkServerVersion() error {
	serverVersion, err := semver.Parse(p.API.GetServerVersion())
	if err != nil {
		return errors.Wrap(err, "failed to parse server version")
	}

	r := semver.MustParseRange(">=" + minimumServerVersion)
	if !r(serverVersion) {
		return fmt.Errorf("this plugin requires Mattermost v%s or later", minimumServerVersion)
	}

	return nil
}

// OnDeactivate is invoked when the plugin is deactivated. This is the plugin's last chance to use
// the API, and the plugin will be terminated shortly after this invocation.
func (p *Plugin) OnDeactivate() error {
  // we don't put any clean login here, so as to preserve data, or to prevent some miss operation(accidently deactivate plugin .e.g.)
  // if you want do some clean work, you have to do it mannually
  // 1. Delete Channel
  // 2. Delete team
  // 3. Delete bot
  return nil
}
