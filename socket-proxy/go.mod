module github.com/yusing/go-proxy/socketproxy

go 1.25.1

replace github.com/yusing/go-proxy/internal/utils => ../internal/utils

require (
	github.com/gorilla/mux v1.8.1
	github.com/yusing/go-proxy/internal/utils v0.0.0-20250903143810-e1133a2daf72
	golang.org/x/net v0.43.0
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)
