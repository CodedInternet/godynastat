package hardware

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCMDSetPos(t *testing.T) {
	Convey("command converts to a byte array correctly", t, func() {
		cmd := &CMDSetPos{
			12,
			34,
			56,
		}

		data := cmd.TXData()
		So(data, ShouldHaveLength, 3)
		So(data[0], ShouldEqual, 12)
		So(data[1], ShouldEqual, 34)
		So(data[2], ShouldEqual, 56)
	})
}
