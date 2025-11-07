package watcher

import (
	"context"

	"github.com/sudosu404/providers/internal/watcher/events"
	gperr "github.com/sudosu404/go-utils/errs"
)

type Event = events.Event

type Watcher interface {
	Events(ctx context.Context) (<-chan Event, <-chan gperr.Error)
}
