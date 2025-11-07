package autocert

import (
	"crypto/tls"

	"github.com/sudosu404/go-utils/task"
)

type Provider interface {
	Setup() error
	GetCert(*tls.ClientHelloInfo) (*tls.Certificate, error)
	ScheduleRenewal(task.Parent)
	ObtainCert() error
}
