package main

import (
	"os"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/sudosu404/providers/internal/api"
	"github.com/sudosu404/providers/internal/auth"
	"github.com/sudosu404/providers/internal/common"
	"github.com/sudosu404/providers/internal/config"
	"github.com/sudosu404/providers/internal/dnsproviders"
	"github.com/sudosu404/providers/internal/homepage"
	"github.com/sudosu404/providers/internal/logging"
	"github.com/sudosu404/providers/internal/logging/memlogger"
	"github.com/sudosu404/providers/internal/metrics/systeminfo"
	"github.com/sudosu404/providers/internal/metrics/uptime"
	"github.com/sudosu404/providers/internal/net/gphttp/middleware"
	gperr "github.com/sudosu404/go-utils/errs"
	"github.com/sudosu404/go-utils/server"
	"github.com/sudosu404/go-utils/task"
	"github.com/sudosu404/go-utils/version"
)

func parallel(fns ...func()) {
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Go(fn)
	}
	wg.Wait()
}

func main() {
	initProfiling()

	logging.InitLogger(os.Stderr, memlogger.GetMemLogger())
	log.Info().Msgf("Tailnet version %s", version.Get())
	log.Trace().Msg("trace enabled")
	parallel(
		dnsproviders.InitProviders,
		homepage.InitIconListCache,
		systeminfo.Poller.Start,
		middleware.LoadComposeFiles,
	)

	if common.APIJWTSecret == nil {
		log.Warn().Msg("API_JWT_SECRET is not set, using random key")
		common.APIJWTSecret = common.RandomJWTKey()
	}

	for _, dir := range common.RequiredDirectories {
		prepareDirectory(dir)
	}

	err := config.Load()
	if err != nil {
		gperr.LogWarn("errors in config", err)
	}

	config.StartProxyServers()
	if err := auth.Initialize(); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize authentication")
	}
	// API Handler needs to start after auth is initialized.
	server.StartServer(task.RootTask("api_server", false), server.Options{
		Name:     "api",
		HTTPAddr: common.APIHTTPAddr,
		Handler:  api.NewHandler(),
	})

	uptime.Poller.Start()
	config.WatchChanges()

	task.WaitExit(config.Value().TimeoutShutdown)
}

func prepareDirectory(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal().Msgf("failed to create directory %s: %v", dir, err)
		}
	}
}
