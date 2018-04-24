package canbus

import (
	"encoding/binary"
	"testing"
)

func BenchmarkCANBusResetTime(b *testing.B) {
	bus, _ := NewCANBus("can0")

	rxc := make(chan []byte)
	bus.AddListener(0, rxc)

	tx := &CANMsg{
		ID:   0x7ff,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint32(tx.Data, 0x0001)

	for n := 0; n < b.N; n++ {
		bus.Tx <- tx.ToByteArray()

		<-rxc
	}
}
