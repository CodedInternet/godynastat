package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/keroserene/go-webrtc"
	"os"
)

var err error
var dcs []*webrtc.DataChannel

func generateAnswer(pc *webrtc.PeerConnection) {
	fmt.Println("Generating answer...")
	answer, err := pc.CreateAnswer() // blocking
	if err != nil {
		fmt.Println(err)
		return
	}
	pc.SetLocalDescription(answer)
}

func receiveDescription(sdp *webrtc.SessionDescription, pc *webrtc.PeerConnection) {
	err = pc.SetRemoteDescription(sdp)
	if nil != err {
		fmt.Println("ERROR", err)
		return
	}
	fmt.Println("SDP " + sdp.Type + " successfully received.")
	if "offer" == sdp.Type {
		generateAnswer(pc)
	}
}

func signalSend(sdp string) {
	fmt.Println(sdp)
}

func broadcastMessage(msg []byte) {
	for _, dc := range dcs {
		dc.Send(msg)
	}
}

func processSignal(msg string) {
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(msg), &parsed)
	if err != nil {
		fmt.Errorf("%s, please try again", err)
		return
	}

	if parsed["sdp"] != nil {
		sdp := webrtc.DeserializeSessionDescription(msg)
		if sdp == nil {
			fmt.Errorf("Invalid session description")
		}

		fmt.Println("Starting up PeerConnection...")
		// TODO: Try with TURN servers.
		config := webrtc.NewConfiguration(
			webrtc.OptionIceServer("stun:stun.l.google.com:19302"))

		pc, err := webrtc.NewPeerConnection(config)
		if nil != err {
			fmt.Println("Failed to create PeerConnection.")
			return
		}

		// Once all ICE candidates are prepared, they need to be sent to the remote
		// peer which will attempt reaching the local peer through NATs.
		pc.OnIceComplete = func() {
			fmt.Println("Finished gathering ICE candidates.")
			sdp := pc.LocalDescription().Serialize()
			signalSend(sdp)
		}

		// Process the description
		receiveDescription(sdp, pc)

		dc, err := pc.CreateDataChannel("test", webrtc.Init{})
		dcs = append(dcs, dc)

		dc.OnOpen = func() {
			fmt.Println("Data Channel Opened!")
		}
		dc.OnClose = func() {
			fmt.Println("Data Channel closed.")
			return
		}
		dc.OnMessage = func(msg []byte) {
			fmt.Println(string(msg))
			// provide a response
			go broadcastMessage(msg)
		}
	}
}

func main() {
	webrtc.SetLoggingVerbosity(3)
	reader := bufio.NewReader(os.Stdin)

	for true {
		text, _ := reader.ReadString('\n')
		go processSignal(text)
	}

}
