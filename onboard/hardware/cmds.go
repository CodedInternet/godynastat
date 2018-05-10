package hardware

import (
	"bytes"
	"errors"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	"time"
)

const (
	CMD_ALLSTOP         = 0x0000
	CMD_STAGE_COMMIT    = 0x0010
	CMD_STAGE_RESET     = 0x0020
	CMD_UPDATE_INTERVAL = 0x0030
	CMD_GET_POS         = 0x0040
	CMD_SET_POS         = 0x0050
	CMD_STAGE_POS       = 0x0060
	CMD_CALIBRATE       = 0x0070
	CMD_NVM_UPDATE      = 0x0080
	CMD_SCAN_I2C        = 0x0090
	CMD_I2C_READ        = 0x00A0
	CMD_I2C_WRITE       = 0x00B0
	CMD_ACCEL_UPDATE    = 0x0100
	CMD_SENSOR_UPDATE   = 0x0110
	CMD_VERSION         = 0x03E0

	CMD_MAX_RETRIES = 5
	CMD_TIMEOUT     = 5 * time.Millisecond
)

var (
	ERR_MAX_RETRIES = errors.New("CMD_MAX_RETRIES reached while attempting to send")
	ERR_SEND_ABORT  = errors.New("send has been aborted")
)

type NodeCommand interface {
	ID() uint16
	Process() (resp canbus.CANMsg, err error)
	Ack(msg canbus.CANMsg)
	Msg() canbus.CANMsg
	Abort() error
}

type BaseCommand struct {
	node  *ControlNode
	msg   canbus.CANMsg
	ack   chan canbus.CANMsg
	abort chan struct{}
}

// Sends the current command and waits for a response/acknowledgment from the node.
// Will retry commands that are not acknowledgement within CMD_TIMEOUT up to CMD_MAX_RETRIES.
// Can be canceled by closing the abort channel
// Returns the response to the message for upstream processing should it be necessary
// Returns an error if the maximum retries are reached without an acknowledgement.
func (c *BaseCommand) Process() (resp canbus.CANMsg, err error) {
	c.node.pending.Add(1)       // add to the wait group
	defer c.node.pending.Done() // whatever happens this should decrement when it fails

	// register the callback with the node
	c.node.pendingCmd[c.ID()] = c
	defer delete(c.node.pendingCmd, c.ID())

	if c.ack == nil {
		c.ack = make(chan canbus.CANMsg)
	}

	if c.abort == nil {
		c.abort = make(chan struct{})
	}

	// attempt initial sending
	msg := c.Msg()
	err = c.node.SendMsg(msg)
	if err != nil {
		return resp, err
	}

	for i := 1; i < CMD_MAX_RETRIES; i++ {
		select {
		case resp := <-c.ack:
			if c.verify(resp) {
				return resp, nil
			}

		case <-c.abort:
			return resp, ERR_SEND_ABORT

		case <-time.After(CMD_TIMEOUT):
			err = c.node.SendMsg(msg)
			if err != nil {
				return resp, err
			}
		}
	}

	// we have exhausted MAX_RETRIES
	return resp, ERR_MAX_RETRIES
}

func (c *BaseCommand) verify(msg canbus.CANMsg) bool {
	return bytes.Equal(c.msg.Data, msg.Data)
}

func (c *BaseCommand) ID() uint16 {
	return c.Msg().Cmd
}

func (c *BaseCommand) SetNode(node *ControlNode) {
	c.node = node
}

func (c *BaseCommand) Msg() canbus.CANMsg {
	return c.msg
}

func (c *BaseCommand) Abort() error {
	if c.abort == nil {
		return errors.New("send not yet attempted")
	}

	close(c.abort)
	return nil
}

func (c *BaseCommand) Ack(msg canbus.CANMsg) {
	c.ack <- msg
}

// commits any curretly staged commands
type CMDCommit struct {
	*BaseCommand
}

func (c *CMDCommit) Msg() (msg canbus.CANMsg) {
	c.msg.Cmd = CMD_STAGE_COMMIT
	c.msg.ID = c.node.id

	return c.msg
}

// Issues a set position command. ID is based on msg.Cmd and the actuator index.
type CMDSetPos struct {
	*BaseCommand
	cmd      uint16
	actuator *Actuator
}

func (c *CMDSetPos) ID() uint16 {
	return CMD_STAGE_POS | uint16(c.actuator.Index)
}

func (c *CMDSetPos) Msg() (msg canbus.CANMsg) {
	if c.cmd == 0 {
		c.cmd = CMD_SET_POS
	}
	c.msg.Cmd = c.cmd
	c.msg.ID = c.actuator.Node.id
	c.msg.Data = make([]byte, 3)
	c.msg.Data[0] = c.actuator.Index
	c.msg.Data[1] = c.actuator.State.Target
	c.msg.Data[2] = c.actuator.State.Speed

	return c.msg
}

// Requests the current position
type CMDVersion struct {
	*BaseCommand
}

func (c *CMDVersion) Msg() (msg canbus.CANMsg) {
	println("testing version")
	c.msg.Cmd = CMD_VERSION
	c.msg.ID = c.node.id

	return c.msg
}
