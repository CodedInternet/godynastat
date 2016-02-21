package main

import (
	"fmt"
	"log"
	"net/http"
	"./signaling"
	"html/template"
)

var Table [10][16]int

func webrtcHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("html/webrtc.html"))
	tmpl.Execute(w, Table)
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "html/" + r.URL.Path)
}

func main() {
	port := "0.0.0.0:8000"
	wsHandler := signaling.Handler()
	http.Handle("/ws/", wsHandler)
	http.HandleFunc("/app", webrtcHandler)
	http.HandleFunc("/", staticHandler)
	fmt.Println("Listening on port", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
