package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/yusing/go-proxy/agent/pkg/agent"
	"github.com/yusing/go-proxy/agent/pkg/env"
	"github.com/yusing/go-proxy/internal/metrics/systeminfo"
	"github.com/yusing/go-proxy/internal/nerdctl"
	"github.com/yusing/go-proxy/pkg"
	socketproxy "github.com/yusing/go-proxy/socketproxy/pkg"
)

type ServeMux struct{ *http.ServeMux }

func (mux ServeMux) HandleEndpoint(method, endpoint string, handler http.HandlerFunc) {
	mux.ServeMux.HandleFunc(method+" "+agent.APIEndpointBase+endpoint, handler)
}

func (mux ServeMux) HandleFunc(endpoint string, handler http.HandlerFunc) {
	mux.ServeMux.HandleFunc(agent.APIEndpointBase+endpoint, handler)
}

var upgrader = &websocket.Upgrader{
	// no origin check needed for internal websocket
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewAgentHandler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	mux := ServeMux{http.NewServeMux()}

	metricsHandler := gin.Default()
	{
		metrics := metricsHandler.Group(agent.APIEndpointBase)
		metrics.GET(agent.EndpointSystemInfo, func(c *gin.Context) {
			c.Set("upgrader", upgrader)
			systeminfo.Poller.ServeHTTP(c)
		})
	}

	mux.HandleFunc(agent.EndpointProxyHTTP+"/{path...}", ProxyHTTP)
	mux.HandleEndpoint("GET", agent.EndpointVersion, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, pkg.GetVersion())
	})
	mux.HandleEndpoint("GET", agent.EndpointName, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, env.AgentName)
	})
	mux.HandleEndpoint("GET", agent.EndpointRuntime, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, env.Runtime)
	})
	mux.HandleEndpoint("GET", agent.EndpointHealth, CheckHealth)
	mux.HandleEndpoint("GET", agent.EndpointSystemInfo, metricsHandler.ServeHTTP)
	switch env.Runtime {
	case agent.ContainerRuntimeDocker, agent.ContainerRuntimePodman:
		mux.ServeMux.HandleFunc("/", socketproxy.DockerSocketHandler(env.DockerSocket))
	case agent.ContainerRuntimeNerdctl:
		mux.HandleEndpoint("GET", agent.EndpointListContainers, func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()

			containers, err := nerdctl.ListContainers(ctx, "unix://"+env.DockerSocket)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(containers)
		})
		// For nerdctl/containerd, we need TCP-level multiplexing
		// This will be handled at the server level, not here
		// Just provide a fallback handler
		mux.ServeMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Containerd requests should be handled at TCP level", http.StatusNotImplemented)
		})
	}
	return mux
}
