package watcher

import (
	"context"
	"errors"
	"time"

	eventstypes "github.com/containerd/containerd/api/events"
	containerd "github.com/containerd/containerd/v2/client"
	containerdEvents "github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/typeurl/v2"
	"github.com/rs/zerolog/log"
	"github.com/yusing/go-proxy/internal/gperr"
	"github.com/yusing/go-proxy/internal/nerdctl"
	"github.com/yusing/go-proxy/internal/watcher/events"
)

type NerdctlWatcher string

func NewNerdctlWatcher(host string) NerdctlWatcher {
	return NerdctlWatcher(host)
}

// TODO: refactor this, almost identical to docker_watcher.go

func (w NerdctlWatcher) Events(ctx context.Context) (<-chan Event, <-chan gperr.Error) {
	eventCh := make(chan Event)
	errCh := make(chan gperr.Error)

	go func() {
		client, err := nerdctl.NewNerdctlClient(ctx, string(w))
		if err != nil {
			errCh <- gperr.Wrap(err, "nerdctl watcher: failed to initialize client")
			return
		}

		defer func() {
			close(eventCh)
			close(errCh)
			client.Close()
		}()

		cEventCh, cErrCh := client.EventService().Subscribe(ctx)
		defer log.Debug().Str("host", string(w)).Msg("nerdctl watcher closed")
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-cEventCh:
				w.handleEvent(ctx, client, msg, eventCh, errCh)
			case err := <-cErrCh:
				if err == nil {
					continue
				}
				errCh <- w.parseError(err)
				// release the error because reopening event channel may block
				//nolint:ineffassign,wastedassign
				err = nil
				// trigger reload (clear routes)
				eventCh <- reloadTrigger

				retry := time.NewTicker(dockerWatcherRetryInterval)
				defer retry.Stop()
				ok := false
			outer:
				for !ok {
					select {
					case <-ctx.Done():
						return
					case <-retry.C:
						if checkNerdctlConnection(ctx, client) {
							ok = true
							break outer
						}
					}
				}
				// connection successful, trigger reload (reload routes)
				eventCh <- reloadTrigger
				// reopen event channel
				cEventCh, cErrCh = client.EventService().Subscribe(ctx)
			}
		}
	}()

	return eventCh, errCh
}

func (w NerdctlWatcher) handleEvent(ctx context.Context, client *containerd.Client, msg *containerdEvents.Envelope, ch chan<- Event, errCh chan<- gperr.Error) {
	// ref: https://github.com/containerd/nerdctl/blob/main/pkg/cmd/container/stats.go
	switch msg.Topic {
	case "/containers/create", "/containers/delete":
		anydata, err := typeurl.UnmarshalAny(msg.Event)
		if err != nil {
			errCh <- gperr.Wrap(err)
			return
		}
		switch data := anydata.(type) {
		case *eventstypes.ContainerCreate:
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			ctx = nerdctl.WithDefaultNamespace(ctx)
			container, err := client.LoadContainer(ctx, data.ID)
			if err != nil {
				errCh <- gperr.Wrap(err)
				return
			}

			labels, err := container.Labels(ctx)
			if err != nil {
				errCh <- gperr.Wrap(err)
				return
			}

			ch <- Event{
				Type:            events.EventTypeDocker,
				Action:          events.ActionContainerCreate,
				ActorID:         data.ID,
				ActorAttributes: labels,
				ActorName:       labels[nerdctl.LabelName],
			}
		case *eventstypes.ContainerDelete:
			ch <- Event{
				Type:            events.EventTypeDocker,
				Action:          events.ActionContainerDestroy,
				ActorID:         data.ID,
				ActorAttributes: map[string]string{},
				ActorName:       "",
			}
		}
	}
}

func (w NerdctlWatcher) parseError(err error) gperr.Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return gperr.New("nerdctl watcher: connection timeout")
	}
	return gperr.Wrap(err)
}

func checkNerdctlConnection(ctx context.Context, client *containerd.Client) bool {
	version, err := client.Version(ctx)
	if err != nil {
		return false
	}
	return version.Version != ""
}
