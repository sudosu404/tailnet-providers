package homepage

import (
	"net/http"

	nettypes "github.com/sudosu404/providers/internal/net/types"
	"github.com/sudosu404/providers/internal/utils/pool"
)

type route interface {
	pool.Object
	ProviderName() string
	References() []string
	TargetURL() *nettypes.URL
}

type httpRoute interface {
	route
	http.Handler
}
