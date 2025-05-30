// Command quickserv serves a folder of files over HTTP quickly.
package main

import (
	"flag"
	"log"
	"net/http"

	"pkg.jsn.cam/jsn/internal"
)

var (
	port = flag.String("port", "3000", "port to use")
	dir  = flag.String("dir", ".", "directory to serve")
)

func main() {
	internal.HandleStartup()
	http.Handle("/", http.FileServer(http.Dir(*dir)))
	log.Printf("Serving %s on port %s", *dir, *port)
	http.ListenAndServe(":"+*port, nil)
}
