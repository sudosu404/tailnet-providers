package main

import (
	"log"
	"net/http"

	socketproxy "github.com/sudosu404/providers/socketproxy/pkg"
)

func main() {
	if socketproxy.ListenAddr == "" {
		log.Fatal("Docker socket address is not set")
	}
	log.Printf("Docker socket listening on: %s", socketproxy.ListenAddr)
	http.ListenAndServe(socketproxy.ListenAddr, socketproxy.NewHandler())
}
