package canbus

import (
	"encoding/binary"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCanMsg_ToByteArray(t *testing.T) {
	Convey("Standard frame format encodes correctly", t, func() {
		msg := &CANMsg{
			ID: 0x123,
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint32(buf, 0x1234)
		msg.Data = buf[:2] // need to do this manually so the length gets correctly set
		raw := msg.ToByteArray()

		Convey("ID gets set correctly", func() {
			So(raw[0:4], ShouldResemble, []byte{0x23, 0x01, 0x00, 0x00})
		})

		Convey("Data length is correctly set", func() {
			So(raw[4], ShouldEqual, 2)
		})

		Convey("Data is copied over", func() {
			So(raw[8:], ShouldResemble, []byte{0x34, 0x12, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		})
	})
}

func BenchmarkCanMsg_ToByteArray(b *testing.B) {
	msg := &CANMsg{
		ID:   0x7ff,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint32(msg.Data, 0x0001)

	for n := 0; n < b.N; n++ {
		msg.ToByteArray()
	}
}

func BenchmarkMsgFromByteArray(b *testing.B) {
	msg := &CANMsg{
		ID:   0x7ff,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint32(msg.Data, 0x0001)
	raw := msg.ToByteArray()

	for n := 0; n < b.N; n++ {
		MsgFromByteArray(raw)
	}
}
