package provider

import (
	"github.com/rs/zerolog"
	"github.com/sudosu404/providers/agent/pkg/agent"
	"github.com/sudosu404/providers/internal/route"
	"github.com/sudosu404/providers/internal/watcher"
	gperr "github.com/sudosu404/tailnet-utils/errs"
)

type AgentProvider struct {
	*agent.AgentConfig
	docker ProviderImpl
}

func (p *AgentProvider) ShortName() string {
	return p.AgentConfig.Name
}

func (p *AgentProvider) NewWatcher() watcher.Watcher {
	return p.docker.NewWatcher()
}

func (p *AgentProvider) IsExplicitOnly() bool {
	return p.docker.IsExplicitOnly()
}

func (p *AgentProvider) loadRoutesImpl() (route.Routes, gperr.Error) {
	return p.docker.loadRoutesImpl()
}

func (p *AgentProvider) Logger() *zerolog.Logger {
	return p.docker.Logger()
}
