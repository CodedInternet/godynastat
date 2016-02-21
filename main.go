package main

import (
	"fmt"
	"log"
	"net/http"
	"./signaling"
	"html/template"
	"github.com/gorilla/mux"
)

var Table [10][16]int

func webrtcHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("html/webrtc.html"))
	tmpl.Execute(w, Table)
}

func main() {
	port := "0.0.0.0:8000"

	r := mux.NewRouter()
	r.StrictSlash(true)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	r.PathPrefix("/ws/").Handler(signaling.Handler())
	r.HandleFunc("/app/", webrtcHandler)

	fmt.Println("Listening on port", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatal(err)
	}
}
