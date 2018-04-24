package main

import (
	"encoding/binary"
	"fmt"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	"time"
)

func main() {
	fmt.Println("Opening listener on can0")
	bus, _ := canbus.NewCANBus("can0")

	rxc := make(chan []byte)
	bus.AddListener(0, rxc)

	go func(rxc chan []byte) {
		for {
			raw := <-rxc
			msg := canbus.MsgFromByteArray(raw)

			fmt.Printf("0x%04x \t[%d] \t", msg.ID, len(msg.Data))
			for i := 0; i < len(msg.Data); i++ {
				fmt.Printf("%02x ", msg.Data[i])
			}
			fmt.Printf("\n")
		}
	}(rxc)

	msg := &canbus.CANMsg{
		ID:   0x7ff,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint32(msg.Data, 0x0001)

	bus.Tx <- msg.ToByteArray()

	time.Sleep(1 * time.Second)
}
