package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/keroserene/go-webrtc"
	"time"
)

type WebRTCClient struct {
	pc     *webrtc.PeerConnection
	tx, rx *webrtc.DataChannel
}

type ConductorInterface interface {
	SendSignal(sdp string)
	ProcessCommand(interface{})
}

func NewWebRTCClient(conductor ConductorInterface, device DynastatInterface) (client *WebRTCClient, err error) {
	config := webrtc.NewConfiguration(
		webrtc.OptionIceServer("stun:stun.l.google.com:19302"),
	)
	client.pc, err = webrtc.NewPeerConnection(config)
	if err != nil {
		return
	}

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
			break

		default:
			panic(errors.New(fmt.Sprintf("Unkown data channel %s", label)))
		}
	}

	client.rx.OnMessage = func(msg []byte) {
		var cmd interface{}
		err := json.Unmarshal(msg, cmd)
		if err != nil {
			client.rx.Send([]byte("Error: invalid json"))
		}

		//f, err := cmd["cmd"]
		//if err != nil {
		//	client.rx.Send([]byte("Error: missing cmd key"))
		//}
		//
		//switch f {
		//case "set_motor":
		//	name, err := cmd["name"]
		//	if err != nil {
		//		client.rx.Send([]byte("Error: missing name"))
		//	}
		//	val, err := cmd["value"]
		//	if err != nil {
		//		client.rx.Send([]byte("Error: missing value"))
		//	}
		//
		//	device.SetMotor(name, val)
		//	break
		//
		//default:
		//	client.rx.Send([]byte("Error: unkown command"))
		//}
	}

	return
}

func (client *WebRTCClient) TransmitState(device DynastatInterface) {
	for {
		state, err := device.GetState().MarshalMsg(nil)
		if err != nil {
			panic(err)
		}

		client.tx.Send(state)

		time.Sleep(time.Second / FRAMERATE)
	}
}
