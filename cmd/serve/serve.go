package main

import (
	"flag"
	"log"
	"net/http"

	"pkg.jsn.cam/jsn/internal"
)

var (
	port    = flag.String("port", "3000", "port to use")
	dir     = flag.String("dir", ".", "directory to serve")
	verbose = flag.Bool("v", false, "enable verbose logging")
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	internal.HandleStartup()

	var handler = http.FileServer(http.Dir(*dir))
	if *verbose {
		handler = loggingMiddleware(handler)
	}

	http.Handle("/", handler)

	log.Printf("Serving %s on port %s", *dir, *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
