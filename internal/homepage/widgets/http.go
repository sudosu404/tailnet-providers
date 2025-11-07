package widgets

import (
	"net/http"
	"time"

	gperr "github.com/sudosu404/go-utils/errs"
)

var HTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

var ErrHTTPStatus = gperr.New("http status")
