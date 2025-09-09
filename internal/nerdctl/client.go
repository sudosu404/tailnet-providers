package nerdctl

import (
	"context"
	"fmt"
	"net"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/yusing/go-proxy/agent/pkg/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewNerdctlClient(ctx context.Context, host string) (*containerd.Client, error) {
	if agent.IsDockerHostAgent(host) {
		cfg, ok := agent.GetAgent(host)
		if !ok {
			return nil, fmt.Errorf("agent %q not found", host)
		}
		conn, err := grpc.NewClient("passthrough:///agent",
			grpc.WithTransportCredentials(cfg.GrpcCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return cfg.DialContext(ctx)
			}))
		if err != nil {
			return nil, err
		}
		return containerd.NewWithConn(conn)
	}

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return containerd.NewWithConn(conn)
}
