package main

import (
	"encoding/json"
	"fmt"
	"github.com/CodedInternet/godynastat/onboard"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func EchoHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func WebRTCSignalHandler(w http.ResponseWriter, r *http.Request) {
	var client *onboard.WebRTCClient
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	msgs := make(chan string, 1)

	go func(conn *websocket.Conn, msgs chan string) {
		for {
			msg := <-msgs
			conn.WriteMessage(websocket.TextMessage, []byte(msg))
		}
	}(conn, msgs)

	for {
		_, msgb, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		msg := string(msgb)

		// artificially add a delay for the simulated device so we can mock some network transactions going on
		if ENV.Simulated {
			time.Sleep(time.Second / 2)
		}

		var parsed map[string]interface{}
		err = json.Unmarshal(msgb, &parsed)
		if nil != err {
			fmt.Errorf("%s\n", err)
		}

		if _, ok := parsed["type"]; ok {
			client, err = ENV.Conductor.ReceiveOffer(msg, msgs)
			if err != nil {
				fmt.Errorf("%s\n", err)
				return
			}
		} else if _, ok := parsed["candidate"]; ok {
			if client != nil {
				client.AddIceCandidate(msg)
			} else {
				fmt.Errorf("Received candidate before client is established")
			}
		} else {
			fmt.Errorf("Received unkown candidate")
		}
	}
}
