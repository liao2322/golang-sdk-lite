package main

import (
	"log"
	"net/http"

	"github.com/halalcloud/golang-sdk-lite/internal/webui"
)

func main() {
	cfg, err := webui.LoadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	app := webui.NewApp(cfg)
	log.Printf("web ui listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, app.Routes()); err != nil {
		log.Fatal(err)
	}
}
