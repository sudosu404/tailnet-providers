package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/sudosu404/providers/agent/pkg/env"
	"github.com/sudosu404/providers/agent/pkg/handler"
	"github.com/sudosu404/tailnet-utils/server"
	"github.com/sudosu404/tailnet-utils/task"
)

type Options struct {
	CACert, ServerCert *tls.Certificate
	Port               int
}

func StartAgentServer(parent task.Parent, opt Options) {
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(opt.CACert.Leaf)

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*opt.ServerCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	if env.AgentSkipClientCertCheck {
		tlsConfig.ClientAuth = tls.NoClientCert
	}

	agentServer := &http.Server{
		Addr:      fmt.Sprintf(":%d", opt.Port),
		Handler:   handler.NewAgentHandler(),
		TLSConfig: tlsConfig,
	}

	server.Start(parent.Subtask("agent-server", false), agentServer, server.WithLogger(&log.Logger))
}
