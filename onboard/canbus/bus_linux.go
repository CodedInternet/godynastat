package canbus

/*
#include <linux/can/raw.h>
*/
import "C"

import (
	"golang.org/x/sys/unix"
	"net"
)

type CANBus struct {
	fd   int
	tx   chan []byte
	Rx   map[uint32]chan CANMsg
	open bool
}

func NewCANBus(ifname string) (bus *CANBus, err error) {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return
	}

	bus = new(CANBus)

	bus.fd, err = unix.Socket(unix.AF_CAN, unix.SOCK_RAW, unix.CAN_RAW)
	if err != nil {
		return
	}
	//unix.SetsockoptInt(bus.fd, C.SOL_CAN_RAW, C.CAN_RAW_LOOPBACK, 0)
	addr := &unix.SockaddrCAN{Ifindex: iface.Index}
	unix.Bind(bus.fd, addr)

	bus.Rx = make(map[uint32]chan CANMsg)
	bus.tx = make(chan []byte)

	bus.open = true
	go bus.reader()
	go bus.writer()

	return
}

func (c *CANBus) AddListener(nodeId uint32, rxchan chan CANMsg) {
	c.Rx[nodeId] = rxchan
}

func (c *CANBus) SendMsg(msg CANMsg) error {
	raw, err := msg.toByteArray()
	if err != nil {
		return err
	}
	c.tx <- raw
	return nil
}

func (c *CANBus) writer() {
	for c.open {
		msg := <-c.tx
		unix.Write(c.fd, msg)
	}
}

func (c *CANBus) reader() {
	for c.open {
		raw := make([]byte, 16)
		unix.Read(c.fd, raw)
		msg := nodeMsgFromByteArray(raw)

		if msg != nil {
			c.Rx[msg.ID] <- *msg
		}
	}
}
