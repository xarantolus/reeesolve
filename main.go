package main

import (
	"flag"
	"log"
	"net/http"
	"reeesolve/config"
	"reeesolve/redirect"
	"strconv"
)

func main() {
	var (
		flagConfigFile = flag.String("cfg", "config.yaml", "Path to config file")
	)
	flag.Parse()

	settings, err := config.Parse(*flagConfigFile)
	if err != nil {
		log.Fatalln("Loading settings file:", err.Error())
	}

	var r = redirect.NewResolver(settings)

	http.Handle("/resolve", r)

	log.Printf("Server listening on port %d\n", settings.Port)
	http.ListenAndServe(":"+strconv.Itoa(settings.Port), nil)
}
