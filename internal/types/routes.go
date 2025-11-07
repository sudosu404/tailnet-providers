package types

import (
	"net/http"

	"github.com/sudosu404/providers/agent/pkg/agent"
	"github.com/sudosu404/providers/internal/homepage"
	nettypes "github.com/sudosu404/providers/internal/net/types"
	provider "github.com/sudosu404/providers/internal/route/provider/types"
	"github.com/sudosu404/providers/internal/utils/pool"
	gperr "github.com/sudosu404/go-utils/errs"
	"github.com/sudosu404/go-utils/http/reverseproxy"
	"github.com/sudosu404/go-utils/task"
)

type (
	Route interface {
		task.TaskStarter
		task.TaskFinisher
		pool.Object
		ProviderName() string
		GetProvider() RouteProvider
		TargetURL() *nettypes.URL
		HealthMonitor() HealthMonitor
		SetHealthMonitor(m HealthMonitor)
		References() []string
		ShouldExclude() bool

		Started() <-chan struct{}

		IdlewatcherConfig() *IdlewatcherConfig
		HealthCheckConfig() *HealthCheckConfig
		LoadBalanceConfig() *LoadBalancerConfig
		HomepageItem() homepage.Item
		DisplayName() string
		ContainerInfo() *Container

		GetAgent() *agent.AgentConfig

		IsDocker() bool
		IsAgent() bool
		UseLoadBalance() bool
		UseIdleWatcher() bool
		UseHealthCheck() bool
		UseAccessLog() bool
	}
	HTTPRoute interface {
		Route
		http.Handler
	}
	ReverseProxyRoute interface {
		HTTPRoute
		ReverseProxy() *reverseproxy.ReverseProxy
	}
	StreamRoute interface {
		Route
		nettypes.Stream
		Stream() nettypes.Stream
	}
	RouteProvider interface {
		Start(task.Parent) gperr.Error
		LoadRoutes() gperr.Error
		GetRoute(alias string) (r Route, ok bool)
		// should be used like `for _, r := range p.IterRoutes` (no braces), not calling it directly
		IterRoutes(yield func(alias string, r Route) bool)
		NumRoutes() int
		FindService(project, service string) (r Route, ok bool)
		Statistics() ProviderStats
		GetType() provider.Type
		ShortName() string
		String() string
	}
)
