package main

import (
	"fmt"
	"log"
	"net/http"
	"signaling"
)

func main() {
	port := "0.0.0.0:8000"
	wsHandler := signaling.Handler()
	http.Handle("/ws/", wsHandler)
	fmt.Println("Listening on port", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
