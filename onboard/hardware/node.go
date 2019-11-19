package hardware

import (
	"github.com/CodedInternet/godynastat/onboard/canbus"
	"os"
	"sync"
	"time"

	"fmt"
	"github.com/Masterminds/semver"
)

const (
	nodeCmdMaxRetries       = 5
	nodeCmdTimeout          = 50 * time.Millisecond
	nodeMotorUpdateInterval = 100 * time.Millisecond
)

var (
	NodeVersion = "~0.1.0"
)

type ControlNode interface {
	Send(cmd NodeCommand) (NodeCommand, error)
	SetTargets() (err error)
	SetSpeeds() (err error)
	Home() (err error)
}

type MotorControlNode struct {
	id         uint32
	Actuators  [4]*LinearActuator
	bus        canbus.CANBusInterface
	lock       *sync.Mutex
	pending    sync.WaitGroup
	pendingCmd map[uint16]*cmdStatus
	rx         chan canbus.CANMsg
}

func (n *MotorControlNode) SetTargets() (err error) {
	_, err = n.Send(&CMDSetPos{[4]uint16{
		n.Actuators[0].State.Target,
		n.Actuators[1].State.Target,
		n.Actuators[2].State.Target,
		n.Actuators[3].State.Target,
	}})
	return
}

func (n *MotorControlNode) SetSpeeds() (err error) {
	_, err = n.Send(&CMDSetSpeed{[4]uint8{
		n.Actuators[0].State.MaxSpeed,
		n.Actuators[1].State.MaxSpeed,
		n.Actuators[2].State.MaxSpeed,
		n.Actuators[3].State.MaxSpeed,
	}})
	return
}

type cmdStatus struct {
	resp chan NodeCommand
	err  chan error
}

func newStatus() *cmdStatus {
	return &cmdStatus{
		make(chan NodeCommand),
		make(chan error),
	}
}

func NewControlNode(bus canbus.CANBusInterface, id uint32) (n *MotorControlNode, err error) {
	n = &MotorControlNode{
		id: id,
		Actuators: [4]*LinearActuator{
			new(LinearActuator),
			new(LinearActuator),
			new(LinearActuator),
			new(LinearActuator),
		},
		bus:        bus,
		lock:       new(sync.Mutex),
		pending:    sync.WaitGroup{},
		pendingCmd: make(map[uint16]*cmdStatus),
		rx:         make(chan canbus.CANMsg), // override to be a buffered channel
	}

	for i := range n.Actuators {
		n.Actuators[i].Node = n
	}

	ready := make(chan struct{})
	go n.listen(ready)

	<-ready

	// check version is acceptable
	vc := &CMDVersion{}

	resp, err := n.Send(vc)

	if err != nil {
		err = fmt.Errorf("unable to determine version: %s", err)
		return
	}

	version := resp.(*CMDVersion)
	if version.version != nil {
		// check semver
		semVerConstraint, err := semver.NewConstraint(NodeVersion)
		if err != nil {
			return n, err
		}

		if !semVerConstraint.Check(version.version) {
			err = fmt.Errorf("unable to use node %d: recieved version %s - require %s", id, version.version, NodeVersion)
			return n, err
		}
	} else if version.dev == true {
		// todo: Check if we are in dev mode rather than just accepting it
		fmt.Printf("ruuning board %d in dev mode\n", id)
	} else if version.sha != "" {
		// todo: Check the commit hash against a list of acceptable commits. Perhaps this could be expanded to examine the git history?
	} else {
		// unable to process version number
		err = fmt.Errorf("unable to use node %d: unkown version", id)
	}

	//go n.updatePositions()

	return
}

// Sends a command to the node and awaits a response which is returned.
// This is a synchronous function around an async behavior, and is controlled by an internal timeout.
func (n *MotorControlNode) Send(cmd NodeCommand) (NodeCommand, error) {
	var (
		msg = canbus.CANMsg{
			ID:   n.id,
			Cmd:  cmd.CMD(),
			Data: cmd.TXData(),
		}
		status   = newStatus()
		complete = newStatus()
	)
	n.pending.Add(1)
	n.pendingCmd[cmd.CID()] = status
	defer n.pending.Done()
	defer delete(n.pendingCmd, cmd.CID())

	go func() {
		err := n.transmit(msg)
		if err != nil {
			complete.err <- err
			return
		}

		for i := 1; i < nodeCmdMaxRetries; i++ {
			select {
			case resp := <-status.resp:
				complete.resp <- resp
				return

			case <-status.err:
				complete.err <- ErrSendAbort
				return

			case <-time.After(nodeCmdTimeout):
				err := n.transmit(msg)
				if err != nil {
					complete.err <- err
					return
				}
			}
		}

		complete.err <- ErrMaxRetries
		return
	}()

	select {
	case resp := <-complete.resp:
		return resp, nil

	case err := <-complete.err:
		return nil, err
	}
}

func (n *MotorControlNode) Home() (err error) {
	_, err = n.Send(&EmptyCommand{cmdHome})
	return
}

func (n *MotorControlNode) transmit(msg canbus.CANMsg) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	return n.bus.SendMsg(msg)
}

func (n *MotorControlNode) listen(ready chan<- struct{}) {
	if n.rx == nil {
		n.rx = make(chan canbus.CANMsg)
	}
	n.bus.AddListener(n.id, n.rx)
	close(ready)

	for {
		msg := <-n.rx

		var resp NodeCommand

		if cmdType, ok := CMDMap[msg.Cmd]; ok {
			resp = cmdReflect(cmdType)
		} else {
			resp = &EmptyCommand{msg.Cmd}
		}

		resp.RXData(msg.Data)

		n.routeResp(resp)
	}
}

func (n *MotorControlNode) abortPending() {
	for _, status := range n.pendingCmd {
		close(status.err)
	}
}

func (n *MotorControlNode) routeResp(resp NodeCommand) {
	c, ok := n.pendingCmd[resp.CID()]
	if ok {
		c.resp <- resp
	}
}

func (n *MotorControlNode) updatePositions() {
	for {
		resp, err := n.Send(&CMDGetPos{})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "unable to update position: %s", err.Error())
		}

		getPos := resp.(*CMDGetPos)
		for i, actuator := range n.Actuators {
			actuator.State.Current = getPos.Positions[i]
		}

		time.Sleep(nodeMotorUpdateInterval)
	}
}
