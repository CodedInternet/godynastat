package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/keroserene/go-webrtc"
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
	device  *DynastatInterface
	clients []WebRTCClient
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

		case "control":
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
