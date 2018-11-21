package canbus

import (
	"encoding/binary"
)

const (
	msgMaxLength = 8

	CAN_BCM            = 0x2
	CAN_EFF_FLAG       = 0x80000000
	CAN_EFF_ID_BITS    = 0x1d
	CAN_EFF_MASK       = 0x1fffffff
	CAN_ERR_FLAG       = 0x20000000
	CAN_ERR_MASK       = 0x1fffffff
	CAN_INV_FILTER     = 0x20000000
	CAN_ISOTP          = 0x6
	CAN_MAX_DLC        = 0x8
	CAN_MAX_DLEN       = 0x8
	CAN_MCNET          = 0x5
	CAN_MTU            = 0x10
	CAN_NPROTO         = 0x7
	CAN_RAW            = 0x1
	CAN_RAW_FILTER_MAX = 0x200
	CAN_RTR_FLAG       = 0x40000000
	CAN_SFF_ID_BITS    = 0xb
	CAN_SFF_MASK       = 0x7ff
	CAN_TP16           = 0x3
	CAN_TP20           = 0x4
)

func (msg *CANMsg) toByteArray() (raw []byte, err error) {
	raw = make([]byte, 16)
	var data []byte

	oid := msg.ID
	if oid != oid&CAN_SFF_MASK {
		// process EFF frame, these put the command in data[1:2]
		oid = oid | CAN_EFF_FLAG

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
	if oid&CAN_EFF_FLAG != 0 {
		msg.ID = oid & CAN_EFF_MASK

		dataLength = dataLength - 2 // account for the command in data[1:2]
		msg.Cmd = binary.LittleEndian.Uint16(raw[8:10])
		msg.Data = raw[10 : 10+dataLength]
	} else {
		oid &= CAN_SFF_MASK
		if oid&canSFFNodeFlag != canSFFNodeFlag {
			return nil
		}
		msg.ID = oid & canIDMask
		msg.Cmd = uint16(oid & canCMDMask)

		msg.Data = raw[8 : 8+dataLength]
	}

	return msg
}
