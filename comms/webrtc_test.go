package comms

import (
	"encoding/json"
	"github.com/pion/webrtc/v2"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

type mockConductor struct {
	tx chan string
	rx chan Cmd
}

func (c *mockConductor) ProcessCommand(cmd Cmd) {
	c.rx <- cmd
}

func (c *mockConductor) processSignals(t *testing.T, pc *webrtc.PeerConnection) {
	for {
		var err error
		var msg = <-c.tx
		var sig map[string]interface{}
		json.Unmarshal([]byte(msg), &sig)
		if _, ok := sig["type"]; ok {
			var sdp webrtc.SessionDescription
			err = json.Unmarshal([]byte(msg), &sdp)
			if err != nil {
				panic(err)
			}
			err = pc.SetRemoteDescription(sdp)
			if err != nil {
				panic(err)
			}
		} else if _, ok := sig["candidate"]; ok {
			if pc.RemoteDescription() == nil {
				continue
			}
			var ice webrtc.ICECandidateInit
			err = json.Unmarshal([]byte(msg), &ice)
			if err != nil {
				panic(err)
			}
			err = pc.AddICECandidate(ice)
			if err != nil {
				panic(err)
			}
		}
	}
}

type mockClient struct {
	pc          *webrtc.PeerConnection
	tx, rx      *webrtc.DataChannel
	iceComplete chan bool
	open        chan bool
}

type mockDynastat struct {
	lastCmd *Cmd
}

func (d *mockDynastat) SetRotation(platform string, degFront, degInc float64) (err error) {
	panic("implement me")
}

func (d *mockDynastat) SetHeight(platform string, height float64) (err error) {
	panic("implement me")
}

func (d *mockDynastat) SetFirstRay(platform string, angle float64) (err error) {
	panic("implement me")
}

func newMockClient() (remote *mockClient, err error) {
	// Build our remote party
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			//{URLs: []string{"stun:stun.stunprotocol.org:3478"}},
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			//{URLs: []string{"stun:stun1.l.google.com:19302"}},
			//{URLs: []string{"stun:stun2.l.google.com:19302"}},
			//{URLs: []string{"stun:stun3.l.google.com:19302"}},
			//{URLs: []string{"stun:stun4.l.google.com:19302"}},
		},
	}

	remote = new(mockClient)
	remote.pc, err = webrtc.NewPeerConnection(config)
	if err != nil {
		return
	}

	// create data channels
	False := false
	True := true
	udp := "udp"
	tcp := "tcp"
	remote.tx, err = remote.pc.CreateDataChannel("data", &webrtc.DataChannelInit{
		Ordered:  &False,
		Protocol: &udp,
	})
	if err != nil {
		return
	}

	remote.rx, err = remote.pc.CreateDataChannel("command", &webrtc.DataChannelInit{
		Ordered:  &True,
		Protocol: &tcp,
	})

	return
}

func TestWebRTCClient(t *testing.T) {
	var err error

	remote, err := newMockClient()
	if err != nil {
		panic(err)
	}

	// create a simple channel to monitor the opening of the connection
	remote.iceComplete = make(chan bool, 1)
	remote.pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		if state == webrtc.ICEGathererStateComplete {
			remote.iceComplete <- true
		}
	})
	remote.open = make(chan bool, 1)
	remote.rx.OnOpen(func() {
		t.Log("Opening")
		remote.open <- true
	})

	// Generate a mock conductor
	conductor := new(mockConductor)
	conductor.tx = make(chan string, 10)
	conductor.rx = make(chan Cmd, 1)
	go conductor.processSignals(t, remote.pc)

	offer, err := remote.pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	}
	err = remote.pc.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	Convey("client generatation and answer creation works", t, func() {
		// Try to create our client
		client, err := NewWebRTCClient(offer, conductor, conductor.tx)
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)

		Convey("Ice gathering works and is complete", func() {
			if remote.pc.ICEGatheringState().String() != webrtc.ICEGathererStateComplete.String() {
				select {
				case <-remote.iceComplete:
					So(remote.pc.ICEGatheringState(), ShouldEqual, webrtc.ICEGathererStateComplete)
				case <-time.After(time.Second * 10):
					t.Fatal("IceGatheringComplete timed out")
				}
			}

			Convey("connection can be opened", func() {
				select {
				case <-remote.open:
					So(remote.rx.ReadyState(), ShouldEqual, webrtc.DataChannelStateOpen)
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
		client.receiveMessage(webrtc.DataChannelMessage{Data: msg})

		rxCmd := <-conductor.rx
		So(rxCmd.Cmd, ShouldEqual, cmd.Cmd)
		So(rxCmd.Name, ShouldEqual, cmd.Name)
		So(rxCmd.Value, ShouldEqual, cmd.Value)
	})

}

func TestConductor_ReceiveOffer(t *testing.T) {
	var err error

	conductor := new(Conductor)

	// Build our remote party
	remote, err := newMockClient()
	if err != nil {
		panic(err)
	}

	offer, err := remote.pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	}
	remote.pc.SetLocalDescription(offer)

	signals := make(chan string, 1)

	Convey("receiving a valid offer produces a new client", t, func() {
		offerJson, err := json.Marshal(offer)
		if err != nil {
			panic(err)
		}
		conductor.ReceiveOffer(string(offerJson), signals)
		So(conductor.clients, ShouldNotBeEmpty)
		So(<-signals, ShouldNotBeBlank)
	})

	Convey("recieving invalid offers produces errors", t, func() {
		Convey("Not a valid SDP", func() {
			client, err := conductor.ReceiveOffer("hello world, I'm going to break this", signals)
			So(client, ShouldBeNil)
			So(err, ShouldNotBeNil)
		})

		Convey("SDP type is not offer - is answer", func() {
			answer := offer
			answer.Type = webrtc.SDPTypeAnswer
			answerJson, err := json.Marshal(answer)
			if err != nil {
				panic(err)
			}
			client, err := conductor.ReceiveOffer(string(answerJson), signals)
			So(client, ShouldBeNil)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestConductor(t *testing.T) {
	device := new(mockDynastat)

	conductor := Conductor{
		Device: device,
	}

	Convey("processing command triggers the correct device method", t, func() {
		t.Skipf("not yet updated to use new canbus message format")

		cmd := &Cmd{
			Name:  "TEST CMD",
			Value: 123,
		}

		device.lastCmd = nil
		cmd.Cmd = "set_motor"
		conductor.ProcessCommand(*cmd)
		So(device.lastCmd, ShouldResemble, cmd)

		device.lastCmd = nil
		cmd.Cmd = "motor_goto_raw"
		conductor.ProcessCommand(*cmd)
		So(device.lastCmd, ShouldResemble, cmd)

		device.lastCmd = nil
		cmd.Cmd = "home_motor"
		conductor.ProcessCommand(*cmd)
		cmd.Value = 0 // hard coded in mockDynastat, change for test
		So(device.lastCmd, ShouldResemble, cmd)
		cmd.Value = 123 // reset back in case we use it again later
	})
}

func BenchmarkConductor_ReceiveOffer(b *testing.B) {
	var err error

	conductor := new(Conductor)

	// Build our remote party
	remote, err := newMockClient()
	if err != nil {
		panic(err)
	}

	offer, err := remote.pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	}
	remote.pc.SetLocalDescription(offer)

	offerJson, err := json.Marshal(offer)
	if err != nil {
		panic(err)
	}
	signals := make(chan string, 1)

	var c *WebRTCClient

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c, err = conductor.ReceiveOffer(string(offerJson), signals)
		if err != nil {
			b.Fatal(err)
		}
	}
	if c == nil {
		b.Fatalf("client has not been defined")
	}
}
