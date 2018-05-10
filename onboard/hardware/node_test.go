package hardware

import (
	"errors"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	. "github.com/smartystreets/goconvey/convey"
	"sync"
	"testing"
)

type testBus struct {
	txerr, rxecho bool
	txCount       int
	lastTx        canbus.CANMsg
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
	}

	return nil
}

func createTestNodeBus() (tBus *testBus, tNode *ControlNode) {
	tBus = &testBus{
		listeners: make(map[uint32]chan canbus.CANMsg),
	}

	tNode = &ControlNode{
		id:         0x1234,
		actuators:  nil,
		bus:        tBus,
		lock:       new(sync.Mutex),
		pending:    sync.WaitGroup{},
		pendingCmd: make(map[uint16]*BaseCommand),
		rx:         make(chan canbus.CANMsg), // override to be a buffered channel
	}

	go tNode.listen()

	return
}

func TestControlNode(t *testing.T) {
	tBus, node := createTestNodeBus()

	Convey("sending a message goes through correctly", t, func() {
		msg := canbus.CANMsg{
			ID:  0xDEAD,
			Cmd: 0xBEEF,
		}

		node.SendMsg(msg)

		So(tBus.lastTx, ShouldResemble, msg)
	})

	Convey("listener is added", t, func() {
		So(tBus.listeners[node.id], ShouldNotBeNil)

		Convey("command stage works correctly", func() {
			tBus.rxecho = true // were entering the ACK command stage

			Convey("reset issues reset command", func() {
				err := node.StageReset()

				So(err, ShouldBeNil)
				So(tBus.lastTx.Cmd, ShouldEqual, CMD_STAGE_RESET)
			})

			Convey("commit blocks until queue pending is ready", func() {
				node.pending.Add(1)
				err := node.StageCommit()
				So(err, ShouldBeError)

				Convey("does not time out when queue is clear", func() {
					// use a goroutine to trigger the pending clear
					go func() {
						node.pending.Done()
					}()
					err := node.StageCommit()

					So(err, ShouldBeNil)
				})
			})
		})
	})
}
