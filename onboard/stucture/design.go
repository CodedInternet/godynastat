package stucture

import (
	"sync"
)

// completely abstract
type Device struct {
	platforms map[string]*Platform
	buses     map[string]*Bus
}

type IDevice interface {
	SetAngle(platformName, axisName string, angle float64)
	CommitAngle() // calls IBus.SendMsg as broadcast message
	ResetStage()  // calls IBus.SendMsg
}

type Platform struct {
	actuators map[string]*Actuator
	children  []*Platform
	parent    *Platform
}

type IPlatform interface {
	SetAngle(axisName string, angle float64)
	calculateLengths()
	IsReady() bool
	ResetStage() // checks range(actuators).Ready
}

// components
type Actuator struct {
	current, target int
	node            *Node
	ready           bool
}

type IActuator interface {
	SetLength(int) // sets ready to false until SendCommand returns success
	GetLength() (current int, err error)
	IsReady() bool
}

type Node struct {
	id         int
	actuators  map[string]*Actuator
	bus        Bus // has one bus
	lock       *sync.Mutex
	pendingMsg sync.WaitGroup
}

type INode interface {
	SendCommand(cmd int, data []byte) (success bool) // produces Cmd
	RecieveDumpFrames(frames []Cmd)                  // acquires lock, receives multiple frames then releases lock on completion/timeout
	ReceiveFrame(frame Cmd)                          // process multiple frame dump, or ACKs
	ResetStage()                                     // empties pendingMsg in case of failure
}

type Msg struct {
	// should be Cmd
	cmd     *Cmd
	success chan bool
}

type IMsg interface {
	SendAndACK() (success bool, err error) // sends CMD, waits for ACK, resends if not received after interval, returns error after X failures
}

// pure transport
type Cmd struct {
	// should be Msg
	cmd  int
	id   int
	dlc  uint8
	data []byte
	// ...
}

type ICmd interface {
	ToBytes() (raw []byte)
	FromBytes(raw []byte)
}

type Bus struct {
	name  string
	nodes map[int]*Node // has many nodes
}

type IBus interface {
	SendMsg(cmd Cmd)
	ReceivMsg(raw []byte) Cmd
}
