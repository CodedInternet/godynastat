package signaling

import (
	"github.com/gorilla/websocket"
	"net/http"
	"log"
	"github.com/gorilla/mux"
	"gopkg.in/redis.v3"
	"encoding/json"
	"os"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:		1024,
	WriteBufferSize:	1024,
	CheckOrigin:        func(r *http.Request) bool { return true },
}

var (
	redisClient *redis.Client
	logger *log.Logger
)

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

func PubSubHandler(conn *websocket.Conn, name string) {
	pubsubClient := redisClient.PubSub()
	defer pubsubClient.Close()

	if err := pubsubClient.Subscribe(name); err != nil {
		logger.Printf("[%s][%s][error] Could not subscribe to topic %s, because %s", conn.RemoteAddr(), name,  name, err)
		conn.Close()
	}

	for {
		i, _ := pubsubClient.Receive()

		if msg, _ := i.(*redis.Message); msg != nil {
			var json_blob interface{}
			bytes_blob := []byte(msg.Payload)

			if err := json.Unmarshal(bytes_blob, &json_blob); err != nil {
				logger.Printf("[%s][%s][error] failed to parse JSON %v, because %v", conn.RemoteAddr(), name, msg.Payload, err)
				continue
			}

			if err := conn.WriteJSON(json_blob); err != nil {
				logger.Printf("[%s][%s][error] failed to send JSON, because %v", conn.RemoteAddr(), name, err)
				conn.Close()
				break
			}

			logger.Printf("[%s][%s][send] OK", conn.RemoteAddr(), name)
		}
	}
}

func DeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["key"]
	if len(name) == 0 {
		http.Error(w, "A Device Key must be provided", 400)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade Error: ", err)
	}

	defer conn.Close()
	PubSubHandler(conn, name)
}

func Handler() http.Handler {
	logger = log.New(os.Stdout, "[signaling] ", log.Ldate|log.Ltime|log.Lshortfile)

	redisClient = redis.NewClient(&redis.Options{
		Addr: "192.168.99.100:6379",
	})

	r := mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc("/ws/echo/", EchoHandler).Methods("GET")
	r.HandleFunc("/ws/device/{key}/", DeviceHandler)

	return r
}
