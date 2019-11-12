package hardware

type Actuator interface {
	GetCurrent() (current uint8)
	GetTarget() (target uint8)
	SetTarget(target, speed uint8)
}

type LinearActuator struct {
	State MotorState
	Node  ControlNode // the parent node that controls this actuator
	Index uint8       // the Index of the motor in the node. Range: 1-4
	Ready bool        // sets to true when the node acknowledges the movement is staged.
}

func (a *LinearActuator) GetCurrent() (current uint8) {
	return a.State.Current
}

func (a *LinearActuator) GetTarget() (target uint8) {
	return a.State.Target
}

func (a *LinearActuator) SetTarget(target, speed uint8) {
	a.Ready = false
	a.State.Target = target

	// blocks until success or err
	_, err := a.Node.Send(&CMDStagePos{
		a.Index,
		a.State.Target,
		speed,
	})

	if err != nil {
		// TODO: handle error
		panic(err)
	}

	a.Ready = true
}
