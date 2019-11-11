package comms

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v2"
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
	Device           onboard.Dynastat
	clients          []*WebRTCClient
	signalingServers []*websocket.Conn
}

type ConductorInterface interface {
	ProcessCommand(cmd Cmd)
}

func NewWebRTCClient(
	sdp webrtc.SessionDescription,
	conductor ConductorInterface,
	signals chan<- string) (client *WebRTCClient, err error) {

	client = new(WebRTCClient)

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.stunprotocol.org:3478"}},
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			{URLs: []string{"stun:stun1.l.google.com:19302"}},
			{URLs: []string{"stun:stun2.l.google.com:19302"}},
			{URLs: []string{"stun:stun3.l.google.com:19302"}},
			{URLs: []string{"stun:stun4.l.google.com:19302"}},
		},
	}

	// Create a new API with Trickle ICE enabled
	// This SettingEngine allows non-standard WebRTC behavior
	s := webrtc.SettingEngine{}
	s.SetTrickle(true)
	api := webrtc.NewAPI(webrtc.WithSettingEngine(s))

	client.pc, err = api.NewPeerConnection(config)
	if err != nil {
		return
	}

	// Assign functions
	client.pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		msg, err := json.Marshal(c.ToJSON())
		if err != nil {
			return
		}
		signals <- string(msg)
	})

	client.pc.OnDataChannel(func(channel *webrtc.DataChannel) {
		label := channel.Label()
		switch label {
		case "data":
			client.tx = channel
			break

		case "command":
			client.rx = channel
			client.rx.OnMessage(client.receiveMessage)
			break

		default:
			panic(errors.New(fmt.Sprintf("Unkown data channel %s", label)))
		}
	})

	client.conductor = conductor

	// establish answer and setup the connection
	err = client.pc.SetRemoteDescription(sdp)
	if err != nil {
		panic(err)
	}
	go func() {
		answer, err := client.pc.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}
		err = client.pc.SetLocalDescription(answer)
		if err != nil {
			panic(err)
		}
		answerJson, err := json.Marshal(answer)
		if err != nil {
			panic(err)
		}
		signals <- string(answerJson)
		return
	}()

	return
}

func (client *WebRTCClient) AddIceCandidate(msg string) error {
	var ic webrtc.ICECandidateInit
	err := json.Unmarshal([]byte(msg), &ic)
	if err != nil {
		return errors.New("Unable to deserialize ice msg")
	}

	err = client.pc.AddICECandidate(ic)
	if err != nil {
		return err
	}
	fmt.Printf("added ice candidate: %v\n", ic)
	return nil
}

func (client *WebRTCClient) receiveMessage(msg webrtc.DataChannelMessage) {
	var cmd Cmd
	err := json.Unmarshal(msg.Data, &cmd)
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
	var sdp webrtc.SessionDescription
	err = json.Unmarshal([]byte(msg), &sdp)
	if err != nil {
		return
	}

	switch sdp.Type {
	case webrtc.SDPTypeOffer:
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
