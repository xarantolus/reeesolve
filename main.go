package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"reeesolve/config"
	"reeesolve/ping"
	"reeesolve/redirect"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/duckdns"
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

	// Set up certmagic with DuckDNS token
	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = settings.EMail
	certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
		DNSProvider: &duckdns.Provider{
			APIToken: settings.DuckDNS.Token,
		},
	}

	// Set ports where we listen, 0 chooses a random port
	certmagic.HTTPPort = 0
	certmagic.HTTPSPort = settings.Port

	log.Println("Contacting DuckDNS to update our IP adress...")
	err = ping.DuckDNS(settings.DuckDNS.Domain, settings.DuckDNS.Token)
	if err != nil {
		log.Println("Error while contacting: ", err.Error())
	}

	var r = redirect.NewResolver(settings)

	mux := http.NewServeMux()
	mux.Handle("/resolve", r)

	domain := fmt.Sprintf("%s.duckdns.org", settings.DuckDNS.Domain)

	log.Printf("If you redirected port %d to this machine in your router, you can now access https://%s:%d/resolve\n", settings.Port, domain, settings.Port)

	err = certmagic.HTTPS([]string{domain}, mux)
	if err != nil {
		panic("setting up https server: " + err.Error())
	}
}
