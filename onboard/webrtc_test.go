package onboard

import (
	"encoding/json"
	"github.com/keroserene/go-webrtc"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

type mockConductor struct {
	tx chan (string)
	rx chan (Cmd)
}

func (c *mockConductor) ProcessCommand(cmd Cmd) {
	c.rx <- cmd
}

func (c *mockConductor) processSignals(t *testing.T, pc *webrtc.PeerConnection) {
	for {
		var msg = <-c.tx
		var sig map[string]interface{}
		json.Unmarshal([]byte(msg), &sig)
		if _, ok := sig["type"]; ok {
			sdp := webrtc.DeserializeSessionDescription(msg)
			pc.SetRemoteDescription(sdp)
		} else if _, ok := sig["candidate"]; ok {
			ice := webrtc.DeserializeIceCandidate(msg)
			pc.AddIceCandidate(*ice)
		}
	}
}

type mockClient struct {
	pc          *webrtc.PeerConnection
	tx, rx      *webrtc.DataChannel
	iceComplete chan bool
	open        chan bool
}

func TestWebRTCClient(t *testing.T) {
	var err error
	// Build our remote party
	config := webrtc.NewConfiguration(
		webrtc.OptionIceServer("stun:stun.l.google.com:19302"),
	)

	remote := new(mockClient)
	remote.pc, err = webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// create data channels
	remote.tx, err = remote.pc.CreateDataChannel("data", webrtc.Init{
		Ordered:  true,
		Protocol: "udp",
	})
	if err != nil {
		panic(err)
	}

	remote.rx, err = remote.pc.CreateDataChannel("command", webrtc.Init{
		Ordered:  true,
		Protocol: "tcp",
	})
	if err != nil {
		panic(err)
	}

	// create a simple channel to monitor the opening of the connection
	remote.iceComplete = make(chan bool, 1)
	remote.pc.OnIceComplete = func() {
		remote.iceComplete <- true
	}
	remote.open = make(chan bool, 1)
	remote.rx.OnOpen = func() {
		t.Log("Opening")
		remote.open <- true
	}

	// Generate a mock conductor
	conductor := new(mockConductor)
	conductor.tx = make(chan string, 10)
	conductor.rx = make(chan (Cmd), 1)
	go conductor.processSignals(t, remote.pc)

	offer, err := remote.pc.CreateOffer()
	if err != nil {
		panic(err)
	}
	remote.pc.SetLocalDescription(offer)

	Convey("client generatation and answer creation works", t, func() {
		// Try to create our client
		client, err := NewWebRTCClient(offer, conductor, conductor.tx)
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)

		Convey("Ice gathering works and is complete", func() {
			select {
			case <-remote.iceComplete:
				So(remote.pc.IceGatheringState(), ShouldEqual, webrtc.IceGatheringStateComplete)
			case <-time.After(time.Second):
				t.Fatal("IceGatheringComplete timed out")
			}

			Convey("connection can be opened", func() {
				select {
				case <-remote.open:
					So(remote.rx.ReadyState(), ShouldEqual, webrtc.DataStateOpen)
					break
				case <-time.After(time.Second * 5):
					// we have timed out waiting
					t.Fatal("OnOpen timed out.")
				}

				Convey("Commands are recieved properly", func() {
					cmd := new(Cmd)
					cmd.Cmd = "test"
					cmd.Name = "webrtc"
					cmd.Value = 123
					msg, err := json.Marshal(cmd)
					if err != nil {
						panic(err)
					}

					remote.rx.Send(msg)

					select {
					case rxCmd := <-conductor.rx:
						So(rxCmd.Cmd, ShouldEqual, cmd.Cmd)
						So(rxCmd.Name, ShouldEqual, cmd.Name)
						So(rxCmd.Value, ShouldEqual, cmd.Value)
					case <-time.After(time.Second):
						t.Fatal("rxCmd was not recieved")
					}
				})
			})
		})
	})

	Convey("processing commands", t, func() {
		client := new(WebRTCClient)
		client.conductor = conductor
		cmd := new(Cmd)
		cmd.Cmd = "test"
		cmd.Name = "processCommand"
		cmd.Value = 123
		msg, err := json.Marshal(cmd)
		if err != nil {
			panic(err)
		}
		client.receiveMessage(msg)

		rxCmd := <-conductor.rx
		So(rxCmd.Cmd, ShouldEqual, cmd.Cmd)
		So(rxCmd.Name, ShouldEqual, cmd.Name)
		So(rxCmd.Value, ShouldEqual, cmd.Value)
	})

}
