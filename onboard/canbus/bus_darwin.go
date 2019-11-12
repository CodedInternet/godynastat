package canbus

import "C"

import (
	"fmt"
)

type CANBus struct {
	fd   int
	tx   chan []byte
	Rx   map[uint32]chan CANMsg
	open bool
}

func NewCANBus(ifname string) (bus *CANBus, err error) {
	bus = new(CANBus)

	bus.Rx = make(map[uint32]chan CANMsg)
	bus.tx = make(chan []byte)

	bus.open = true
	//go bus.reader()
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
		fmt.Printf("writing: %s", msg)
		// echo back
		resp := nodeMsgFromByteArray(msg)
		if resp != nil {
			c.Rx[resp.ID] <- *resp
		}
	}
}

func (c *CANBus) reader() {
	for c.open {
		raw := make([]byte, 16)
		msg := nodeMsgFromByteArray(raw)

		if msg != nil {
			c.Rx[msg.ID] <- *msg
		}
	}
}
