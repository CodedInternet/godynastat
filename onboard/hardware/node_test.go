package hardware

import (
	"errors"
	"fmt"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	. "github.com/smartystreets/goconvey/convey"
	"sync"
	"testing"
)

type testBus struct {
	txerr, rxecho bool
	txCount       int
	lastTx        canbus.CANMsg
	rxmsg         *canbus.CANMsg
	listeners     map[uint32]chan canbus.CANMsg
}

func (t *testBus) AddListener(nodeId uint32, rxchan chan canbus.CANMsg) {
	t.listeners[nodeId] = rxchan
}

func (t *testBus) SendMsg(msg canbus.CANMsg) error {
	t.lastTx = msg
	t.txCount++
	if t.txerr {
		return errors.New("this is a simulated tx error")
	}

	if t.rxecho {
		c, ok := t.listeners[msg.ID]
		if !ok || c == nil {
			return errors.New("unable to find listener")
		}
		c <- msg // echo back for ACK
	} else if t.rxmsg != nil {
		c, ok := t.listeners[msg.ID]
		if !ok || c == nil {
			return errors.New("unable to find listener")
		}
		c <- *t.rxmsg // echo back for ACK
	}

	return nil
}

func createTestNodeBus() (tBus *testBus, tNode *MotorControlNode) {
	tBus = &testBus{
		listeners: make(map[uint32]chan canbus.CANMsg),
	}

	tNode = &MotorControlNode{
		id: 0x1234,
		Actuators: [4]*LinearActuator{
			new(LinearActuator),
			new(LinearActuator),
			new(LinearActuator),
			new(LinearActuator),
		},
		bus:        tBus,
		lock:       new(sync.Mutex),
		pending:    sync.WaitGroup{},
		pendingCmd: make(map[uint16]*cmdStatus),
		rx:         make(chan canbus.CANMsg, 1), // override to be a buffered channel
	}

	ready := make(chan struct{})
	go tNode.listen(ready)
	<-ready

	return
}

func TestControlNode(t *testing.T) {
	tBus, node := createTestNodeBus()

	Convey("sending a message goes through correctly", t, func() {
		msg := canbus.CANMsg{
			ID:  0xDEAD,
			Cmd: 0xBEEF,
		}

		node.transmit(msg)

		So(tBus.lastTx, ShouldResemble, msg)
	})

	Convey("listener is added", t, func() {
		So(tBus.listeners[node.id], ShouldNotBeNil)

		Convey("node pulls data from actuators correctly", func() {
			tBus.rxecho = true // were entering the ACK command stage

			for i, a := range node.Actuators {
				a.SetTarget(float64(i + 1))
			}
			err := node.SetTargets()

			So(err, ShouldBeNil)
			So(tBus.lastTx.Cmd, ShouldEqual, cmdSetPos)
			So(tBus.lastTx.Data, ShouldHaveLength, 8)
			So(tBus.lastTx.Data, ShouldResemble, []byte{3, 105, 6, 210, 10, 59, 13, 164}) // correct values for 75mm
		})

		Convey("commands get sent correctly", func() {
			Convey("command attempts multiple times then times out", func() {
				tBus.rxecho = false
				tBus.txCount = 0

				cmd := &EmptyCommand{0x1234}

				_, err := node.Send(cmd)
				So(err, ShouldEqual, ErrMaxRetries)
				So(tBus.txCount, ShouldEqual, nodeCmdMaxRetries)
			})

			Convey("aborting returns correct error and does not send till max", func() {
				tBus.rxecho = false
				tBus.txCount = 0

				cmd := &EmptyCommand{0x1234}

				go node.abortPending()
				_, err := node.Send(cmd)
				So(err, ShouldEqual, ErrSendAbort)
				So(tBus.txCount, ShouldBeLessThan, nodeCmdMaxRetries)
			})

			Convey("simple echo commands get routed correctly", func() {
				tBus.rxecho = true
				tBus.txCount = 0

				cmd := &EmptyCommand{0x1234}

				resp, err := node.Send(cmd)

				So(err, ShouldBeNil)
				So(resp, ShouldHaveSameTypeAs, cmd)
				So(resp.CMD(), ShouldEqual, cmd.cmd)

				// check it has successfully sent
				So(tBus.txCount, ShouldEqual, 1) // sent exactly once
				So(tBus.lastTx.Cmd, ShouldEqual, cmd.cmd)
				So(tBus.lastTx.ID, ShouldEqual, node.id)

				Convey("a more complex example involving reflection", func() {
					tBus.txCount = 0
					cmd := &CMDSetPos{[4]uint16{
						0x0102,
						0x0304,
						0x0506,
						0x0708,
					}}

					resp, err := node.Send(cmd)

					So(err, ShouldBeNil)
					So(resp, ShouldHaveSameTypeAs, cmd)
				})

				Reset(func() {
					tBus.rxecho = false
				})
			})
		})
	})

	Convey("constructor works as expected", t, func() {
		const tNodeID = 0x42

		tBus.rxmsg = &canbus.CANMsg{
			ID:   tNodeID,
			Cmd:  cmdVersion,
			Data: []byte{'D', 'E', 'V'},
		}

		tNode, err := NewControlNode(tBus, tNodeID)
		So(err, ShouldBeNil)
		So(tBus.listeners[tNodeID], ShouldNotBeNil)

		So(tNode, ShouldNotBeNil)
		So(tNode.rx, ShouldNotBeNil)

		Convey("test version with valid commit hash", func() {
			tBus.rxmsg.Data = []byte{'1', 'b', '3', 'd', '5', 'f', '7'}
			tNode, err := NewControlNode(tBus, tNodeID)
			So(err, ShouldBeNil)
			So(tNode, ShouldNotBeNil)

			Convey("invalid hash does not work", func() {
				tBus.rxmsg.Data = []byte{'1', 'b', '3', 'd', '5', 'f'} // 6 chars is not valid

				_, err := NewControlNode(tBus, tNodeID)
				So(err, ShouldBeError, fmt.Sprintf("unable to use node %d: unkown version", tNodeID))
			})
		})

		Convey("a valid semver is allowed", func() {
			NodeVersion = "^0.1.0"
			tBus.rxmsg.Data = []byte{'0', '.', '2', '.', '1', '2'}
			tNode, err := NewControlNode(tBus, tNodeID)
			So(err, ShouldBeNil)
			So(tNode, ShouldNotBeNil)

			Convey("when version is unsupported", func() {
				NodeVersion = "~0.1.0"
				_, err := NewControlNode(tBus, tNodeID)
				So(err, ShouldBeError, fmt.Sprintf("unable to use node %d: recieved version %s - require %s",
					tNodeID, "0.2.12", NodeVersion))
			})
		})

		Reset(func() {
			tBus.rxmsg = nil
		})
	})
}
