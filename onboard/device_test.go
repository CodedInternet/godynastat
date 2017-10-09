package onboard

import (
	"encoding/binary"
	. "github.com/smartystreets/goconvey/convey"
	"sync"
	"testing"
	"time"
)

const (
	kScaleTolerance = 0.1
	kMotorTolerance = 25
)

type MockI2CSensorBoard struct {
	data    []byte
	putAddr int
	putCmd  uint16
	buf     []byte
}

func (s *MockI2CSensorBoard) Get(i2cAddr int, cmd uint16, buf []byte) {
	for i := range buf {
		buf[i] = s.data[i]
	}
}

func (s *MockI2CSensorBoard) Put(i2cAddr int, cmd uint16, buf []byte) {
	s.putAddr = i2cAddr
	s.putCmd = cmd
	s.buf = buf
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

func (b *MockUARTMCU) Get(i2cAddr int, cmd uint8) (value int32, err error) {
	b.i2cAddr = i2cAddr
	b.cmd = cmd
	return b.value, nil
}

func (b *MockUARTMCU) connected(i2cAddr int) bool {
	if i2cAddr == 0x42 {
		return false
	}
	return true
}

type MockSwitchMCU struct {
	mcu     *MockUARTMCU
	trigger int32
	control uint16
	base    uint16
}

type MockMotor struct {
	target int
}

func (m *MockMotor) SetTarget(target int) {
	m.target = target
}

func (m *MockMotor) GetPosition() (int, error) {
	return 123, nil
}

func (m *MockMotor) Home(_ int) error {
	panic("MockMotor does not implement Home")
}

func (m *MockMotor) findHome(_ bool) error {
	panic("MockMotor does not implement findHome")
}

func (m *MockMotor) GetState() (state MotorState, err error) {
	state.Target = m.target
	state.Current = m.target
	return
}

func (m *MockMotor) getRaw(_ uint8) (_ int, _ error) {
	panic("MockMotor does not implement raw getters and setters")
}

func (m *MockMotor) putRaw(_ uint8, _ int) {
	panic("MockMotor does not implement raw getters and setters")
}

func (c *MockSwitchMCU) Get(i2cAddr int, cmd uint16, buf []byte) {
	if i2cAddr != sm_ADDRESS || !(cmd == sm_REG_VALUES || cmd == sm_REG_ID) {
		panic("Incorrect call to the control mcu")
	}

	if cmd == sm_REG_ID {
		binary.LittleEndian.PutUint16(buf, sm_KNOWN_ID)
	} else {
		if c.mcu.cmd == m_REG_RELATIVE && c.mcu.value <= c.trigger {
			binary.LittleEndian.PutUint16(buf, c.base-(1<<(c.control)-1))
		} else {
			binary.LittleEndian.PutUint16(buf, c.base)
		}
	}
}

func (c *MockSwitchMCU) Put(i2cAddr int, cmd uint16, buf []byte) {
	panic("MockSwitchMCU does not implement Put")
}

func TestSensorBoard(t *testing.T) {
	msb := &MockI2CSensorBoard{
		data: make([]byte, sb_ROWS*sb_COLS*2),
		buf:  make([]byte, 1),
	}
	sb := &SensorBoard{
		msb,
		0,
		make([]byte, sb_COLS*sb_ROWS*2),
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
			sb.buf[391] = 0x88
			sb.buf[392] = 0xff
			sb.buf[393] = 0xff
			sb.buf[394] = 0x88
			So(s.GetValue(8, 4), ShouldAlmostEqual, 255, 1)
		})

		Convey("deliberately out of bounds", func() {
			So(func() { s.GetValue(sb_ROWS+1, sb_COLS+1) }, ShouldPanic)
		})
	})

	Convey("Updater fetches new data", t, func() {
		quit := make(chan bool)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			ticker := time.NewTicker(time.Second / 2)
			for {
				select {
				case <-quit:
					ticker.Stop()
					wg.Done()
					break
				case <-ticker.C:
					for i := range msb.data {
						msb.data[i]++
					}
				}
			}
		}()

		go sb.Update()              // start updater
		time.Sleep(time.Second * 1) // Wait some time
		start := s.GetValue(0, 0)
		time.Sleep(time.Second * 1) // Wait some time
		So(s.GetValue(0, 0), ShouldBeGreaterThan, start)

		// tidy up update goroutine
		quit <- true
		wg.Wait()
	})

	Convey("set address sends the correct data", t, func() {
		sb.address = 0x21
		msb.data[0] = 0x21
		sb.changeAddress(0x22)
		So(msb.putAddr, ShouldEqual, 0x21)
		So(msb.putCmd, ShouldEqual, sb_REG_ADDR)
		So(msb.buf[0], ShouldEqual, 0x22)

		Convey("New Address is out of range", func() {
			var err error
			msb.buf[0] = 0x12
			err = sb.changeAddress(-0x01)
			So(err, ShouldNotBeNil)
			So(msb.buf[0], ShouldEqual, 0x12)
			err = sb.changeAddress(0x80)
			So(err, ShouldNotBeNil)
			So(msb.buf[0], ShouldEqual, 0x12)
		})

		Convey("Current address from board doesn't match stored address", func() {
			var err error
			sb.address = 0x21
			msb.data[0] = 0x22
			msb.buf[0] = 0x12
			err = sb.changeAddress(0x34)
			So(err, ShouldNotBeNil)
			So(msb.buf[0], ShouldEqual, 0x12)
		})
	})

	Convey("Setting mode sends the correct data", t, func() {
		sb.address = 0x21
		msb.data[0] = 0x12
		sb.SetMode(0x12)
		So(msb.putAddr, ShouldEqual, sb.address)
		So(msb.putCmd, ShouldEqual, sb_REG_MODE)
		So(msb.buf[0], ShouldEqual, 0x12)

		Convey("Should panic without the readback", func() {
			So(func() { sb.SetMode(0x13) }, ShouldPanic)
		})
	})

	Convey("NewSensor constructor handles unknown reg mode", t, func() {
		_, err := NewSensor(sb, 0, false, sb_ROWS, s_BANK1_COLS, 0, 127, 255)
		So(err, ShouldNotBeNil)
	})
}

func TestRMCS220xMotor(t *testing.T) {
	mcu := &MockUARTMCU{}
	control := &MockSwitchMCU{
		mcu,
		5000,
		2,
		0xffff,
	}
	switches, err := NewSwitchMCU(control, sm_ADDRESS)
	if err != nil {
		panic(err) // should be impossible in a test
	}

	motor := NewRMCS220xMotor(
		mcu,
		switches,
		2,
		0x16,
		-3000,
		2550,
		255,
		42,
	)

	Convey("constructor has worked", t, func() {
		So(mcu.i2cAddr, ShouldEqual, motor.address)
		So(mcu.cmd, ShouldEqual, m_REG_DAMPING)
		So(mcu.value, ShouldEqual, 42)
		So(motor.target, ShouldEqual, 138) // should be a raw value of 0
	})

	Convey("basic operations", t, func() {
		Convey("write position", func() {
			motor.writePosition(123)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_GOTO)
			So(mcu.value, ShouldEqual, 123)
		})

		Convey("read position", func() {
			mcu.value = 456
			pos, _ := motor.readPosition()
			So(pos, ShouldEqual, 456)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_POSITION)
		})

		Convey("raw commands", func() {
			Convey("raw put", func() {
				mcu.i2cAddr = -1
				mcu.cmd = 255
				mcu.value = -1
				motor.putRaw(m_REG_POSITION, 784)
				So(mcu.i2cAddr, ShouldEqual, motor.address)
				So(mcu.cmd, ShouldEqual, m_REG_POSITION)
				So(mcu.value, ShouldEqual, 784)
			})

			Convey("raw get", func() {
				mcu.i2cAddr = -1
				mcu.cmd = 255
				mcu.value = 475
				val, _ := motor.getRaw(m_REG_POSITION)
				So(mcu.i2cAddr, ShouldEqual, motor.address)
				So(mcu.cmd, ShouldEqual, m_REG_POSITION)
				So(val, ShouldEqual, 475)
			})
		})
	})

	Convey("more advanced options with scaling", t, func() {
		Convey("setting Target position", func() {
			motor.SetTarget(0)
			So(motor.target, ShouldEqual, 0)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_GOTO)
			So(mcu.value, ShouldAlmostEqual, motor.rawLow, kMotorTolerance)

			motor.SetTarget(255)
			So(mcu.value, ShouldAlmostEqual, motor.rawHigh, kMotorTolerance)
		})

		Convey("getting Current position", func() {
			mcu.value = int32(motor.rawLow)
			pos, _ := motor.GetPosition()
			So(pos, ShouldEqual, 0)
			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_POSITION)

			mcu.value = int32(motor.rawHigh)
			pos, _ = motor.GetPosition()
			So(pos, ShouldBeBetweenOrEqual, 255-1, 255+1)
		})

		SkipConvey("get position when motor drifts out of bounds", func() {
			mcu.value = int32(motor.rawLow * 2)
			pos, _ := motor.GetPosition()
			So(pos, ShouldBeBetweenOrEqual, -255-1, -255+1)

			So(mcu.i2cAddr, ShouldEqual, motor.address)
			So(mcu.cmd, ShouldEqual, m_REG_POSITION)

			mcu.value = int32(motor.rawHigh * 2)
			pos, _ = motor.GetPosition()
			So(pos, ShouldBeBetweenOrEqual, 512-1, 512+1)

		})

		Convey("current state returns the expected values", func() {
			mcu.value = int32(motor.rawHigh)
			motor.target = 123
			state, err := motor.GetState()
			So(err, ShouldBeNil)
			So(state.Target, ShouldEqual, 123)
			So(state.Current, ShouldBeBetweenOrEqual, 255-1, 255+1)
		})

		Convey("test inverse scaling", func() {
			inv := *motor
			inv.rawLow = 3000
			inv.rawHigh = -2550
			var pos int

			mcu.value = int32(inv.rawHigh)
			pos, _ = inv.GetPosition()
			So(pos, ShouldBeBetweenOrEqual, 255-1, 255+1)

			mcu.value = int32(inv.rawLow)
			pos, _ = inv.GetPosition()
			So(pos, ShouldEqual, 0)

			mcu.value = int32(inv.rawLow)
			pos, _ = inv.GetPosition()
			So(pos, ShouldEqual, 0)
		})
	})

	SkipConvey("test homing", t, func() {
		// reset all values
		motor.SetTarget(0)
		control.trigger = 600
		mcu.cmd = 0
		mcu.value = 0

		go motor.Home(int(control.trigger))
		time.Sleep(time.Millisecond) // let it get started
		// check we are issuing relative move commands
		SkipSo(mcu.cmd, ShouldEqual, m_REG_RELATIVE)
		SkipSo(mcu.value, ShouldEqual, 555)

		// mock the trigger being activated by moving the control value
		control.trigger = 555
		time.Sleep(time.Millisecond * 50)
		// assert we are back at at the raw 0 point
		SkipSo(mcu.cmd, ShouldEqual, m_REG_GOTO)
		SkipSo(mcu.value, ShouldEqual, 0)

		Convey("test in reverse", t, func() {
			// reset all values
			motor.SetTarget(255)
			control.trigger = -600
			mcu.cmd = 0
			mcu.value = 0

			go motor.Home(int(control.trigger))
			time.Sleep(time.Millisecond) // let it get started
			// check we are issuing relative move commands
			So(mcu.cmd, ShouldEqual, m_REG_RELATIVE)
			So(mcu.value, ShouldEqual, -555)

			// mock the trigger being activated by moving the control value
			control.trigger = -555
			time.Sleep(time.Millisecond * 50)
			// assert we are back at the raw 0 point
			So(mcu.cmd, ShouldEqual, m_REG_GOTO)
			So(mcu.value, ShouldEqual, 0)
		})
	})
}

func TestDynastat(t *testing.T) {
	motor := new(MockMotor)
	sb := &SensorBoard{
		&MockI2CSensorBoard{
			data: make([]byte, sb_ROWS*sb_COLS*2),
		},
		0,
		make([]byte, sb_COLS*sb_ROWS*2),
	}
	sensor, _ := NewSensor(sb, 2, true, 2, 4, 0, 127, 255)
	dynastat := new(Dynastat)
	dynastat.Motors = make(map[string]MotorInterface, 1)
	dynastat.Motors["TestMotor"] = motor
	dynastat.sensors = make(map[string]SensorInterface, 1)
	dynastat.sensors["TestSensor"] = sensor

	Convey("Setting a motor value works", t, func() {
		Convey("known motor works as expected", func() {
			dynastat.SetMotor("TestMotor", 42)
			So(motor.target, ShouldEqual, 42)
		})

		Convey("unkown motor returns appropriate error", func() {
			err := dynastat.SetMotor("whoami", 123)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "whoami")
		})
	})

	Convey("get states works as expected", t, func() {
		Convey("get Motors contains our test motor", func() {
			state, _ := dynastat.readMotors()
			So(state, ShouldContainKey, "TestMotor")
		})

		Convey("read sensors contains our test sensor", func() {
			state := dynastat.readSensors()
			So(state, ShouldContainKey, "TestSensor")
			So(state["TestSensor"], ShouldHaveLength, sensor.rows)
			So(state["TestSensor"][0], ShouldHaveLength, sensor.cols)
		})

		Convey("get global state works as expected", func() {
			state, _ := dynastat.GetState()
			So(state.Motors, ShouldContainKey, "TestMotor")
			So(state.Sensors, ShouldContainKey, "TestSensor")
		})
	})
}
