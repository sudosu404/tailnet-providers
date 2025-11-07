package config

import (
	"context"
	"errors"
	"iter"
	"net/http"

	"github.com/sudosu404/providers/internal/types"
	"github.com/sudosu404/tailnet-utils/server"
	"github.com/sudosu404/tailnet-utils/synk"
	"github.com/sudosu404/tailnet-utils/task"
)

type State interface {
	InitFromFile(filename string) error
	Init(data []byte) error

	Task() *task.Task
	Context() context.Context

	Value() *Config

	EntrypointHandler() http.Handler
	AutoCertProvider() server.CertProvider

	LoadOrStoreProvider(key string, value types.RouteProvider) (actual types.RouteProvider, loaded bool)
	DeleteProvider(key string)
	IterProviders() iter.Seq2[string, types.RouteProvider]
	NumProviders() int
	StartProviders() error

	FlushTmpLog()
}

// could be nil
var ActiveState synk.Value[State]

var ErrConfigChanged = errors.New("config changed")
