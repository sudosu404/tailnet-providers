package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yusing/go-proxy/agent/pkg/certs"
	"github.com/yusing/go-proxy/pkg"
	"google.golang.org/grpc/credentials"
)

type AgentConfig struct {
	Addr    string           `json:"addr"`
	Name    string           `json:"name"`
	Version string           `json:"version"`
	Runtime ContainerRuntime `json:"runtime"`

	httpClient *http.Client
	tlsConfig  *tls.Config
	l          zerolog.Logger
} // @name Agent

const (
	EndpointVersion        = "/version"
	EndpointName           = "/name"
	EndpointRuntime        = "/runtime"
	EndpointProxyHTTP      = "/proxy/http"
	EndpointHealth         = "/health"
	EndpointLogs           = "/logs"
	EndpointSystemInfo     = "/system_info"
	EndpointListContainers = "/containers/list" // nerdctl only

	AgentHost = CertsDNSName

	APIEndpointBase = "/godoxy/agent"
	APIBaseURL      = "https://" + AgentHost + APIEndpointBase

	DockerHost = "https://" + AgentHost

	FakeDockerHostPrefix    = "agent://"
	FakeDockerHostPrefixLen = len(FakeDockerHostPrefix)
)

func mustParseURL(urlStr string) *url.URL {
	u, err := url.Parse(urlStr)
	if err != nil {
		panic(err)
	}
	return u
}

var (
	AgentURL              = mustParseURL(APIBaseURL)
	HTTPProxyURL          = mustParseURL(APIBaseURL + EndpointProxyHTTP)
	HTTPProxyURLPrefixLen = len(APIEndpointBase + EndpointProxyHTTP)
)

func IsDockerHostAgent(dockerHost string) bool {
	return strings.HasPrefix(dockerHost, FakeDockerHostPrefix)
}

func GetAgentAddrFromDockerHost(dockerHost string) string {
	return dockerHost[FakeDockerHostPrefixLen:]
}

func (cfg *AgentConfig) FakeDockerHost() string {
	return FakeDockerHostPrefix + cfg.Addr
}

func (cfg *AgentConfig) Parse(addr string) error {
	cfg.Addr = addr
	return nil
}

var serverVersion = pkg.GetVersion()

func (cfg *AgentConfig) StartWithCerts(ctx context.Context, ca, crt, key []byte) error {
	clientCert, err := tls.X509KeyPair(crt, key)
	if err != nil {
		return err
	}

	// create tls config
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(ca)
	if !ok {
		return errors.New("invalid ca certificate")
	}

	cfg.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		ServerName:   CertsDNSName,
	}

	// create transport and http client
	cfg.httpClient = cfg.NewHTTPClient()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// get agent name
	name, _, err := cfg.Fetch(ctx, EndpointName)
	if err != nil {
		return err
	}

	cfg.Name = string(name)

	cfg.l = log.With().Str("agent", cfg.Name).Logger()

	// check agent version
	agentVersionBytes, _, err := cfg.Fetch(ctx, EndpointVersion)
	if err != nil {
		return err
	}

	// check agent runtime
	runtimeBytes, status, err := cfg.Fetch(ctx, EndpointRuntime)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		switch string(runtimeBytes) {
		case "docker":
			cfg.Runtime = ContainerRuntimeDocker
		case "nerdctl":
			cfg.Runtime = ContainerRuntimeNerdctl
		case "podman":
			cfg.Runtime = ContainerRuntimePodman
		default:
			return fmt.Errorf("invalid agent runtime: %s", runtimeBytes)
		}
	case http.StatusNotFound:
		// backward compatibility, old agent does not have runtime endpoint
		cfg.Runtime = ContainerRuntimeDocker
	default:
		return fmt.Errorf("failed to get agent runtime: HTTP %d %s", status, runtimeBytes)
	}

	cfg.Version = string(agentVersionBytes)
	agentVersion := pkg.ParseVersion(cfg.Version)

	if serverVersion.IsNewerMajorThan(agentVersion) {
		log.Warn().Msgf("agent %s major version mismatch: server: %s, agent: %s", cfg.Name, serverVersion, agentVersion)
	}

	log.Info().Msgf("agent %q initialized", cfg.Name)
	return nil
}

func (cfg *AgentConfig) Start(ctx context.Context) error {
	filepath, ok := certs.AgentCertsFilepath(cfg.Addr)
	if !ok {
		return fmt.Errorf("invalid agent host: %s", cfg.Addr)
	}

	certData, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read agent certs: %w", err)
	}

	ca, crt, key, err := certs.ExtractCert(certData)
	if err != nil {
		return fmt.Errorf("failed to extract agent certs: %w", err)
	}

	return cfg.StartWithCerts(ctx, ca, crt, key)
}

func (cfg *AgentConfig) NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: cfg.Transport(),
	}
}

func (cfg *AgentConfig) Transport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr != AgentHost+":443" {
				return nil, &net.AddrError{Err: "invalid address", Addr: addr}
			}
			if network != "tcp" {
				return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil}
			}
			return cfg.DialContext(ctx)
		},
		TLSClientConfig: cfg.tlsConfig,
	}
}

var dialer = &net.Dialer{Timeout: 5 * time.Second}

func (cfg *AgentConfig) DialContext(ctx context.Context) (net.Conn, error) {
	return dialer.DialContext(ctx, "tcp", cfg.Addr)
}

func (cfg *AgentConfig) GrpcCredentials() credentials.TransportCredentials {
	return credentials.NewTLS(cfg.tlsConfig)
}

func (cfg *AgentConfig) String() string {
	return cfg.Name + "@" + cfg.Addr
}
