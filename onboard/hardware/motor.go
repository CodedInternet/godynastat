package hardware

type MotorState struct {
	Target, Current    uint16
	MaxSpeed, CurSpeed uint8
}

type MotorInterface interface {
	SetTarget(target uint8)
	GetState() (state MotorState)
}
