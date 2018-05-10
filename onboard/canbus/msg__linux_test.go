package canbus

import (
	"encoding/binary"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCANMsg_toByteArray(t *testing.T) {
	Convey("Standard frame format encodes correctly", t, func() {
		msg := &CANMsg{
			ID:  0x123,
			Cmd: 0x4567,
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint32(buf, 0x1234)
		msg.Data = buf[:2] // need to do this manually so the length gets correctly set
		raw, _ := msg.toByteArray()

		Convey("ID gets set correctly", func() {
			So(raw[0:4], ShouldResemble, []byte{0x23, 0x01, 0x00, 0x00})
		})

		Convey("Data length is correctly set", func() {
			So(raw[4], ShouldEqual, 2)
		})

		Convey("Cmd is correctly set", func() {
			So(raw[8:10], ShouldResemble, []byte{0x67, 0x45})
		})

		Convey("Data is copied over", func() {
			So(raw[10:], ShouldResemble, []byte{0x34, 0x12, 0x00, 0x00, 0x00, 0x00})
		})

		Convey("data length error is handled correctly", func() {
			var err error
			msg.Data = buf[:2]
			_, err = msg.toByteArray()
			So(err, ShouldBeNil)

			msg.Data = buf[:8]
			_, err = msg.toByteArray()
			So(err, ShouldEqual, ERR_DATA_TOO_LONG)
		})
	})
}

func BenchmarkCANMsg_toByteArray(b *testing.B) {
	msg := &CANMsg{
		ID:   0x7ff,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint32(msg.Data, 0x0001)

	for n := 0; n < b.N; n++ {
		msg.toByteArray()
	}
}

func BenchmarkCANMsg_msgFromByteArray(b *testing.B) {
	msg := &CANMsg{
		ID:   0x7ff,
		Data: make([]byte, 6),
	}
	buf := make([]byte, 6)
	binary.LittleEndian.PutUint32(buf, 0x0001)
	msg.Data = buf[:6]
	raw, _ := msg.toByteArray()

	for n := 0; n < b.N; n++ {
		msgFromByteArray(raw)
	}
}
