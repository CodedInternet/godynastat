package comms

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/keroserene/go-webrtc"
	//"io"
	//"time"
	"github.com/CodedInternet/godynastat/onboard"
)

type WebRTCClient struct {
	pc        *webrtc.PeerConnection
	tx, rx    *webrtc.DataChannel
	conductor ConductorInterface
}

type Cmd struct {
	Cmd   string
	Name  string
	Value float64
}

type Conductor struct {
	Device           *onboard.ActuatorDynastat
	clients          []*WebRTCClient
	signalingServers []*websocket.Conn
}

type ConductorInterface interface {
	ProcessCommand(cmd Cmd)
}

func NewWebRTCClient(
	sdp *webrtc.SessionDescription,
	conductor ConductorInterface,
	signals chan<- string) (client *WebRTCClient, err error) {

	client = new(WebRTCClient)

	config := webrtc.NewConfiguration(
		webrtc.OptionIceServer("stun:stun.stunprotocol.org"),
		webrtc.OptionIceServer("stun:stun.l.google.com:19302"),
		webrtc.OptionIceServer("stun:stun1.l.google.com:19302"),
		webrtc.OptionIceServer("stun:stun2.l.google.com:19302"),
		webrtc.OptionIceServer("stun:stun3.l.google.com:19302"),
		webrtc.OptionIceServer("stun:stun4.l.google.com:19302"),
	)
	client.pc, err = webrtc.NewPeerConnection(config)
	if err != nil {
		return
	}

	// Assign functions
	client.pc.OnIceCandidate = func(c webrtc.IceCandidate) {
		signals <- c.Serialize()
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
	go func() {
		answer, err := client.pc.CreateAnswer()
		if err != nil {
			panic(err)
		}
		client.pc.SetLocalDescription(answer)
		signals <- answer.Serialize()
		return
	}()

	return
}

func (client *WebRTCClient) AddIceCandidate(msg string) error {
	ic := webrtc.DeserializeIceCandidate(msg)
	if ic == nil {
		return errors.New("Unable to deserialize ice msg")
	}

	client.pc.AddIceCandidate(*ic)
	fmt.Printf("added ice candidate: %s\n", ic.Serialize())
	return nil
}

func (client *WebRTCClient) receiveMessage(msg []byte) {
	var cmd Cmd
	err := json.Unmarshal(msg, &cmd)
	if err != nil {
		client.rx.Send([]byte("Error: invalid json"))
	}

	client.conductor.ProcessCommand(cmd)
}

func (c *Conductor) ProcessCommand(cmd Cmd) {
	switch cmd.Cmd {
	case "set_height":
		c.Device.SetHeight(cmd.Name, cmd.Value)
		break

	case "set_frontal":
		c.Device.SetRotation(cmd.Name, cmd.Value, 0)
		break

		//case "persist_config":
		//	filename, _ := yamlFilename()
		//	yml, _ := yaml.Marshal(c.Device.GetConfig())
		//	ioutil.WriteFile(filename, yml, 0744)
		//	break

	default:
		fmt.Printf("Unable to process command %v\n", cmd)
	}
}

func (c *Conductor) UpdateClients() {
	//for {
	//state, err := c.Device.GetState()
	//if err != nil {
	//	switch err {
	//	case io.EOF:
	//		fmt.Println("Recieved EOF, continuing")
	//		continue
	//	default:
	//		panic(err)
	//	}
	//}
	//msg, err := json.Marshal(state)
	//if err != nil {
	//	panic(err)
	//}
	//for _, client := range c.clients {
	//	if client.tx != nil && client.tx.ReadyState() == webrtc.DataStateOpen {
	//		client.tx.SendText(string(msg))
	//	}
	//}
	//
	//time.Sleep(time.Second / onboard.FRAMERATE)
	//}
}

func (c *Conductor) ReceiveOffer(msg string, signals chan<- string) (client *WebRTCClient, err error) {
	sdp := webrtc.DeserializeSessionDescription(msg)
	if sdp != nil {
		switch sdp.Type {
		case "offer":
			client, err = NewWebRTCClient(sdp, ConductorInterface(c), signals)
			if err != nil {
				return nil, err
			}
			c.clients = append(c.clients, client)
			return client, nil
			break
		default:
			return nil, errors.New("Unkown SDP type")
		}
	}
	return nil, errors.New("SDP was not valid")
}

//func (c *Conductor) AddSignalingServer(wsUrl string) (server *websocket.Conn, err error) {
//	server, _, err = websocket.DefaultDialer.Dial(wsUrl, nil)
//	c.signalingServers = append(c.signalingServers, server)
//
//	go func() {
//		for {
//			_, message, err := server.ReadMessage()
//			if err != nil {
//				log.Println("error:", err)
//			}
//			log.Printf("recv: %s\n", message)
//			_, err = c.ReceiveOffer(string(message))
//			if err != nil {
//				log.Println("error:", err)
//			}
//		}
//	}()
//
//	return
//}
