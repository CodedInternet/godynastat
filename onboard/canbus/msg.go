package canbus

import (
	"errors"
)

const (
	canSFFNodeFlag   = 0x0400
	canIDMask        = 0x000F
	canCMDMask       = 0x03F0
	CANBroadcastFlag = 0x000F
)

// errors
var (
	ERR_DATA_TOO_LONG = errors.New("data length exceeds 6 bytes")
)

type CANMsg struct {
	ID   uint32 // node CID this is being issued for
	Cmd  uint16 // command being issued in this message
	Data []byte // raw data up to six bytes. DLC is taken from len(TXData).
}
