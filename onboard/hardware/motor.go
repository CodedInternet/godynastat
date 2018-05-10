package hardware

type MotorState struct {
	Target, Current, Speed uint8
}

type MotorInterface interface {
	SetTarget(target uint8)
	GetState() (state MotorState)
}
