package hardware

import (
	"github.com/CodedInternet/godynastat/onboard/canbus"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestBaseCommand(t *testing.T) {
	tBus, tNode := createTestNodeBus()

	Convey("without sending abort errors", t, func() {
		cmd := &BaseCommand{}
		err := cmd.Abort()
		So(err, ShouldNotBeNil)
	})

	Convey("SendAndACK tries multiple times before timing out", t, func() {
		cmd := &BaseCommand{
			node: tNode,
			msg: canbus.CANMsg{
				ID: tNode.id,
			},
		}
		tBus.txCount = 0
		_, err := cmd.Process()
		So(err, ShouldEqual, ERR_MAX_RETRIES)
		So(tBus.txCount, ShouldEqual, CMD_MAX_RETRIES)

		Convey("aborting returns correct error and does not send till max", func() {
			// need to create the channel manually else Abort will error
			cmd.abort = make(chan struct{})
			go cmd.Abort() // trigger the abort first, this should result in it attempting to recover it
			tBus.txCount = 0
			_, err := cmd.Process()
			So(err, ShouldEqual, ERR_SEND_ABORT)
			So(tBus.txCount, ShouldBeLessThan, CMD_MAX_RETRIES)
		})

		Convey("successful send with ACK writes without an err", func() {
			tBus.rxecho = true
			resp, err := cmd.Process()
			So(err, ShouldBeNil)
			So(resp.ID, ShouldEqual, tNode.id)
			So(tBus.lastTx, ShouldResemble, cmd.msg)
		})
	})
}
