package hardware

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCMDSetPos(t *testing.T) {
	Convey("command converts to a byte array correctly", t, func() {
		cmd := &CMDSetPos{[4]uint16{
			0x0102,
			0x0304,
			0x0405,
			0x0607,
		}}

		data := cmd.TXData()
		So(data, ShouldHaveLength, 8)
		for i := 0; i < 8; i++ {
			ShouldEqual(data[i], i)
		}
	})
}

func TestCMDSetSpeed(t *testing.T) {
	Convey("command converts to a byte array correctly", t, func() {
		cmd := &CMDSetSpeed{[4]uint8{
			1,
			2,
			3,
			4,
		}}

		data := cmd.TXData()
		So(data, ShouldHaveLength, 4)
		for i := range data {
			ShouldEqual(data[i], i)
		}
	})
}
