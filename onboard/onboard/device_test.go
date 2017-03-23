package onboard

import (
	"encoding/binary"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

const (
	kScaleTolerance = 0.1
	kMotorTolerance = 25
)

type MockI2CSensorBoard struct {
	data []byte
}

func (s *MockI2CSensorBoard) Get(i2cAddr int, cmd uint16, buf []byte) {
	for i := range s.data {
		buf[i] = s.data[i]
		s.data[i]++
	}
}

type MockUARTMCU struct {
	i2cAddr int
	cmd     uint8
	value   int32
}

func (b *MockUARTMCU) Put(i2cAddr int, cmd uint8, value int32) {
	b.i2cAddr = i2cAddr
	b.cmd = cmd
	b.value = value
}
func (b *MockUARTMCU) Get(i2cAddr int, cmd uint8) (value int32) {
	b.i2cAddr = i2cAddr
	b.cmd = cmd
	return b.value
}

type MockControlI2C struct {
	mcu     *MockUARTMCU
	trigger int32
	control uint16
	base    uint16
}

func (c *MockControlI2C) Get(i2cAddr int, cmd uint16, buf []byte) {
	if i2cAddr != m_CONTROL_ADDRESS || cmd != m_CONTROL_REG {
		panic("Incorrect call to the control mcu")
	}
	if c.mcu.cmd == m_REG_GOTO && c.mcu.value <= c.trigger {
		binary.LittleEndian.PutUint16(buf, c.base-c.control)
	} else {
		binary.LittleEndian.PutUint16(buf, c.base)
	}
}

func TestSensorBoard(t *testing.T) {
	sb := &SensorBoard{
		&MockI2CSensorBoard{
			data: make([]byte, sb_ROWS*sb_COLS),
		},
		0,
		make([]byte, sb_COLS*sb_ROWS),
	}
	s, _ := NewSensor(sb, 1, false, sb_ROWS, s_BANK1_COLS, 0, 127, 255)

	Convey("Setting the scale works as expected", t, func() {
		Convey("1:1 scaling", func() {
			s.SetScale(0, 127, 255)
			So(s.scaleFactor, ShouldAlmostEqual, 1, kScaleTolerance)
		})

		Convey("1:2 scaling", func() {
			s.SetScale(0, 255, 511)
			So(s.scaleFactor, ShouldAlmostEqual, 2, kScaleTolerance)
		})

		Convey("1:1 scaling with non zero start point", func() {
			s.SetScale(10, 137, 265)
			So(s.scaleFactor, ShouldAlmostEqual, 1, kScaleTolerance)
		})

		Convey("Some larger realistic values", func() {
			s.SetScale(24, 36213, 64536)
			So(s.scaleFactor, ShouldAlmostEqual, 268.5, kScaleTolerance)
		})
	})

	Convey("Getting value works as expectd", t, func() {
		// ensure the scale is as expected
		s.SetScale(0, 32768, 65535)

		Convey("first two bytes", func() {
			sb.buf[0] = 0x80
			sb.buf[1] = 0x00
			sb.buf[2] = 0xff
			So(s.GetValue(0, 0), ShouldEqual, 127)
		})

		Convey("somewhere in the middle of the array", func() {
			sb.buf[195] = 0x88
			sb.buf[196] = 0xff
			sb.buf[197] = 0xff
			sb.buf[198] = 0x88
			So(s.GetValue(8, 4), ShouldAlmostEqual, 255, 1)
		})

		Convey("deliberately out of bounds", func() {
			So(func() { s.GetValue(sb_ROWS+1, sb_COLS+1) }, ShouldPanic)
		})
	})

	Convey("Updater fetches new data", t, func() {
		go sb.Update()              // start updater
		time.Sleep(time.Second * 1) // Wait some time
		start := s.GetValue(0, 0)
		time.Sleep(time.Second * 1) // Wait some time
		So(s.GetValue(0, 0), ShouldBeGreaterThan, start)
	})
}

func TestRMCS220xMotor(t *testing.T) {
	mcu := &MockUARTMCU{}
	control := &MockControlI2C{
		mcu,
		1000,
		2,
		0xffff,
	}
	motor := RMCS220xMotor{
		mcu,
		control,
		0x16,
		2,
		-2550,
		2550,
		0,
	}

	Convey("basic operations", t, func() {
		Convey("write position", func() {
			motor.writePosition(123)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_GOTO)
			So(mcu.value, ShouldEqual, 123)
		})

		Convey("read position", func() {
			mcu.value = 456
			So(motor.readPosition(), ShouldEqual, 456)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_POSITION)
		})

		Convey("reading the control pin", func() {
			Convey("not at home", func() {
				mcu.value = 999
				So(motor.readControl(), ShouldBeFalse)
			})

			Convey("past home position", func() {
				mcu.value = 1000
				mcu.cmd = m_REG_GOTO
				So(motor.readControl(), ShouldBeTrue)
			})
		})
	})

	Convey("more advanced options with scaling", t, func() {
		Convey("setting target position", func() {
			motor.SetTarget(0)
			So(motor.target, ShouldEqual, 0)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_GOTO)
			So(mcu.value, ShouldAlmostEqual, motor.rawLow, kMotorTolerance)

			motor.SetTarget(255)
			So(mcu.value, ShouldAlmostEqual, motor.rawHigh, kMotorTolerance)
		})

		Convey("getting current position", func() {
			mcu.value = int32(motor.rawLow)
			So(motor.GetPosition(), ShouldEqual, 0)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_POSITION)

			mcu.value = int32(motor.rawHigh)
			So(motor.GetPosition(), ShouldEqual, 255)
		})

		Convey("get position when motor drifts out of bounds", func() {
			mcu.value = int32(motor.rawLow * 2)
			So(motor.GetPosition(), ShouldEqual, 0)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_POSITION)

			mcu.value = int32(motor.rawHigh * 2)
			So(motor.GetPosition(), ShouldEqual, 255)
		})
	})

	Convey("test homing", t, func() {
		go motor.Home(0)
		So(motor.GetPosition(), ShouldBeGreaterThan, 0) // difficult to actually test so just check it is moving
		time.Sleep(time.Second)                         // should be plenty of time
		So(motor.GetPosition(), ShouldEqual, 128)       // we can confirm it has finished because it is at 0
	})
}
