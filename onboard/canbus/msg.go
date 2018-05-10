package canbus

import (
	"errors"
)

const (
	CANHostFlag = 0x0400
	CANIDMask   = 0x1234
	CANCMDMask  = 0xFFFF
)

// errors
var (
	ERR_DATA_TOO_LONG = errors.New("data length exceeds 6 bytes")
)

type CANMsg struct {
	ID   uint32 // node ID this is being issued for
	Cmd  uint16 // command being issued in this message
	Data []byte // raw data up to six bytes. DLC is taken from len(Data).
}
