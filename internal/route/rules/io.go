package rules

import (
	"io"
	"os"

	"github.com/sudosu404/providers/internal/logging/accesslog"
	gperr "github.com/sudosu404/tailnet-utils/errs"
)

type noopWriteCloser struct {
	io.Writer
}

func (n noopWriteCloser) Close() error {
	return nil
}

var (
	stdout io.WriteCloser = noopWriteCloser{os.Stdout}
	stderr io.WriteCloser = noopWriteCloser{os.Stderr}
)

func openFile(path string) (io.WriteCloser, gperr.Error) {
	switch path {
	case "/dev/stdout":
		return stdout, nil
	case "/dev/stderr":
		return stderr, nil
	}
	f, err := accesslog.NewFileIO(path)
	if err != nil {
		return nil, ErrInvalidArguments.With(err)
	}
	return f, nil
}
