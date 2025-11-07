package idlewatcher

import (
	"net/http"

	nettypes "github.com/sudosu404/providers/internal/net/types"
	"github.com/sudosu404/providers/internal/types"
)

type Waker interface {
	types.HealthMonitor
	http.Handler
	nettypes.Stream
	Wake() error
}
