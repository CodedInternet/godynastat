package hardware

import (
	"errors"
	"github.com/Masterminds/semver"
	"reflect"
)

const (
	cmdAllstop  = 0x0000
	cmdAllStart = 0x0010
	//RESERVED	        = 0x0020
	cmdUpdateInterval = 0x0030
	cmdGetPos         = 0x0040
	cmdSetPos         = 0x0050
	cmdSetSpeed       = 0x0060
	cmdHome           = 0x0070
	cmdNvmUpdate      = 0x0080
	cmdScanI2C        = 0x0090
	cmdI2CRead        = 0x00A0
	cmdI2CWrite       = 0x00B0
	cmdAccelUpdate    = 0x0100
	cmdSensorUpdate   = 0x0110
	cmdVersion        = 0x03E0
)

var (
	ErrMaxRetries = errors.New("nodeCmdMaxRetries reached while attempting to send")
	ErrSendAbort  = errors.New("send has been aborted")

	// a map of all the custom command type ID's and the associated types. Can be used later
	CMDMap = map[uint16]reflect.Type{
		cmdGetPos:   reflect.TypeOf(CMDGetPos{}),
		cmdSetPos:   reflect.TypeOf(CMDSetPos{}),
		cmdSetSpeed: reflect.TypeOf(CMDSetSpeed{}),
		cmdVersion:  reflect.TypeOf(CMDVersion{}),
	}
)

// Provides a common interface for all commands sent or received from a node.
type NodeCommand interface {
	// Provides a pseudo-unique ID for this command so responses can be correlated correctly.
	CID() uint16

	// Returns the underlying command bits to be set as part of the CAN ID
	CMD() uint16

	// Converts the command to the raw data bytes required to be transmitted on the wire.
	// Should never return len([]byte) > 8 as this cannot be transmitted on the wire.
	TXData() []byte

	// Processes incoming data back into a command for future comparison/upstream processing
	RXData([]byte)
}

func cmdReflect(t reflect.Type) NodeCommand {
	return reflect.New(t).Interface().(NodeCommand)
}

// Represents a very simple hacky command that does not have any data.
// Makes it simple to send a basic command.
//
// This should only ever be used when there is no data transmitted in either direction such as cmdAllstop
type EmptyCommand struct {
	cmd uint16
}

func (c *EmptyCommand) CID() uint16 {
	return c.cmd
}

func (c *EmptyCommand) CMD() uint16 {
	return c.cmd
}

func (c *EmptyCommand) TXData() []byte {
	return make([]byte, 0)
}

func (c *EmptyCommand) RXData([]byte) {
	return
}

type CMDGetPos struct {
	Positions [4]uint16
}

func (c *CMDGetPos) CID() uint16 {
	return c.CMD()
}

func (c *CMDGetPos) CMD() uint16 {
	return cmdGetPos
}

func (c *CMDGetPos) TXData() []byte {
	return make([]byte, 0)
}

func (c *CMDGetPos) RXData(data []byte) {
	for _, i := range c.Positions {
		c.Positions[i] = uint16(data[2*i]<<8 | data[2*i])
	}
}

// Command to take set an actuator to a Position directly without staging the movement
type CMDSetPos struct {
	Positions [4]uint16
}

func (c *CMDSetPos) CID() uint16 {
	return c.CMD()
}

func (*CMDSetPos) CMD() uint16 {
	return cmdSetPos
}

func (c *CMDSetPos) TXData() []byte {
	ret := make([]byte, len(c.Positions)*2)
	for i, p := range c.Positions {
		ret[2*i] = byte(p >> 8)
		ret[2*i+1] = byte(p & 0xFF)
	}
	return ret
}

func (c *CMDSetPos) RXData(data []byte) {
	for i := range c.Positions {
		c.Positions[i] = uint16(data[2*i]<<8 | data[2*i])
	}
}

// Command to stage a motor movement. Does not perform any action until a cmdStageCommit is received
type CMDSetSpeed struct {
	Speeds [4]uint8
}

func (c *CMDSetSpeed) CID() uint16 {
	return c.CMD()
}

func (*CMDSetSpeed) CMD() uint16 {
	return cmdSetSpeed
}

func (c *CMDSetSpeed) TXData() []byte {
	ret := make([]byte, len(c.Speeds))
	for i, s := range c.Speeds {
		ret[i] = s
	}
	return ret
}

func (c *CMDSetSpeed) RXData(data []byte) {
	for _, i := range c.Speeds {
		c.Speeds[i] = data[i]
	}
}

type CMDVersion struct {
	version *semver.Version
	sha     string
	dev     bool
}

func (c *CMDVersion) CID() uint16 {
	return cmdVersion
}

func (c *CMDVersion) CMD() uint16 {
	return cmdVersion
}

func (c *CMDVersion) TXData() []byte {
	return make([]byte, 0)
}

func (c *CMDVersion) RXData(data []byte) {
	versionString := string(data)
	var err error
	c.version, err = semver.NewVersion(versionString)

	if err != nil {
		// non semver, check if this is a commit or dev
		if versionString == "DEV" {
			c.dev = true
		} else if len(versionString) == 7 {
			c.sha = versionString
		}
	}
}
