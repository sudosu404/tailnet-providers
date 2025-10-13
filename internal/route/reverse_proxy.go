package route

import (
	"net/http"
	"sync"

	"github.com/yusing/godoxy/agent/pkg/agent"
	"github.com/yusing/godoxy/agent/pkg/agentproxy"
	"github.com/yusing/godoxy/internal/idlewatcher"
	"github.com/yusing/godoxy/internal/logging/accesslog"
	gphttp "github.com/yusing/godoxy/internal/net/gphttp"
	"github.com/yusing/godoxy/internal/net/gphttp/loadbalancer"
	"github.com/yusing/godoxy/internal/net/gphttp/middleware"
	nettypes "github.com/yusing/godoxy/internal/net/types"
	"github.com/yusing/godoxy/internal/route/routes"
	"github.com/yusing/godoxy/internal/types"
	"github.com/yusing/godoxy/internal/watcher/health/monitor"
	gperr "github.com/yusing/goutils/errs"
	"github.com/yusing/goutils/http/reverseproxy"
	"github.com/yusing/goutils/task"
	"github.com/yusing/goutils/version"
)

type ReveseProxyRoute struct {
	*Route

	loadBalancer *loadbalancer.LoadBalancer
	handler      http.Handler
	rp           *reverseproxy.ReverseProxy
}

var _ types.ReverseProxyRoute = (*ReveseProxyRoute)(nil)

// var globalMux    = http.NewServeMux() // TODO: support regex subdomain matching.

func NewReverseProxyRoute(base *Route) (*ReveseProxyRoute, gperr.Error) {
	httpConfig := base.HTTPConfig
	proxyURL := base.ProxyURL

	var trans *http.Transport
	a := base.GetAgent()
	if a != nil {
		trans = a.Transport()
		proxyURL = nettypes.NewURL(agent.HTTPProxyURL)
	} else {
		tlsConfig, err := httpConfig.BuildTLSConfig(&base.ProxyURL.URL)
		if err != nil {
			return nil, err
		}

		trans = gphttp.NewTransportWithTLSConfig(tlsConfig)
		if httpConfig.ResponseHeaderTimeout > 0 {
			trans.ResponseHeaderTimeout = httpConfig.ResponseHeaderTimeout
		}
		if httpConfig.DisableCompression {
			trans.DisableCompression = true
		}
	}

	service := base.Name()
	rp := reverseproxy.NewReverseProxy(service, &proxyURL.URL, trans)

	if len(base.Middlewares) > 0 {
		err := middleware.PatchReverseProxy(rp, base.Middlewares)
		if err != nil {
			return nil, err
		}
	}

	if a != nil {
		cfg := agentproxy.Config{
			Scheme:     base.ProxyURL.Scheme,
			Host:       base.ProxyURL.Host,
			HTTPConfig: httpConfig,
		}
		setHeaderFunc := cfg.SetAgentProxyConfigHeaders
		if !a.Version.IsOlderThan(version.New(0, 18, 6)) {
			setHeaderFunc = cfg.SetAgentProxyConfigHeadersLegacy
		}

		ori := rp.HandlerFunc
		rp.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			setHeaderFunc(r.Header)
			ori(w, r)
		}
	}

	r := &ReveseProxyRoute{
		Route: base,
		rp:    rp,
	}
	return r, nil
}

// ReverseProxy implements routes.ReverseProxyRoute.
func (r *ReveseProxyRoute) ReverseProxy() *reverseproxy.ReverseProxy {
	return r.rp
}

// Start implements task.TaskStarter.
func (r *ReveseProxyRoute) Start(parent task.Parent) gperr.Error {
	r.task = parent.Subtask("http."+r.Name(), false)

	switch {
	case r.UseIdleWatcher():
		waker, err := idlewatcher.NewWatcher(parent, r, r.IdlewatcherConfig())
		if err != nil {
			r.task.Finish(err)
			return gperr.Wrap(err)
		}
		r.handler = waker
		r.HealthMon = waker
	case r.UseHealthCheck():
		r.HealthMon = monitor.NewMonitor(r)
	}

	if r.handler == nil {
		r.handler = r.rp
	}

	if r.UseAccessLog() {
		var err error
		r.rp.AccessLogger, err = accesslog.NewAccessLogger(r.task, r.AccessLog)
		if err != nil {
			r.task.Finish(err)
			return gperr.Wrap(err)
		}
	}

	if len(r.Rules) > 0 {
		r.handler = r.Rules.BuildHandler(r.handler.ServeHTTP)
	}

	if r.HealthMon != nil {
		if err := r.HealthMon.Start(r.task); err != nil {
			return err
		}
	}

	if r.ShouldExclude() {
		return nil
	}

	if r.UseLoadBalance() {
		r.addToLoadBalancer(parent)
	} else {
		routes.HTTP.Add(r)
		r.task.OnCancel("remove_route_from_http", func() {
			routes.HTTP.Del(r)
		})
	}
	return nil
}

func (r *ReveseProxyRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}

var lbLock sync.Mutex

func (r *ReveseProxyRoute) addToLoadBalancer(parent task.Parent) {
	var lb *loadbalancer.LoadBalancer
	cfg := r.LoadBalance
	lbLock.Lock()

	l, ok := routes.HTTP.Get(cfg.Link)
	var linked *ReveseProxyRoute
	if ok {
		lbLock.Unlock()
		linked = l.(*ReveseProxyRoute)
		lb = linked.loadBalancer
		lb.UpdateConfigIfNeeded(cfg)
		if linked.Homepage.Name == "" {
			linked.Homepage = r.Homepage
		}
	} else {
		lb = loadbalancer.New(cfg)
		_ = lb.Start(parent) // always return nil
		linked = &ReveseProxyRoute{
			Route: &Route{
				Alias:    cfg.Link,
				Homepage: r.Homepage,
			},
			loadBalancer: lb,
			handler:      lb,
		}
		linked.SetHealthMonitor(lb)
		routes.HTTP.AddKey(cfg.Link, linked)
		r.task.OnFinished("remove_loadbalancer_route", func() {
			routes.HTTP.DelKey(cfg.Link)
		})
		lbLock.Unlock()
	}
	r.loadBalancer = lb

	server := loadbalancer.NewServer(r.task.Name(), r.ProxyURL, r.LoadBalance.Weight, r.handler, r.HealthMon)
	lb.AddServer(server)
	r.task.OnCancel("lb_remove_server", func() {
		lb.RemoveServer(server)
	})
}
