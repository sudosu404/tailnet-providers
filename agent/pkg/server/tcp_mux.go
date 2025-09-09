package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

// GrpcHttpMuxListener wraps a net.Listener and only returns HTTP connections from Accept().
// gRPC connections are handled internally by proxying to the containerd socket.
type GrpcHttpMuxListener struct {
	listener         net.Listener
	containerdSocket string
	ctx              context.Context
}

// NewFilteringListener creates a new filtering listener that proxies gRPC connections
// and only returns HTTP connections to the caller.
func NewGrpcHttpMuxListener(listener net.Listener, containerdSocket string, ctx context.Context) *GrpcHttpMuxListener {
	return &GrpcHttpMuxListener{
		listener:         listener,
		containerdSocket: containerdSocket,
		ctx:              ctx,
	}
}

// Accept accepts connections from the underlying listener, but only returns HTTP connections.
// gRPC connections are proxied to the containerd socket and the method continues to accept
// new connections until an HTTP connection is received.
func (fl *GrpcHttpMuxListener) Accept() (net.Conn, error) {
	for {
		conn, err := fl.listener.Accept()
		if err != nil {
			return nil, err
		}

		// Check if this is a gRPC connection
		if fl.isGRPCConnection(conn) {
			// Handle gRPC connection in background and continue accepting
			go fl.handleGRPCConnection(conn)
			continue
		}

		// Return HTTP connection
		return conn, nil
	}
}

// Close closes the underlying listener.
func (fl *GrpcHttpMuxListener) Close() error {
	return fl.listener.Close()
}

// Addr returns the address of the underlying listener.
func (fl *GrpcHttpMuxListener) Addr() net.Addr {
	return fl.listener.Addr()
}

// isGRPCConnection checks if the connection is using gRPC by examining TLS ALPN.
func (fl *GrpcHttpMuxListener) isGRPCConnection(conn net.Conn) bool {
	if tlsConn, ok := conn.(*tls.Conn); ok {
		// Perform TLS handshake to get negotiated protocol
		err := tlsConn.HandshakeContext(fl.ctx)
		if err != nil {
			log.Debug().Err(err).Msg("TLS handshake failed during protocol detection")
			return false
		}

		state := tlsConn.ConnectionState()
		return state.NegotiatedProtocol == "h2"
	}
	return false
}

// handleGRPCConnection forwards gRPC traffic directly to containerd socket.
func (fl *GrpcHttpMuxListener) handleGRPCConnection(clientConn net.Conn) {
	defer clientConn.Close()

	dialer := &net.Dialer{KeepAlive: 1 * time.Second}
	// Connect to containerd socket
	containerdConn, err := dialer.DialContext(fl.ctx, "unix", fl.containerdSocket)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to containerd socket")
		return
	}
	defer containerdConn.Close()

	// Bidirectional copy
	done := make(chan error, 2)

	go func() {
		_, err := io.Copy(containerdConn, clientConn)
		done <- err
	}()

	go func() {
		_, err := io.Copy(clientConn, containerdConn)
		done <- err
	}()

	// Wait for one direction to complete
	<-done
}
