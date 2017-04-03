package main

import (
	"bytes"
	"encoding/binary"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/tinylib/msgp/msgp"
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
	for i := range s.data {
		buf[i] = s.data[i]
		s.data[i]++
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

type MockControlI2C struct {
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

func (m *MockMotor) GetPosition() int {
	return 123
}

func (m *MockMotor) Home(_ int) {
	panic("MockMotor panic")
}

func (m *MockMotor) GetState() (state MotorState) {
	state.Target = m.target
	state.Current = m.target
	return
}

func (c *MockControlI2C) Get(i2cAddr int, cmd uint16, buf []byte) {
	if i2cAddr != m_CONTROL_ADDRESS || cmd != m_CONTROL_REG {
		panic("Incorrect call to the control mcu")
	}
	if c.mcu.cmd == m_REG_RELATIVE && c.mcu.value <= c.trigger {
		binary.BigEndian.PutUint16(buf, c.base-c.control)
	} else {
		binary.BigEndian.PutUint16(buf, c.base)
	}
}

func (c *MockControlI2C) Put(i2cAddr int, cmd uint16, buf []byte) {
	panic("MockControlI2C does not implment Put")
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
		go sb.Update()              // start updater
		time.Sleep(time.Second * 1) // Wait some time
		start := s.GetValue(0, 0)
		time.Sleep(time.Second * 1) // Wait some time
		So(s.GetValue(0, 0), ShouldBeGreaterThan, start)
	})

	SkipConvey("set address send the correct data", t, func() {
		sb.address = 0x21
		msb.buf[0] = 0xFF
		msb.buf[1] = 0x20
		sb.changeAddress(0x22)
		So(msb.putAddr, ShouldEqual, 0x21)
		So(msb.putCmd, ShouldEqual, sb_REG_ADDR)
		So(msb.buf[0], ShouldEqual, 0x22)
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

	motor := NewRMCS220xMotor(
		mcu,
		control,
		2,
		0x16,
		-2550,
		2550,
		255,
		42,
	)

	Convey("constructor has worked", t, func() {
		So(mcu.i2cAddr, ShouldEqual, motor.address)
		So(mcu.cmd, ShouldEqual, m_REG_DAMPING)
		So(mcu.value, ShouldEqual, 42)
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
				mcu.cmd = m_REG_RELATIVE // home function uses relative movement, as should our mock version
				So(motor.readControl(), ShouldBeTrue)
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
			state := dynastat.readMotors()
			So(state, ShouldContainKey, "TestMotor")
		})

		Convey("read sensors contains our test sensor", func() {
			state := dynastat.readSensors()
			So(state, ShouldContainKey, "TestSensor")
			So(state["TestSensor"], ShouldHaveLength, sensor.rows)
			So(state["TestSensor"][0], ShouldHaveLength, sensor.cols)
		})

		Convey("get global state works as expected", func() {
			state := dynastat.GetState()
			So(state.Motors, ShouldContainKey, "TestMotor")
			So(state.Sensors, ShouldContainKey, "TestSensor")
		})
	})
}

func TestMarshalUnmarshalDynastatState(t *testing.T) {
	v := DynastatState{}
	bts, err := v.MarshalMsg(nil)
	if err != nil {
		t.Fatal(err)
	}
	left, err := v.UnmarshalMsg(bts)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Errorf("%d bytes left over after UnmarshalMsg(): %q", len(left), left)
	}

	left, err = msgp.Skip(bts)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Errorf("%d bytes left over after Skip(): %q", len(left), left)
	}
}

func BenchmarkMarshalMsgDynastatState(b *testing.B) {
	v := DynastatState{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.MarshalMsg(nil)
	}
}

func BenchmarkAppendMsgDynastatState(b *testing.B) {
	v := DynastatState{}
	bts := make([]byte, 0, v.Msgsize())
	bts, _ = v.MarshalMsg(bts[0:0])
	b.SetBytes(int64(len(bts)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bts, _ = v.MarshalMsg(bts[0:0])
	}
}

func BenchmarkUnmarshalDynastatState(b *testing.B) {
	v := DynastatState{}
	bts, _ := v.MarshalMsg(nil)
	b.ReportAllocs()
	b.SetBytes(int64(len(bts)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := v.UnmarshalMsg(bts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestEncodeDecodeDynastatState(t *testing.T) {
	v := DynastatState{}
	var buf bytes.Buffer
	msgp.Encode(&buf, &v)

	m := v.Msgsize()
	if buf.Len() > m {
		t.Logf("WARNING: Msgsize() for %v is inaccurate", v)
	}

	vn := DynastatState{}
	err := msgp.Decode(&buf, &vn)
	if err != nil {
		t.Error(err)
	}

	buf.Reset()
	msgp.Encode(&buf, &v)
	err = msgp.NewReader(&buf).Skip()
	if err != nil {
		t.Error(err)
	}
}

func BenchmarkEncodeDynastatState(b *testing.B) {
	v := DynastatState{}
	var buf bytes.Buffer
	msgp.Encode(&buf, &v)
	b.SetBytes(int64(buf.Len()))
	en := msgp.NewWriter(msgp.Nowhere)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.EncodeMsg(en)
	}
	en.Flush()
}

func BenchmarkDecodeDynastatState(b *testing.B) {
	v := DynastatState{}
	var buf bytes.Buffer
	msgp.Encode(&buf, &v)
	b.SetBytes(int64(buf.Len()))
	rd := msgp.NewEndlessReader(buf.Bytes(), b)
	dc := msgp.NewReader(rd)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := v.DecodeMsg(dc)
		if err != nil {
			b.Fatal(err)
		}
	}
}
