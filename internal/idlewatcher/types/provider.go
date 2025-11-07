package idlewatcher

import (
	"context"

	"github.com/sudosu404/providers/internal/types"
	"github.com/sudosu404/providers/internal/watcher/events"
	gperr "github.com/sudosu404/go-utils/errs"
)

type Provider interface {
	ContainerPause(ctx context.Context) error
	ContainerUnpause(ctx context.Context) error
	ContainerStart(ctx context.Context) error
	ContainerStop(ctx context.Context, signal types.ContainerSignal, timeout int) error
	ContainerKill(ctx context.Context, signal types.ContainerSignal) error
	ContainerStatus(ctx context.Context) (ContainerStatus, error)
	Watch(ctx context.Context) (eventCh <-chan events.Event, errCh <-chan gperr.Error)
	Close()
}
