package canbus

import (
	"encoding/binary"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCANMsg_toByteArray(t *testing.T) {
	Convey("Standard frame format encodes correctly", t, func() {
		msg := &CANMsg{
			ID:  0x0003,
			Cmd: 0x0120,
		}
		buf := make([]byte, 16)
		msg.Data = []byte{1, 2, 3, 4, 5}
		raw, _ := msg.toByteArray()

		So(raw[0:3], ShouldResemble, []byte{0x23, 0x01, 0})
		So(raw[4], ShouldEqual, 5)
		So(raw[8:13], ShouldResemble, msg.Data)

		Convey("data length error is handled correctly", func() {
			var err error
			msg.Data = buf[:8]
			_, err = msg.toByteArray()
			So(err, ShouldBeNil)

			msg.Data = buf[:9]
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
		nodeMsgFromByteArray(raw)
	}
}
