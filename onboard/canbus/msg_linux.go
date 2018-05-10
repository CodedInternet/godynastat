package canbus

import (
	"encoding/binary"
	"golang.org/x/sys/unix"
)

func (msg *CANMsg) toByteArray() (raw []byte, err error) {
	raw = make([]byte, 16)

	oid := msg.ID
	if oid != oid&unix.CAN_SFF_MASK {
		oid = oid & unix.CAN_EFF_FLAG
	}

	binary.LittleEndian.PutUint32(raw[0:4], oid)

	// check and assign length to DLC
	if len(msg.Data) > 6 {
		return nil, ERR_DATA_TOO_LONG
	}
	raw[4] = byte(len(msg.Data))

	// assign our command bytes
	binary.LittleEndian.PutUint16(raw[8:10], msg.Cmd)

	// copy the raw command data
	copy(raw[10:], msg.Data)

	return
}

func msgFromByteArray(raw []byte) CANMsg {
	var msg CANMsg

	oid := binary.LittleEndian.Uint32(raw[0:4])

	// determine ID
	if oid&unix.CAN_EFF_FLAG != 0 {
		msg.ID = oid & unix.CAN_EFF_MASK
	} else {
		msg.ID = oid & unix.CAN_SFF_MASK
	}

	dataLength := raw[4] - 2
	msg.Cmd = binary.LittleEndian.Uint16(raw[8:10])
	msg.Data = raw[10 : 10+dataLength]

	return msg
}
