package canbus

import (
	"encoding/binary"
	"golang.org/x/sys/unix"
)

const (
	msgMaxLength = 8
)

func (msg *CANMsg) toByteArray() (raw []byte, err error) {
	raw = make([]byte, 16)
	var data []byte

	oid := msg.ID
	if oid != oid&unix.CAN_SFF_MASK {
		// process EFF frame, these put the command in data[1:2]
		oid = oid | unix.CAN_EFF_FLAG

		binary.LittleEndian.PutUint16(data, msg.Cmd)
		data = append(data, msg.Data...)
	} else {
		oid |= uint32(msg.Cmd)
		data = msg.Data
	}

	binary.LittleEndian.PutUint32(raw[0:4], oid)

	// check and assign length to DLC
	if len(msg.Data) > msgMaxLength {
		return nil, ERR_DATA_TOO_LONG
	}
	raw[4] = byte(len(data))

	// copy the raw command data
	copy(raw[8:], data)

	return
}

func nodeMsgFromByteArray(raw []byte) (msg *CANMsg) {
	msg = new(CANMsg)

	oid := binary.LittleEndian.Uint32(raw[0:4])

	dataLength := raw[4]

	// determine CID
	if oid&unix.CAN_EFF_FLAG != 0 {
		msg.ID = oid & unix.CAN_EFF_MASK

		dataLength = dataLength - 2 // account for the command in data[1:2]
		msg.Cmd = binary.LittleEndian.Uint16(raw[8:10])
		msg.Data = raw[10 : 10+dataLength]
	} else {
		oid &= unix.CAN_SFF_MASK
		if oid&canSFFNodeFlag != canSFFNodeFlag {
			return nil
		}
		msg.ID = oid & canIDMask
		msg.Cmd = uint16(oid & canCMDMask)

		msg.Data = raw[8 : 8+dataLength]
	}

	return msg
}
