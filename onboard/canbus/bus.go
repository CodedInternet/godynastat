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
	Tx   chan []byte
	Rx   map[uint32]chan []byte
	open bool
}

func NewCANBus(ifname string) (bus CANBus, err error) {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return
	}

	bus.fd, err = unix.Socket(unix.AF_CAN, unix.SOCK_RAW, unix.CAN_RAW)
	if err != nil {
		return
	}
	unix.SetsockoptInt(bus.fd, C.SOL_CAN_RAW, C.CAN_RAW_LOOPBACK, 0)
	addr := &unix.SockaddrCAN{Ifindex: iface.Index}
	unix.Bind(bus.fd, addr)

	bus.Rx = make(map[uint32]chan []byte)
	bus.Tx = make(chan []byte)

	bus.open = true
	go bus.reader()
	go bus.writer()

	return
}

func (c *CANBus) AddListener(nodeId uint32, rxchan chan []byte) {
	c.Rx[nodeId] = rxchan
}

func (c *CANBus) writer() {
	for c.open {
		msg := <-c.Tx
		unix.Write(c.fd, msg)
	}
}

func (c *CANBus) reader() {
	for c.open {
		raw := make([]byte, 16)
		unix.Read(c.fd, raw)
		//msg := msgFromByteArray(raw)

		c.Rx[0] <- raw
	}
}
