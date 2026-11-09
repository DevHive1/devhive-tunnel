// Command devhive-hub runs the devhive-tunnel coordination server:
// control protocol (mesh signaling) + GM/admin web dashboard.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"devhive-tunnel/pkg/conf"
	"devhive-tunnel/pkg/hub"
)

func main() {
	dir := flag.String("dir", filepath.Join(home(), ".devhive-hub"), "state directory")
	listen := flag.String("listen", ":7676", "control TCP listen address")
	web := flag.String("web", ":7600", "dashboard HTTP listen address")
	subnet := flag.String("subnet", "100.72.0.0/16", "default virtual subnet for new networks")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file for dashboard (optional)")
	tlsKey := flag.String("tls-key", "", "TLS key file for dashboard (optional)")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println(conf.Version)
		return
	}

	srv, err := hub.OpenServer(hub.ServerOptions{
		Dir: *dir, Listen: *listen, Web: *web, Subnet: *subnet,
	})
	if err != nil {
		log.Fatalf("devhive-hub: %v", err)
	}

	go func() {
		if err := srv.Serve(); err != nil {
			log.Fatalf("control server: %v", err)
		}
	}()

	httpSrv := &http.Server{Addr: *web, Handler: hub.Handler(srv)}
	go func() {
		var err error
		if *tlsCert != "" && *tlsKey != "" {
			log.Printf("[hub] dashboard: https://%s", *web)
			err = httpSrv.ListenAndServeTLS(*tlsCert, *tlsKey)
		} else {
			log.Printf("[hub] dashboard: http://%s", *web)
			err = httpSrv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("dashboard: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Printf("[hub] shutting down")
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}
