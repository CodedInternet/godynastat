package canbus

import (
	"encoding/binary"
	"golang.org/x/sys/unix"
)

const (
	host_mask = 0x2000
	id_mask   = 0x1234
	cmd_mask  = 0x5678
)

type CANMsg struct {
	ID   uint32
	Cmd  uint32
	Data []byte
}

func (msg *CANMsg) ToByteArray() (frame []byte) {
	frame = make([]byte, 16)

	oid := msg.ID
	if oid != oid&unix.CAN_SFF_MASK {
		oid = oid & unix.CAN_EFF_FLAG
	}

	binary.LittleEndian.PutUint32(frame[0:4], oid)
	frame[4] = byte(len(msg.Data))
	copy(frame[8:], msg.Data)

	return
}

func MsgFromByteArray(raw []byte) CANMsg {
	var msg CANMsg

	oid := binary.LittleEndian.Uint32(raw[0:4])

	// determine ID
	if oid&unix.CAN_EFF_FLAG != 0 {
		msg.ID = oid & unix.CAN_EFF_MASK
	} else {
		msg.ID = oid & unix.CAN_SFF_MASK
	}

	dlc := raw[4]
	msg.Data = raw[8 : 8+dlc]

	return msg
}
