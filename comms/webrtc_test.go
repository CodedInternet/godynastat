package comms

import (
	"encoding/json"
	"github.com/keroserene/go-webrtc"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
	"github.com/CodedInternet/godynastat/onboard"
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

type mockDynastat struct {
	lastCmd *Cmd
}

func (d *mockDynastat) GetState() (onboard.DynastatState, error) {
	panic("[NotImplemented]")
}

func (d *mockDynastat) GetConfig() *onboard.DynastatConfig {
	panic("[NotImplemented]")
}

func (d *mockDynastat) SetMotor(name string, position int) (err error) {
	d.lastCmd = &Cmd{
		"set_motor",
		name,
		position,
	}
	return nil
}

func (d *mockDynastat) HomeMotor(name string) error {
	d.lastCmd = &Cmd{
		"home_motor",
		name,
		0,
	}
	return nil
}

func (d *mockDynastat) GotoMotorRaw(name string, position int) error {
	d.lastCmd = &Cmd{
		"motor_goto_raw",
		name,
		position,
	}
	return nil
}

func (d *mockDynastat) WriteMotorRaw(name string, position int) error {
	panic("[NotImplemented]")
}

func (d *mockDynastat) RecordMotorLow(name string) error {
	panic("[NotImplemented]")
}

func (d *mockDynastat) RecordMotorHigh(name string) error {
	panic("[NotImplemented]")
}

func (d *mockDynastat) RecordMotorHome(name string, reverse bool) (pos int, err error) {
	panic("[NotImplemented]")
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

func TestConductor_ReceiveOffer(t *testing.T) {
	var err error

	conductor := new(Conductor)

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

	offer, err := remote.pc.CreateOffer()
	if err != nil {
		panic(err)
	}
	remote.pc.SetLocalDescription(offer)

	signals := make(chan string, 1)

	Convey("receiving a valid offer produces a new client", t, func() {
		conductor.ReceiveOffer(offer.Serialize(), signals)
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
			answer.Type = "answer"
			client, err := conductor.ReceiveOffer(answer.Serialize(), signals)
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
