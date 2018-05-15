package hardware

import (
	"errors"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	"sync"
	"time"

	"fmt"
	"github.com/Masterminds/semver"
)

const (
	nodeCmdMaxRetries = 5
	nodeCmdTimeout    = 50 * time.Millisecond
)

var (
	NodeVersion = "~0.1.0"
)

type ControlNode struct {
	id         uint32
	actuators  map[int]*Actuator
	bus        canbus.CANBusInterface
	lock       *sync.Mutex
	pending    sync.WaitGroup
	pendingCmd map[uint16]*cmdStatus
	rx         chan canbus.CANMsg
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

func NewControlNode(bus canbus.CANBusInterface, id uint32) (n *ControlNode, err error) {
	n = &ControlNode{
		id:         id,
		actuators:  nil,
		bus:        bus,
		lock:       new(sync.Mutex),
		pending:    sync.WaitGroup{},
		pendingCmd: make(map[uint16]*cmdStatus),
		rx:         make(chan canbus.CANMsg), // override to be a buffered channel
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
	} else if version.sha != "" {
		// todo: Check the commit hash against a list of acceptable commits. Perhaps this could be expanded to examine the git history?
	} else {
		// unable to process version number
		err = fmt.Errorf("unable to use node %d: unkown version", id)
	}

	return
}

func (n *ControlNode) Send(cmd NodeCommand) (NodeCommand, error) {
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

func (n *ControlNode) StageReset() (err error) {
	n.abortPending()

	n.Send(&EmptyCommand{cmdStageReset})
	return
}

func (n *ControlNode) StageCommit() (err error) {
	ready := make(chan struct{})

	go func() {
		defer close(ready)
		n.pending.Wait()
	}()

	select {
	case <-ready:
		n.Send(&EmptyCommand{cmdStageReset})
		return

	case <-time.After(time.Second):
		return errors.New("timed out waiting for staged commands")
	}
}

func (n *ControlNode) transmit(msg canbus.CANMsg) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	return n.bus.SendMsg(msg)
}

func (n *ControlNode) listen(ready chan<- struct{}) {
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

func (n *ControlNode) abortPending() {
	for _, status := range n.pendingCmd {
		close(status.err)
	}
}

func (n *ControlNode) routeResp(resp NodeCommand) {
	c, ok := n.pendingCmd[resp.CID()]
	if ok {
		c.resp <- resp
	}
}
