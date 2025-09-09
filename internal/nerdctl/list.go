package nerdctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/yusing/go-proxy/agent/pkg/agent"
	"github.com/yusing/go-proxy/internal/gperr"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
)

func WithDefaultNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, "default")
}

func ListContainers(ctx context.Context, host string) ([]container.Summary, error) {
	if agent.IsDockerHostAgent(host) {
		agentCfg, ok := agent.GetAgent(host)
		if !ok {
			return nil, fmt.Errorf("agent %q not found", host)
		}

		data, status, err := agentCfg.Fetch(ctx, agent.EndpointListContainers)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers: %w", err)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("failed to list containers: http %d - %s", status, data)
		}
		var containers []container.Summary
		err = json.Unmarshal(data, &containers)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal containers: %w", err)
		}
		return containers, nil
	}

	ctx = WithDefaultNamespace(ctx)
	c, err := NewNerdctlClient(ctx, host)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	containers, err := c.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	errs := gperr.NewBuilder()
	res := make([]container.Summary, 0, len(containers))
	for _, c := range containers {
		container, err := FromNerdctl(ctx, c, host)
		if err != nil {
			errs.Add(err)
			continue
		}
		res = append(res, container)
	}
	return res, errs.Error()
}

func FromNerdctl(ctx context.Context, c containerd.Container, host string) (container.Summary, error) {
	image, err := c.Image(ctx)
	if err != nil {
		return container.Summary{}, fmt.Errorf("failed to get container image: %w", err)
	}
	labels, err := c.Labels(ctx)
	if err != nil {
		return container.Summary{}, fmt.Errorf("failed to get container labels: %w", err)
	}

	cont := container.Summary{
		ID:     c.ID(),
		Image:  image.Name(),
		Names:  []string{labels[LabelName]},
		Labels: labels,
		Status: labels[LabelRestartStatus],
		State:  labels[LabelRestartStatus],
	}

	// get mounts
	{
		mountJSON := labels[LabelMounts]
		if mountJSON != "" {
			err := json.Unmarshal([]byte(mountJSON), &cont.Mounts)
			if err != nil {
				return container.Summary{}, fmt.Errorf("failed to get container mounts: %w", err)
			}
		}
	}

	// get networks
	{
		networkJSON := labels[LabelNetworks]
		if networkJSON != "" {
			var networks []string
			err := json.Unmarshal([]byte(networkJSON), &networks)
			if err != nil {
				return container.Summary{}, fmt.Errorf("failed to get container networks: %w", err)
			}
			if len(networks) > 0 && networks[0] == "host" {
				cont.HostConfig.NetworkMode = "host"
			}
		}
	}

	stateDir := labels[LabelStateDir]
	// get ports
	if stateDir != "" {
		networkConfigFile := filepath.Join(stateDir, NetworkConfigFile)
		ports, err := getPorts(networkConfigFile)
		if err != nil {
			return container.Summary{}, fmt.Errorf("failed to get container ports: %w", err)
		}
		cont.Ports = ports
	}

	return cont, nil
}

func getPorts(networkConfigFile string) ([]container.Port, error) {
	f, err := os.Open(networkConfigFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config NetworkConfig
	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return nil, err
	}

	ports := make([]container.Port, len(config.PortMappings))
	for i, port := range config.PortMappings {
		ports[i] = container.Port{
			PrivatePort: uint16(port.ContainerPort),
			PublicPort:  uint16(port.HostPort),
			Type:        port.Protocol,
			IP:          port.HostIP,
		}
	}
	return ports, nil
}
