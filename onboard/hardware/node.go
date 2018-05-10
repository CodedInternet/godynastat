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
	NODE_VERSION = "~0.1.0"
)

type ControlNode struct {
	id         uint32
	actuators  map[int]*Actuator
	bus        canbus.CANBusInterface
	lock       *sync.Mutex
	pending    sync.WaitGroup
	pendingCmd map[uint16]*BaseCommand
	rx         chan canbus.CANMsg
}

func NewControlNode(bus canbus.CANBusInterface, id uint32) (n *ControlNode, err error) {
	n = &ControlNode{
		id:         id,
		actuators:  nil,
		bus:        bus,
		lock:       new(sync.Mutex),
		pending:    sync.WaitGroup{},
		pendingCmd: make(map[uint16]*BaseCommand),
		rx:         make(chan canbus.CANMsg), // override to be a buffered channel
	}

	go n.listen()

	// check version is acceptable
	vc := &CMDVersion{
		&BaseCommand{
			node: n,
		},
	}

	resp, err := vc.Process()
	if err != nil {
		return
	}

	versionString := string(resp.Data)
	semVer, err := semver.NewVersion(versionString)
	if err != nil {
		// not a semver, but we might be able to recover

		if versionString == "DEV" {
			// running a direct dev version, consider it safe for now but require a flag in the future
			// todo: add support for running a dev version via config/env/cli flag
			err = nil
		} else if len(versionString) == 7 {
			// running a direct commit build, assume it is unsafe as this shouldn't happen
			// todo: add support for running a specific commit via config/env/cli flag
			return
		}
	}

	// check semver
	semVerConstraint, err := semver.NewConstraint(NODE_VERSION)
	if err != nil {
		return
	}

	if !semVerConstraint.Check(semVer) {
		err = fmt.Errorf("unable to use node %d: recieved version %s - require %s", id, versionString, NODE_VERSION)
	}

	return
}

func (n *ControlNode) SendMsg(msg canbus.CANMsg) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	return n.bus.SendMsg(msg)
}

func (n *ControlNode) StageReset() (err error) {
	n.abortPending()

	resetCmd := &BaseCommand{
		node: n,
		msg: canbus.CANMsg{
			ID:  n.id,
			Cmd: CMD_STAGE_RESET,
		},
	}

	_, err = resetCmd.Process()
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
		commitCmd := &BaseCommand{
			node: n,
			msg: canbus.CANMsg{
				ID:  n.id,
				Cmd: CMD_STAGE_COMMIT,
			},
		}

		_, err = commitCmd.Process()
		return

	case <-time.After(time.Second):
		return errors.New("timed out waiting for staged commands")
	}
}

func (n *ControlNode) listen() {
	if n.rx == nil {
		n.rx = make(chan canbus.CANMsg)
	}
	n.bus.AddListener(n.id, n.rx)

	for {
		msg := <-n.rx

		switch msg.Cmd {
		case CMD_STAGE_POS:
			resp := &CMDSetPos{
				BaseCommand: &BaseCommand{
					msg: msg,
				},
				cmd: msg.Cmd,
			}

			n.routeACK(resp)

		default:
			n.routeACK(&BaseCommand{
				msg: msg,
			})

		}
	}
}

func (n *ControlNode) abortPending() {
	for _, cmd := range n.pendingCmd {
		cmd.Abort()
	}
}

func (n *ControlNode) routeACK(resp NodeCommand) {
	n.pendingCmd[resp.ID()].Ack(resp.Msg())
}
