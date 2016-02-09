package main

import (
	"fmt"
	"log"
	"net/http"
	"signaling"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "html/" + r.URL.Path[1:])
}

func main() {
	port := "0.0.0.0:8000"
	wsHandler := signaling.Handler()
	http.Handle("/ws/", wsHandler)
	http.HandleFunc("/", indexHandler)
	fmt.Println("Listening on port", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
