package hardware

type Actuator struct {
	State MotorState
	Node  ControlNode // the parent node that controls this actuator
	Index uint8       // the Index of the motor in the node. Range: 1-4
	Ready bool        // sets to true when the node acknowledges the movement is staged.
}

func (m *Actuator) SetTarget(target uint8) {
	m.Ready = false
	m.State.Target = target

	// blocks until success or err
	_, err := m.Node.Send(&CMDStagePos{
		m.Index,
		m.State.Target,
		255, // todo: make Speed dynamically controllable
	})

	if err != nil {
		// TODO: handle error
		panic(err)
	}

	m.Ready = true
}
