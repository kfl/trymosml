package main

import (
	"flag"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":8080", "http service address")
var rootdir = flag.String("rootdir", "./web", "static root dir")


func loggingHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("STATIC", r.Method, r.URL.Path, "|", r.RemoteAddr, r.UserAgent())
		h.ServeHTTP(w, r)
	})
}


func main() {
	flag.Parse()
	cmd := flag.Arg(0)
	cmdArgs := flag.Args()[1:]
	reg := newRegistry()
	go reg.run()

	log.Printf("Starting http server at: %s", *addr)
	
    // Normal resources
    http.Handle("/", loggingHandler(http.FileServer(http.Dir(*rootdir))))

	
	http.HandleFunc("/webshell", func(w http.ResponseWriter, r *http.Request) {
		log.Println("DYNAMIC Starting webshell", "|", r.RemoteAddr, r.UserAgent())
		serveClient(reg, cmd, cmdArgs, w, r)
	})

	log.Fatalf("ListenAndServe: %v", http.ListenAndServe(*addr, nil))
	
}
