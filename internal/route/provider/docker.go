package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yusing/go-proxy/agent/pkg/agent"
	"github.com/yusing/go-proxy/internal/docker"
	"github.com/yusing/go-proxy/internal/gperr"
	"github.com/yusing/go-proxy/internal/nerdctl"
	"github.com/yusing/go-proxy/internal/route"
	"github.com/yusing/go-proxy/internal/serialization"
	"github.com/yusing/go-proxy/internal/types"
	"github.com/yusing/go-proxy/internal/utils/strutils"
	"github.com/yusing/go-proxy/internal/watcher"
)

type DockerProvider struct {
	name, dockerHost string
	runtime          agent.ContainerRuntime
	l                zerolog.Logger
}

const (
	aliasRefPrefix    = '#'
	aliasRefPrefixAlt = '$'
)

var ErrAliasRefIndexOutOfRange = gperr.New("index out of range")

func DockerProviderImpl(name, dockerHost string, runtime agent.ContainerRuntime) ProviderImpl {
	return &DockerProvider{
		name,
		dockerHost,
		runtime,
		log.With().Str("type", "docker").Str("name", name).Str("runtime", string(runtime)).Logger(),
	}
}

func (p *DockerProvider) String() string {
	return "docker@" + p.name
}

func (p *DockerProvider) ShortName() string {
	return p.name
}

func (p *DockerProvider) IsExplicitOnly() bool {
	return p.name[len(p.name)-1] == '!'
}

func (p *DockerProvider) Logger() *zerolog.Logger {
	return &p.l
}

func (p *DockerProvider) NewWatcher() watcher.Watcher {
	if p.runtime == agent.ContainerRuntimeNerdctl {
		return watcher.NewNerdctlWatcher(p.dockerHost)
	}
	return watcher.NewDockerWatcher(p.dockerHost)
}

func (p *DockerProvider) loadRoutesImpl() (route.Routes, gperr.Error) {
	var (
		containers []container.Summary
		err        error
	)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if p.runtime == agent.ContainerRuntimeNerdctl {
		containers, err = nerdctl.ListContainers(ctx, p.dockerHost)
	} else {
		containers, err = docker.ListContainers(ctx, p.dockerHost)
	}
	if err != nil {
		return nil, gperr.Wrap(err)
	}

	errs := gperr.NewBuilder("")
	routes := make(route.Routes)

	for _, c := range containers {
		container := docker.FromDocker(&c, p.dockerHost)

		if container.Errors != nil {
			errs.Add(container.Errors)
			continue
		}

		if container.IsHostNetworkMode {
			err := docker.UpdatePorts(container)
			if err != nil {
				errs.Add(gperr.PrependSubject(container.ContainerName, err))
				continue
			}
		}

		newEntries, err := p.routesFromContainerLabels(container)
		if err != nil {
			errs.Add(err.Subject(container.ContainerName))
		}
		for k, v := range newEntries {
			if conflict, ok := routes[k]; ok {
				err := gperr.Multiline().
					Addf("route with alias %s already exists", k).
					Addf("container %s", container.ContainerName).
					Addf("conflicting container %s", conflict.Container.ContainerName)
				if conflict.ShouldExclude() || v.ShouldExclude() {
					gperr.LogWarn("skipping conflicting route", err)
				} else {
					errs.Add(err)
				}
			} else {
				routes[k] = v
			}
		}
	}

	return routes, errs.Error()
}

// Returns a list of proxy entries for a container.
// Always non-nil.
func (p *DockerProvider) routesFromContainerLabels(container *types.Container) (route.Routes, gperr.Error) {
	if !container.IsExplicit && p.IsExplicitOnly() {
		return make(route.Routes, 0), nil
	}

	routes := make(route.Routes, len(container.Aliases))

	// init entries map for all aliases
	for _, a := range container.Aliases {
		routes[a] = &route.Route{
			Metadata: route.Metadata{
				Container: container,
			},
		}
	}

	errs := gperr.NewBuilder("label errors")

	m, err := docker.ParseLabels(container.Labels, container.Aliases...)
	errs.Add(err)

	for alias, entryMapAny := range m {
		if len(alias) == 0 {
			errs.Add(gperr.New("empty alias"))
			continue
		}

		entryMap, ok := entryMapAny.(types.LabelMap)
		if !ok {
			// try to deserialize to map
			entryMap = make(types.LabelMap)
			yamlStr, ok := entryMapAny.(string)
			if !ok {
				// should not happen
				panic(fmt.Errorf("invalid entry map type %T", entryMapAny))
			}
			if err := yaml.Unmarshal([]byte(yamlStr), &entryMap); err != nil {
				errs.Add(gperr.Wrap(err).Subject(alias))
				continue
			}
		}

		// check if it is an alias reference
		switch alias[0] {
		case aliasRefPrefix, aliasRefPrefixAlt:
			index, err := strutils.Atoi(alias[1:])
			if err != nil {
				errs.Add(err)
				break
			}
			if index < 1 || index > len(container.Aliases) {
				errs.Add(ErrAliasRefIndexOutOfRange.Subject(strconv.Itoa(index)))
				break
			}
			alias = container.Aliases[index-1]
		}

		// init entry if not exist
		r, ok := routes[alias]
		if !ok {
			r = &route.Route{
				Metadata: route.Metadata{
					Container: container,
				},
			}
			routes[alias] = r
		}

		// deserialize map into entry object
		err := serialization.MapUnmarshalValidate(entryMap, r)
		if err != nil {
			errs.Add(err.Subject(alias))
		} else {
			routes[alias] = r
		}
	}

	return routes, errs.Error()
}
