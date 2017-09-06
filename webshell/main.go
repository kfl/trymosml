package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"crypto/tls"
	"golang.org/x/crypto/acme/autocert"
)

//var addr = flag.String("addr", ":8080", "http service address")
//var rootdir = flag.String("rootdir", "./web", "static root dir")

func loggingHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("STATIC", r.Method, r.URL.Path, "|", r.RemoteAddr, r.UserAgent())
		h.ServeHTTP(w, r)
	})
}

type Configuration struct {
	Addr        string
	Rootdir     string
	Command     string
	CommandArgs []string
}

var conf = flag.String("conf", "conf.json", "JSON configuration file")

func readConfig(filename string) Configuration {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal("Config File Missing. ", err)
	}

	var config Configuration
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal("Config Parse Error: ", err)
	}

	return config
}

func main() {
	flag.Parse()

	config := readConfig(*conf)
	cmd := config.Command
	cmdArgs := config.CommandArgs
	reg := newRegistry()
	go reg.run()

	log.Printf("Starting http server at: %s", config.Addr)


    mux := http.NewServeMux()
	// Normal resources
	mux.Handle("/", loggingHandler(http.FileServer(http.Dir(config.Rootdir))))

	// websockets
	mux.HandleFunc("/webshell", func(w http.ResponseWriter, r *http.Request) {
		log.Println("DYNAMIC Starting webshell", "|", r.RemoteAddr, r.UserAgent())
		serveClient(reg, cmd, cmdArgs, w, r)
	})

	// TLS via Let's Encrypt
	m := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("try.mosml.org"),
	}
	s := &http.Server{
		Addr:      ":https",
		TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
		Handler: mux,
	}


	go log.Fatalf("ListenAndServeTLS: %v", s.ListenAndServeTLS("", ""))

	log.Fatalf("ListenAndServe: %v", http.ListenAndServe(config.Addr, mux))

}
