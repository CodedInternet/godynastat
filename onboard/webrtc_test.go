package main

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

func (c *mockConductor) SendSignal(sdp string) {
	c.tx <- sdp
}

func (c *mockConductor) ProcessCommand(cmd Cmd) {
	c.rx <- cmd
}

type mockClient struct {
	pc     *webrtc.PeerConnection
	tx, rx *webrtc.DataChannel
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
	remote.rx, err = remote.pc.CreateDataChannel("control", webrtc.Init{
		Ordered:  true,
		Protocol: "tcp",
	})

	offer, err := remote.pc.CreateOffer()
	if err != nil {
		panic(err)
	}
	remote.pc.SetLocalDescription(offer)

	// Generate a mock conductor
	conductor := new(mockConductor)
	conductor.tx = make(chan (string))
	conductor.rx = make(chan (Cmd))

	Convey("client generatation and answer creation works", t, func() {
		// Try to create our client
		client, err := NewWebRTCClient(offer, conductor)
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)

		Convey("answer has been sent to the conductor to be transmitted", func() {
			var tx string
			select {
			case tx = <-conductor.tx:
				var sdp map[string]interface{}
				json.Unmarshal([]byte(tx), &sdp)
				So(sdp["type"], ShouldEqual, "answer")
				break
			case <-time.After(time.Second * 1):
				t.Fatal("Answer timed out.")
			}

			Convey("connection can be opened", func() {
				sdp := webrtc.DeserializeSessionDescription(tx)
				remote.pc.SetRemoteDescription(sdp)

				open := make(chan (bool))
				remote.rx.OnOpen = func() {
					open <- true
				}
				select {
				case <-open:
					So(remote.rx.ReadyState(), ShouldEqual, webrtc.DataStateOpen)
				case <-time.After(time.Second):
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

}
