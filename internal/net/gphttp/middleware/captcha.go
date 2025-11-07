package middleware

import (
	"net/http"

	"github.com/sudosu404/providers/internal/net/gphttp/middleware/captcha"
)

type hCaptcha struct {
	captcha.HcaptchaProvider
}

func (h *hCaptcha) before(w http.ResponseWriter, r *http.Request) (proceed bool) {
	return captcha.PreRequest(h, w, r)
}

var HCaptcha = NewMiddleware[hCaptcha]()
