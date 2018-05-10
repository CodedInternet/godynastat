package hardware

type Actuator struct {
	State MotorState
	Node  ControlNode // the parent node that controls this actuator
	Index uint8       // the index of the motor in the node. Range: 1-4
	Ready bool        // sets to true when the node acknowledges the movement is staged.
}

func (m *Actuator) SetTarget(target uint8) {
	cmd := &CMDSetPos{
		cmd:      CMD_STAGE_POS,
		actuator: m,
	}

	m.Ready = false
	m.State.Target = target

	// blocks until success or err
	_, err := cmd.Process()
	if err != nil {
		// TODO: handle error
	}

	m.Ready = true
}
