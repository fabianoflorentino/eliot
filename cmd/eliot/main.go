package main

import (
	"log"
	"os"

	apphttp "github.com/fabianoflorentino/eliot/internal/http"
)

func main() {
	app, err := apphttp.NewApp()

	if err != nil {
		log.Fatal(err)
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("listening on %s", addr)

	if err := app.Server.ListenAndServe(addr); err != nil {
		log.Fatal(err)
	}
}
