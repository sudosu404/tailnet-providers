package config

import (
	config "github.com/yusing/go-proxy/internal/config/types"
	"github.com/yusing/go-proxy/internal/route/provider"
	"github.com/yusing/go-proxy/internal/types"
)

func (cfg *Config) DumpRouteProviders() map[string]*provider.Provider {
	entries := make(map[string]*provider.Provider, cfg.providers.Size())
	for _, p := range cfg.providers.Range {
		entries[p.ShortName()] = p
	}
	return entries
}

func (cfg *Config) RouteProviderList() []config.RouteProviderListResponse {
	list := make([]config.RouteProviderListResponse, 0, cfg.providers.Size())
	for _, p := range cfg.providers.Range {
		list = append(list, config.RouteProviderListResponse{
			ShortName: p.ShortName(),
			FullName:  p.String(),
		})
	}
	return list
}

func (cfg *Config) SearchRoute(alias string) types.Route {
	for _, p := range cfg.providers.Range {
		if r, ok := p.GetRoute(alias); ok {
			return r
		}
	}
	return nil
}

func (cfg *Config) Statistics() map[string]any {
	var rps, streams types.RouteStats
	var total uint16
	providerStats := make(map[string]types.ProviderStats)

	for _, p := range cfg.providers.Range {
		stats := p.Statistics()
		providerStats[p.ShortName()] = stats
		rps.AddOther(stats.RPs)
		streams.AddOther(stats.Streams)
		total += stats.RPs.Total + stats.Streams.Total
	}

	return map[string]any{
		"total":           total,
		"reverse_proxies": rps,
		"streams":         streams,
		"providers":       providerStats,
	}
}
