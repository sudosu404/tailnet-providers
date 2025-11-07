package autocert

import (
	"crypto/tls"

	"github.com/sudosu404/tailnet-utils/task"
)

type Provider interface {
	Setup() error
	GetCert(*tls.ClientHelloInfo) (*tls.Certificate, error)
	ScheduleRenewal(task.Parent)
	ObtainCert() error
}
