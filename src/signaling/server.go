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
	ReadBufferSize:        1024,
	WriteBufferSize:    1024,
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

func PubSubHandler(conn *websocket.Conn, pubsubClient *redis.PubSub) {
	for {
		msgi, err := pubsubClient.ReceiveMessage()

		if err != nil {
			return
		}

		switch msg := interface{} (msgi).(type) {
		case *redis.Message:
			var json_blob interface{}
			bytes_blob := []byte(msg.Payload)

			if err := json.Unmarshal(bytes_blob, &json_blob); err != nil {
				logger.Printf("[%s][error] failed to parse JSON %v, because %v", conn.RemoteAddr(), msg.Payload, err)
				continue
			}

			if err := conn.WriteJSON(json_blob); err != nil {
				logger.Printf("[%s][error] failed to send JSON, because %v", conn.RemoteAddr(), err)
				conn.Close()
				return
			}

			logger.Printf("[%s][send] OK", conn.RemoteAddr())
		default:
			logger.Printf("[%s][error] Unkown message: %s", conn.RemoteAddr(), msg)
			return
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
	logger.Printf("Subscribing to %s", name)

	pubsubClient := redisClient.PubSub()
	defer pubsubClient.Close()

	if err := pubsubClient.Subscribe(name); err != nil {
		logger.Printf("[%s][%s][error] Could not subscribe to topic %s, because %s", conn.RemoteAddr(), name, name, err)
		return
	}

	go PubSubHandler(conn, pubsubClient)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("error:", err)
			return
		}
		log.Printf("recv: %s", message)

		redisClient.Publish(name, string(message))
	}
}

func Handler() http.Handler {
	logger = log.New(os.Stdout, "[signaling] ", log.Ldate | log.Ltime | log.Lshortfile)

	redisClient = redis.NewClient(&redis.Options{
		Addr: "192.168.99.100:6379",
	})

	r := mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc("/ws/echo/", EchoHandler).Methods("GET")
	r.HandleFunc("/ws/device/{key}/", DeviceHandler)

	return r
}
