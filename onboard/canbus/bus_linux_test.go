package canbus

import (
	"testing"
)

func BenchmarkCANBusResetTime(b *testing.B) {
	bus, _ := NewCANBus("can0")

	rxc := make(chan CANMsg)
	bus.AddListener(0, rxc)

	tx := &CANMsg{
		ID:  0x7ff,
		Cmd: 0x0020,
	}

	for n := 0; n < b.N; n++ {
		raw, _ := tx.toByteArray()
		bus.tx <- raw

		<-rxc
	}
}
