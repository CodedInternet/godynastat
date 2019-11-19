package hardware

import (
	. "math"
)

const (
	mMax   = 75
	mIncmm = 0xFFFF / mMax
)

type Actuator interface {
	GetCurrent() (current float64)
	GetTarget() (target float64)
	SetTarget(target float64)
	SetSpeed(speed uint8)
}

type LinearActuator struct {
	State MotorState
	Node  ControlNode // the parent node that controls this actuator
	Index uint8       // the Index of the motor in the node. Range: 1-4
	Ready bool        // sets to true when the node acknowledges the movement is staged.
}

func (a *LinearActuator) GetCurrent() (current float64) {
	return float64(a.State.Current / mIncmm)
}

func (a *LinearActuator) GetTarget() (target float64) {
	return float64(a.State.Target / mIncmm)
}

func (a *LinearActuator) SetTarget(target float64) {
	a.State.Target = uint16(Round(target * mIncmm))
}

func (a *LinearActuator) SetSpeed(speed uint8) {
	a.State.MaxSpeed = speed
}
