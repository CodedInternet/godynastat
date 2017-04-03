package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/keroserene/go-webrtc"
	"log"
	"time"
)

type WebRTCClient struct {
	pc        *webrtc.PeerConnection
	tx, rx    *webrtc.DataChannel
	conductor ConductorInterface
}

type Cmd struct {
	Cmd   string
	Name  string
	Value int
}

type Conductor struct {
	device           DynastatInterface
	clients          []*WebRTCClient
	signalingServers []*websocket.Conn
}

type ConductorInterface interface {
	SendSignal(sdp string)
	ProcessCommand(cmd Cmd)
}

func NewWebRTCClient(
	sdp *webrtc.SessionDescription,
	conductor ConductorInterface) (client *WebRTCClient, err error) {

	client = new(WebRTCClient)

	config := webrtc.NewConfiguration(
		webrtc.OptionIceServer("stun:stun.l.google.com:19302"),
	)
	client.pc, err = webrtc.NewPeerConnection(config)
	if err != nil {
		return
	}

	// Assign functions
	client.pc.OnIceComplete = func() {
		conductor.SendSignal(client.pc.LocalDescription().Serialize())
	}

	client.pc.OnDataChannel = func(channel *webrtc.DataChannel) {
		label := channel.Label()
		switch label {
		case "data":
			client.tx = channel
			break

		case "command":
			client.rx = channel
			client.rx.OnMessage = client.receiveMessage
			break

		default:
			panic(errors.New(fmt.Sprintf("Unkown data channel %s", label)))
		}
	}

	client.conductor = conductor

	// establish answer and setup the connection
	err = client.pc.SetRemoteDescription(sdp)
	if err != nil {
		panic(err)
	}
	answer, err := client.pc.CreateAnswer()
	if err != nil {
		panic(err)
	}
	client.pc.SetLocalDescription(answer)

	return
}

func (client *WebRTCClient) receiveMessage(msg []byte) {
	var cmd Cmd
	err := json.Unmarshal(msg, &cmd)
	if err != nil {
		client.rx.Send([]byte("Error: invalid json"))
	}

	client.conductor.ProcessCommand(cmd)
}

func (c *Conductor) SendSignal(msg string) {
	for _, server := range c.signalingServers {
		server.WriteMessage(websocket.TextMessage, []byte(msg))
	}
}

func (c *Conductor) ProcessCommand(cmd Cmd) {
	switch cmd.Cmd {
	case "set_motor":
		c.device.SetMotor(cmd.Name, cmd.Value)
		break

	default:
		fmt.Printf("Unable to process command %v\n", cmd)
	}
}

func (c *Conductor) UpdateClients() {
	for {
		state := c.device.GetState()
		msg, err := state.MarshalMsg(nil)
		if err != nil {
			panic(err)
		}
		for _, client := range c.clients {
			if client.tx != nil && client.tx.ReadyState() == webrtc.DataStateOpen {
				client.tx.Send(msg)
			}
		}

		time.Sleep(time.Second / FRAMERATE)
	}
}

func (c *Conductor) ReceiveOffer(msg string) (client *WebRTCClient, err error) {
	sdp := webrtc.DeserializeSessionDescription(msg)
	switch sdp.Type {
	case "offer":
		client, err = NewWebRTCClient(sdp, ConductorInterface(c))
		c.clients = append(c.clients, client)
		return
	}
	return nil, errors.New("offer was not of type offer")
}

func (c *Conductor) AddSignalingServer(wsUrl string) (server *websocket.Conn, err error) {
	server, _, err = websocket.DefaultDialer.Dial(wsUrl, nil)
	c.signalingServers = append(c.signalingServers, server)

	go func() {
		for {
			_, message, err := server.ReadMessage()
			if err != nil {
				log.Println("error:", err)
			}
			log.Printf("recv: %s\n", message)
			_, err = c.ReceiveOffer(string(message))
			if err != nil {
				log.Println("error:", err)
			}
		}
	}()

	return
}