package config

import (
	"slices"

	"github.com/yusing/go-proxy/agent/pkg/agent"
	"github.com/yusing/go-proxy/internal/gperr"
	"github.com/yusing/go-proxy/internal/route/provider"
)

func (cfg *Config) VerifyNewAgent(host string, ca agent.PEMPair, client agent.PEMPair) (int, gperr.Error) {
	if slices.ContainsFunc(cfg.value.Providers.Agents, func(a *agent.AgentConfig) bool {
		return a.Addr == host
	}) {
		return 0, gperr.New("agent already exists")
	}

	agentCfg := agent.AgentConfig{
		Addr: host,
	}
	err := agentCfg.StartWithCerts(cfg.Task().Context(), ca.Cert, client.Cert, client.Key)
	if err != nil {
		return 0, gperr.Wrap(err, "failed to start agent")
	}

	provider := provider.NewAgentProvider(&agentCfg)
	if _, loaded := cfg.providers.LoadOrStore(provider.String(), provider); loaded {
		return 0, gperr.Errorf("provider %s already exists", provider.String())
	}

	err = provider.LoadRoutes()
	if err != nil {
		return 0, gperr.Wrap(err, "failed to load routes")
	}

	agent.AddAgent(&agentCfg)
	return provider.NumRoutes(), nil
}
