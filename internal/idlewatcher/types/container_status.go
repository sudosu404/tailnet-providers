package idlewatcher

import gperr "github.com/sudosu404/tailnet-utils/errs"

type ContainerStatus string

const (
	ContainerStatusError   ContainerStatus = "error"
	ContainerStatusRunning ContainerStatus = "running"
	ContainerStatusPaused  ContainerStatus = "paused"
	ContainerStatusStopped ContainerStatus = "stopped"
)

var ErrUnexpectedContainerStatus = gperr.New("unexpected container status")
