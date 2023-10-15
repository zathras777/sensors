package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
)

type JsonEndpoint struct {
	Endpoint string
	Handler  func() map[string]interface{}
}

var endpoints []JsonEndpoint
var httpServer *http.Server

func AddEndpoint(endp JsonEndpoint) {
	endpoints = append(endpoints, endp)
}

func logAvailableEndpoints() {
	var avail []string
	for _, e := range endpoints {
		avail = append(avail, e.Endpoint)
	}
	sort.Strings(avail)
	log.Printf("available endpoints: %s", strings.Join(avail, ", "))
}

func startHttpServer(host string, port int) error {
	log.Printf("Starting HTTP server listening @ http://%s:%d/", host, port)
	logAvailableEndpoints()
	mux := http.NewServeMux()
	mux.HandleFunc("/", jsonResponse)

	httpServer = &http.Server{Addr: fmt.Sprintf("%s:%d", host, port), Handler: mux}
	err := httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		log.Println("HTTP server shutdown")
	} else if err != nil {
		log.Printf("server error: %v\n", err)
	}
	return err
}

var unknownURLs map[string]int = make(map[string]int)

func jsonResponse(w http.ResponseWriter, r *http.Request) {
	var dataMap map[string]interface{}
	var found bool

	for _, poss := range endpoints {
		if poss.Endpoint == r.RequestURI {
			dataMap = poss.Handler()
			found = true
			break
		}
	}

	if !found {
		_, ck := unknownURLs[r.RequestURI]
		if !ck {
			log.Printf("Unknown url '%s' requested", r.RequestURI)
			unknownURLs[r.RequestURI] = 1
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "404 - Not found")
		return
	}
	outData, err := json.Marshal(dataMap)
	w.Header().Set("Content-Type", "application/json")
	if err == nil {
		w.Write(outData)
		return
	}
	log.Printf("jsonResponse: Unable to generate json data: %s", err)
}
