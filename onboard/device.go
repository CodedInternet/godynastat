// Copyright 2017 Coded Internet Ltd. All rights reserved.

/*
	A set of features related directly to the interaction with the device hardware.
*/

package onboard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/jacobsa/go-serial/serial"
	"github.com/tinylib/msgp/msgp"
	"io"
	"math"
	"sync"
	"syscall"
	"time"
)

const (
	FRAMERATE = 20
	i2c_SLAVE = 0x0703

	sb_REG_VALUES = 0x0100
	sb_REG_ADDR   = 0x0004
	sb_REG_MODE   = 0x01
	sb_ROWS       = 16
	sb_COLS       = 24
	sb_BITS       = 8
	s_BANK1_COLS  = 16
	s_BANK2_COLS  = 8

	// Switch MCU
	sm_ADDRESS    = 0x20
	sm_REG_ID     = 0x0000
	sm_REG_VALUES = 0x0003
	sm_KNOWN_ID   = 0xFE00

	// Motor constants
	m_BITS          = 8
	m_REG_MAX_SPEED = 0
	m_REG_MANUAL    = 1
	m_REG_DAMPING   = 2
	m_REG_POSITION  = 3
	m_REG_GOTO      = 4
	m_REG_RELATIVE  = 8
)

type UARTMCU struct {
	port io.ReadWriteCloser
	lock sync.Mutex
}

type UARTMCUInterface interface {
	Put(i2cAddr int, cmd uint8, value int32)
	Get(i2cAddr int, cmd uint8) (value int32, err error)
}

type I2CBus struct {
	fd   int
	lock sync.Mutex
}

type I2CBusInterface interface {
	Get(i2cAddr int, reg uint16, buf []byte)
	Put(i2cAddr int, reg uint16, buf []byte)
}

type SensorBoard struct {
	i2cBus  I2CBusInterface
	address int
	buf     []byte
}

type Sensor struct {
	board        *SensorBoard
	zeroValue    uint16
	scaleFactor  float64
	mirror       bool
	rows, cols   int
	oRows, oCols int
}

type SensorState [][]uint8

type SensorInterface interface {
	SetScale(zero, half, full uint16)
	GetValue(row, col int) uint8
	GetState() SensorState
}

type SwitchMCU struct {
	address int
	bus     I2CBusInterface
}

type RMCS220xMotor struct {
	bus      UARTMCUInterface
	switches *SwitchMCU
	address  int
	control  uint16
	rawLow   int
	rawHigh  int
	target   int
}

type MotorState struct {
	Target, Current int
}

type MotorInterface interface {
	SetTarget(target int)
	GetPosition() (position int, err error)
	Home(calibrationValue int) error
	GetState() (state MotorState, err error)
	getRaw(reg uint8) (int, error)
	putRaw(reg uint8, val int)
	findHome(reverse bool) error
}

type Dynastat struct {
	Motors    map[string]MotorInterface
	sensors   map[string]SensorInterface
	SensorBus I2CBusInterface
	motorBus  UARTMCUInterface
	switches  *SwitchMCU
	config    *DynastatConfig
	lock      sync.Mutex
}

//go:generate msgp

type DynastatConfig struct {
	Version          int
	SignalingServers []string
	I2CBus           struct {
		Sensor int
	}
	UART struct {
		Motor string
	}
	Motors map[string]struct {
		Address        int
		Cal, Low, High int
		Speed, Damping int32
		Control        uint16
	}
	Sensors map[string]struct {
		Address                         int
		Mode                            uint8
		Registry                        uint
		Mirror                          bool
		Rows, Cols                      int
		ZeroValue, HalfValue, FullValue uint16
	}
}

type DynastatState struct {
	Motors  map[string]MotorState
	Sensors map[string]SensorState
}

type DynastatInterface interface {
	GetState() (DynastatState, error)
	GetConfig() *DynastatConfig
	SetMotor(name string, position int) (err error)
	HomeMotor(name string) error
	GotoMotorRaw(name string, position int) error
	WriteMotorRaw(name string, position int) error
	RecordMotorLow(name string) error
	RecordMotorHigh(name string) error
	RecordMotorHome(name string, reverse bool) (pos int, err error)
}

// Generic functions

// translateValue takes in a value and scales it from between left -> right.
func translateValue(val, leftMin, leftMax, rightMin, rightMax int) int {
	// Figure out how 'wide' each range is
	leftSpan := float64(leftMax - leftMin)
	rightSpan := float64(rightMax - rightMin)

	// Convert the left range into a 0-1 range (float)
	valueScaled := float64(val-leftMin) / leftSpan

	// Scale the 0-1 range backup and shift by the appropriate amount
	return rightMin + int(valueScaled*rightSpan)
}

// MCU

// OpenUARTMCU performs the necessary actions to open a new UART connection on the device.
// This sets up the UART port for propper communication with the hardware.
func OpenUARTMCU(ttyName string) *UARTMCU {
	port, err := serial.Open(serial.OpenOptions{
		PortName:              ttyName,
		BaudRate:              115200,
		DataBits:              8,
		StopBits:              1,
		InterCharacterTimeout: 200,
	})

	if err != nil {
		panic(fmt.Sprintf("Unable to open UART %s: %v", ttyName, err))
		return nil
	}
	mcu := new(UARTMCU)
	mcu.port = port
	return mcu
}

// Close is a proxy to the existing close method on the port
func (mcu *UARTMCU) Close() {
	mcu.port.Close()
}

// Put sends date out to the UARTMCU in the required format to act on a motor.
// More features may be added later.
func (mcu *UARTMCU) Put(i2cAddr int, cmd uint8, value int32) {
	buf := fmt.Sprintf("M%d %d %d\n", i2cAddr, cmd, value)

	// Keep as little processing outside the critical section as possible
	mcu.lock.Lock()
	defer mcu.lock.Unlock()
	mcu.port.Write([]byte(buf))
}

// Get returns values from the MCU on the specified registry
func (mcu *UARTMCU) Get(i2cAddr int, cmd uint8) (value int32, err error) {
	// Create buffers and format strings outside of cricial section
	wbuf := fmt.Sprintf("M%d %d\n", i2cAddr, cmd)
	rbuf := make([]byte, 18)

	// Perform read/write in critical section - keep to minimum to prevent excessive locking between threads
	mcu.lock.Lock()
	defer mcu.lock.Unlock()
	mcu.port.Write([]byte(wbuf))
	i, err := mcu.port.Read(rbuf)
	if i == 0 || err != nil {
		return 0, err
	}

	resp := string(rbuf)
	if resp == "ERROR NO RESPONSE" {
		return 0xEEEEEEE, errors.New("No response from motor")
	}

	fmt.Sscanf(string(rbuf), "%d", &value)
	return
}

// I2C related functions

// OpenI2C performs the actions necessary to create the I2CBus object.
// This opens the file and does very basic error checking on it
func OpenI2C(dev string) *I2CBus {
	fd, err := syscall.Open(dev, syscall.O_RDWR, 0777)
	if err != nil {
		panic(err)
	}

	return &I2CBus{fd: fd}
}

// ioctl proxy to appropriate syscall method.
// This is part of our own i2c library
func ioctl(fd, cmd, arg uintptr) (err error) {
	_, _, e1 := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, arg)
	if e1 != 0 {
		err = e1
	}
	return
}

// Connect send the commands to put the receiving device into slave mode so it can accept commands from the BBB
func (bus *I2CBus) Connect(i2cAddr int) {
	if err := ioctl(uintptr(bus.fd), i2c_SLAVE, uintptr(i2cAddr)); err != nil {
		panic(err)
	}
	return
}

// Get performs write/read to get a value from an I2C device in a 16bit registry space.
// Thread-safe.
func (bus *I2CBus) Get(i2cAddr int, reg uint16, buf []byte) {
	// perform bitbashing to get write command first
	wbuf := make([]byte, 2)
	wbuf[0] = byte(reg >> 8 & 0xff)
	wbuf[1] = byte(reg & 0xff)

	bus.lock.Lock()
	bus.Connect(i2cAddr)
	// Do write/read inside critical section
	syscall.Write(bus.fd, wbuf)
	syscall.Read(bus.fd, buf)
	bus.lock.Unlock()
}

// Put performs write to I2C device in a 16bit registry space.
// Thread-safe
func (bus *I2CBus) Put(i2cAddr int, reg uint16, buf []byte) {
	wbuf := make([]byte, len(buf)+2)
	wbuf[0] = byte(reg >> 8 & 0xff)
	wbuf[1] = byte(reg & 0xff)

	for i, b := range buf {
		wbuf[i+2] = b
	}

	bus.lock.Lock()
	bus.Connect(i2cAddr)
	syscall.Write(bus.fd, wbuf)
	bus.lock.Unlock()
}

// Sensor Boards

// Update routine to fetch new data from the board at the appropriate frame-rate
func (sb *SensorBoard) Update() {
	for {
		sb.i2cBus.Get(sb.address, sb_REG_VALUES, sb.buf)
		time.Sleep(time.Second / FRAMERATE)
	}
}

// Sets the mode of the sensor board in the firmware
func (sb *SensorBoard) SetMode(mode uint8) {
	buf := make([]byte, 1)
	buf[0] = mode
	sb.i2cBus.Put(sb.address, sb_REG_MODE, buf)
	sb.i2cBus.Get(sb.address, sb_REG_MODE, buf)
	if buf[0] != mode {
		panic(fmt.Sprintf("Setting sensor board mode has not worked 0x%x got %d expected %d", sb.address, buf[0], mode))
	}
}

// getValue returns a uint16 from the appropriate reg and +1 to ease with this process
func (sb *SensorBoard) getValue(reg int) uint16 {
	reg *= 2 // scale from 16bit to 8 bit
	return binary.BigEndian.Uint16([]byte{sb.buf[reg], sb.buf[reg+1]})
}

// changeAddress updates the address register on the sensor board.
// This update to the hardware requires a reboot of the sensor board.
func (sb *SensorBoard) changeAddress(newAddr int) error {
	if newAddr < 0x00 || newAddr > 0x7f {
		return errors.New(fmt.Sprintf("Invalid address: %x", newAddr))
	}

	// Read old address to sanity check
	buf := make([]byte, 1)
	sb.i2cBus.Get(sb.address, sb_REG_ADDR, buf)
	oldAddr := int(buf[0])

	if oldAddr != sb.address {
		return errors.New(fmt.Sprintf("Stored address %x does not match Current device %x", oldAddr, sb.address))
	}

	buf[0] = byte(newAddr)
	sb.i2cBus.Put(sb.address, sb_REG_ADDR, buf)

	return nil
}

// NewSensor provides an individual sensor on the given sensor board.
func NewSensor(board *SensorBoard, reg uint, mirror bool, rows, cols int,
	zeroValue, halfValue, fullValue uint16) (sensor *Sensor, err error) {

	sensor = new(Sensor)
	sensor.board = board
	sensor.mirror = mirror
	sensor.rows = rows
	sensor.cols = cols

	switch reg {
	case 1:
		sensor.oCols = (s_BANK1_COLS - cols) / 2
		break

	case 2:
		sensor.oCols = s_BANK1_COLS + (s_BANK2_COLS-cols)/2
		break

	default:
		return nil, errors.New("Unkown reg mode")
		break
	}

	sensor.oRows = (sb_ROWS - rows) / 2

	sensor.SetScale(zeroValue, halfValue, fullValue)
	return
}

// SetScale manually calculates the scale and updates it on the sensor
func (s *Sensor) SetScale(zero, half, full uint16) {
	var max, m1, m2 float64

	// Calculate the proportions of the scaling
	max = math.Pow(2, sb_BITS) - 1
	m1 = float64(half) / (max / 2)
	m2 = float64(full) / max

	// Assign struct values
	s.zeroValue = zero
	s.scaleFactor = (m1 + m2) / 2
}

// GetValue calculates the appropriate reg value then returns it from the board.
// Applies offsets if operating in two sensor mode.
func (s *Sensor) GetValue(row, col int) uint8 {
	if s.mirror {
		row = (s.rows - 1) - row
		col = (s.cols - 1) - col
	}

	row += s.oRows
	col += s.oCols

	i := row*sb_COLS + col
	val := s.board.getValue(i)
	return uint8(float64(val) / s.scaleFactor)
}

// GetState goes over all rows and cols on a sensor and gives values for this
func (s *Sensor) GetState() (state SensorState) {
	state = make(SensorState, s.rows)
	for i := 0; i < s.rows; i++ {
		state[i] = make([]uint8, s.cols)
		for j := 0; j < s.cols; j++ {
			state[i][j] = s.GetValue(i, j)
		}
	}
	return state
}

// SwitchMCU

// Creates a new object to reflect the onboard MCU that reads
func NewSwitchMCU(bus I2CBusInterface, address int) (mcu *SwitchMCU, err error) {
	// initialise object
	mcu = new(SwitchMCU)
	mcu.bus = bus
	mcu.address = address

	// check ID reads correctly
	buf := make([]byte, 2)
	mcu.bus.Get(address, sm_REG_ID, buf)
	val := binary.LittleEndian.Uint16(buf)
	if val != sm_KNOWN_ID {
		return nil, errors.New(fmt.Sprintf("Switch MCU not recognised. Expected ID %x recieved %x", sm_KNOWN_ID, val))
	}

	return
}

func (mcu *SwitchMCU) ReadInput(target uint16) (bool, error) {
	buf := make([]byte, 2)
	mcu.bus.Get(mcu.address, sm_REG_VALUES, buf)
	val := binary.LittleEndian.Uint16(buf)
	if val == 0 {
		return true, errors.New("Switch MCU reported value of 0")
	}
	return val&target == 0, nil
}

// RMCS220xMotor

// scalePos takes in a value and either scales it up to 16bit motor range or down to 0-255 application range.
func (m *RMCS220xMotor) scalePos(val int, up bool) int {
	max := int(math.Pow(2, float64(m_BITS)))
	if up {
		return translateValue(val, 0, max, m.rawLow, m.rawHigh)
	} else {
		val := translateValue(val, m.rawLow, m.rawHigh, 0, max)
		return val
	}
}

// writePosition performs the write to the motor.
func (m *RMCS220xMotor) writePosition(pos int32) {
	m.bus.Put(m.address, m_REG_GOTO, pos)
}

// readPosition gets the Current position from the motors encoder.
func (m *RMCS220xMotor) readPosition() (val int32, err error) {
	val, err = m.bus.Get(m.address, m_REG_POSITION)
	return
}

// SetTarget updates the Current Target in software and issues the write to the motor with the scaled value.
func (m *RMCS220xMotor) SetTarget(target int) {
	m.target = target
	m.writePosition(int32(m.scalePos(target, true)))
}

// GetPosition reads the position from motor and scales it to application range.
func (m *RMCS220xMotor) GetPosition() (val int, err error) {
	raw, err := m.readPosition()
	if err != nil {
		return
	}
	return m.scalePos(int(raw), false), err
}

// Home gradually moves the motor until it is pressing its home pin.
// To avoid crashes being potentially destructive to hardware, this in done in small increments so the motor will not
// continue unless the software deems it safe and reissues the move command.
func (m *RMCS220xMotor) Home(cal int) (err error) {
	err = m.findHome(cal < 0)
	if err != nil {
		return err
	}

	m.bus.Put(m.address, m_REG_POSITION, int32(cal))

	// Sleep to allow the motor to reset the PID to the new encoder position and allow the MCU time to catch up
	time.Sleep(time.Millisecond * 10)
	m.writePosition(0)
	return
}

// GetState provides information on the desired and Current position of the motor.
// This can be used to determine if the motor is currently at its Target or is in transit
func (m *RMCS220xMotor) GetState() (state MotorState, err error) {
	state.Target = m.target
	state.Current, err = m.GetPosition()
	return
}

func (m *RMCS220xMotor) getRaw(reg uint8) (int, error) {
	raw, err := m.bus.Get(m.address, reg)
	return int(raw), err
}

func (m *RMCS220xMotor) putRaw(reg uint8, val int) {
	m.bus.Put(m.address, reg, int32(val))
}

func (m *RMCS220xMotor) findHome(reverse bool) (err error) {
	if m.switches == nil {
		return errors.New("Control switches not found")
	}

	inc := int32(math.Abs(float64(m.rawHigh-m.rawLow))) / 10

	if reverse {
		// Invert increment so we go in the right direction
		inc = -inc
	}

	var home bool
	for ; !home && err == nil; home, err = m.switches.ReadInput(m.control) {
		m.bus.Put(m.address, m_REG_RELATIVE, inc)
		time.Sleep(time.Millisecond * 5)
	}
	// all stop
	m.bus.Put(m.address, m_REG_MANUAL, 0)
	return
}

// NewRMCS220xMotor sets up all the necessary components to run a motor through the custom MCU.
// Also sets up the onboard controller of the motor to include desired speed and damping parameters.
func NewRMCS220xMotor(bus UARTMCUInterface, switches *SwitchMCU, control uint16,
	address, rawLow, rawHigh int, speed, damping int32) (motor *RMCS220xMotor) {

	motor = new(RMCS220xMotor)
	motor.bus = bus
	motor.switches = switches
	motor.control = 1 << (control - 1)
	motor.address = address
	motor.rawLow = rawLow
	motor.rawHigh = rawHigh

	// calculate target and set to a raw value of zero
	motor.target = motor.scalePos(0, false)

	motor.bus.Put(motor.address, m_REG_MAX_SPEED, speed)
	motor.bus.Put(motor.address, m_REG_DAMPING, damping)
	return
}

// Device level functions

// NewDynastat sets up all the components of the device ready to go based on the config provided.
func NewDynastat(config *DynastatConfig) (dynastat *Dynastat, err error) {
	dynastat = new(Dynastat)
	dynastat.config = config
	switch config.Version {
	case 1:
		// initialise
		dynastat.Motors = make(map[string]MotorInterface, len(config.Motors))
		dynastat.sensors = make(map[string]SensorInterface, len(config.Sensors))

		// Open COM ports
		dynastat.SensorBus = OpenI2C(fmt.Sprintf("/dev/i2c-%d", config.I2CBus.Sensor))
		dynastat.motorBus = OpenUARTMCU(config.UART.Motor)

		dynastat.switches, err = NewSwitchMCU(dynastat.SensorBus, sm_ADDRESS)
		if err != nil {
			fmt.Errorf("Unable to proceed with homing: %s", err)
			dynastat.switches = nil
		}

		for name, conf := range config.Motors {
			dynastat.Motors[name] = NewRMCS220xMotor(
				dynastat.motorBus,
				dynastat.switches,
				conf.Control,
				conf.Address,
				conf.Low,
				conf.High,
				conf.Speed,
				conf.Damping,
			)
		}

		var sensorBoards map[int]*SensorBoard
		for name, conf := range config.Sensors {
			board, exists := sensorBoards[conf.Address]

			if !exists {
				board = &SensorBoard{
					dynastat.SensorBus,
					conf.Address,
					make([]byte, sb_ROWS*sb_COLS*2),
				}
				board.SetMode(conf.Mode)
				go board.Update()
			}

			dynastat.sensors[name], _ = NewSensor(
				board,
				conf.Registry,
				conf.Mirror,
				conf.Rows,
				conf.Cols,
				conf.ZeroValue,
				conf.HalfValue,
				conf.FullValue,
			)
		}

		break

	default:
		return nil, errors.New("Unkown config version")
	}
	return
}

// SetMotor issues the write command to the desired motor with an application level value.
func (d *Dynastat) SetMotor(name string, position int) (err error) {
	motor, ok := d.Motors[name]
	if ok == false {
		return errors.New(fmt.Sprintf("Unkown motor %s", name))
	}
	motor.SetTarget(position)
	return nil
}

func (d *Dynastat) HomeMotor(name string) (err error) {
	motor, ok := d.Motors[name]
	if ok == false {
		return errors.New(fmt.Sprintf("Unable to find motor %s", name))
	}
	err = motor.Home(d.config.Motors[name].Cal)
	return
}

func (d *Dynastat) GotoMotorRaw(name string, position int) (err error) {
	motor, ok := d.Motors[name]
	if !ok {
		return errors.New(fmt.Sprintf("Unkown motor %s", name))
	}
	motor.putRaw(m_REG_GOTO, position)
	return nil
}

func (d *Dynastat) WriteMotorRaw(name string, position int) (err error) {
	motor, ok := d.Motors[name]
	if !ok {
		return errors.New(fmt.Sprintf("Unkown motor %s", name))
	}
	motor.putRaw(m_REG_POSITION, position)
	return nil
}

func (d *Dynastat) RecordMotorLow(name string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	motor, ok := d.Motors[name]
	if !ok {
		return errors.New(fmt.Sprintf("Unkown motor %s", name))
	}

	pos, err := motor.getRaw(m_REG_POSITION)
	if err != nil {
		return err
	}

	// update the config
	conf := d.config.Motors[name]
	conf.Low = pos
	d.config.Motors[name] = conf

	// recreate the motor with new values
	motor = NewRMCS220xMotor(
		d.motorBus,
		d.switches,
		conf.Control,
		conf.Address,
		conf.Low,
		conf.High,
		conf.Speed,
		conf.Damping,
	)
	d.Motors[name] = motor

	return nil
}

func (d *Dynastat) RecordMotorHigh(name string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	motor, ok := d.Motors[name]
	if !ok {
		return errors.New(fmt.Sprintf("Unkown motor %s", name))
	}

	pos, err := motor.getRaw(m_REG_POSITION)
	if err != nil {
		return err
	}

	// update the config
	conf := d.config.Motors[name]
	conf.High = pos
	d.config.Motors[name] = conf

	// recreate the motor with new values
	motor = NewRMCS220xMotor(
		d.motorBus,
		d.switches,
		conf.Control,
		conf.Address,
		conf.Low,
		conf.High,
		conf.Speed,
		conf.Damping,
	)
	d.Motors[name] = motor

	return nil
}

func (d *Dynastat) RecordMotorHome(name string, reverse bool) (pos int, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	motor, ok := d.Motors[name]
	if !ok {
		return 0, errors.New(fmt.Sprintf("Unkown motor %s", name))
	}

	motor.findHome(reverse)

	time.Sleep(time.Second / 2)

	pos, err = motor.getRaw(m_REG_POSITION)
	if pos == 0 {
		err = errors.New("Recieved position of 0")
	}
	if err != nil {
		return
	}

	// update the config
	conf := d.config.Motors[name]
	conf.Cal = pos
	d.config.Motors[name] = conf

	// recreate the motor with new values
	motor = NewRMCS220xMotor(
		d.motorBus,
		d.switches,
		conf.Control,
		conf.Address,
		conf.Low,
		conf.High,
		conf.Speed,
		conf.Damping,
	)
	d.Motors[name] = motor

	time.Sleep(time.Second / 2)

	// reset to a sensible position
	motor.putRaw(m_REG_GOTO, 0)

	return
}

// readSensors calls GetState on each sensor to build a dictionary of the Current sensor readings.
func (d *Dynastat) readSensors() (result map[string]SensorState) {
	result = make(map[string]SensorState)
	for name, sensor := range d.sensors {
		result[name] = sensor.GetState()
	}
	return
}

// readMotors calls GetState on each motor to build a dictionary of the Current motor states.
func (d *Dynastat) readMotors() (result map[string]MotorState, err error) {
	result = make(map[string]MotorState)
	for name, motor := range d.Motors {
		state, err := motor.GetState()
		if err != nil {
			return nil, err
		}
		result[name] = state
	}
	return
}

// GetState builds a complete state of the device including sensor and motor states.
func (d *Dynastat) GetState() (result DynastatState, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	result.Motors, err = d.readMotors()
	result.Sensors = d.readSensors()
	return
}

func (d *Dynastat) GetConfig() *DynastatConfig {
	return d.config
}

// DecodeMsg implements msgp.Decodable
func (z *DynastatState) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zsbz uint32
	zsbz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zsbz > 0 {
		zsbz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Motors":
			var zrjx uint32
			zrjx, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Motors == nil && zrjx > 0 {
				z.Motors = make(map[string]MotorState, zrjx)
			} else if len(z.Motors) > 0 {
				for key := range z.Motors {
					delete(z.Motors, key)
				}
			}
			for zrjx > 0 {
				zrjx--
				var zjpj string
				var zzpf MotorState
				zjpj, err = dc.ReadString()
				if err != nil {
					return
				}
				var zawn uint32
				zawn, err = dc.ReadMapHeader()
				if err != nil {
					return
				}
				for zawn > 0 {
					zawn--
					field, err = dc.ReadMapKeyPtr()
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "Target":
						zzpf.Target, err = dc.ReadInt()
						if err != nil {
							return
						}
					case "Current":
						zzpf.Current, err = dc.ReadInt()
						if err != nil {
							return
						}
					default:
						err = dc.Skip()
						if err != nil {
							return
						}
					}
				}
				z.Motors[zjpj] = zzpf
			}
		case "Sensors":
			var zwel uint32
			zwel, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Sensors == nil && zwel > 0 {
				z.Sensors = make(map[string]SensorState, zwel)
			} else if len(z.Sensors) > 0 {
				for key := range z.Sensors {
					delete(z.Sensors, key)
				}
			}
			for zwel > 0 {
				zwel--
				var zrfe string
				var zgmo SensorState
				zrfe, err = dc.ReadString()
				if err != nil {
					return
				}
				var zrbe uint32
				zrbe, err = dc.ReadArrayHeader()
				if err != nil {
					return
				}
				if cap(zgmo) >= int(zrbe) {
					zgmo = (zgmo)[:zrbe]
				} else {
					zgmo = make(SensorState, zrbe)
				}
				for ztaf := range zgmo {
					var zmfd uint32
					zmfd, err = dc.ReadArrayHeader()
					if err != nil {
						return
					}
					if cap(zgmo[ztaf]) >= int(zmfd) {
						zgmo[ztaf] = (zgmo[ztaf])[:zmfd]
					} else {
						zgmo[ztaf] = make([]uint8, zmfd)
					}
					for zeth := range zgmo[ztaf] {
						zgmo[ztaf][zeth], err = dc.ReadUint8()
						if err != nil {
							return
						}
					}
				}
				z.Sensors[zrfe] = zgmo
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *DynastatState) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "Motors"
	err = en.Append(0x82, 0xa6, 0x4d, 0x6f, 0x74, 0x6f, 0x72, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.Motors)))
	if err != nil {
		return
	}
	for zjpj, zzpf := range z.Motors {
		err = en.WriteString(zjpj)
		if err != nil {
			return
		}
		// map header, size 2
		// write "Target"
		err = en.Append(0x82, 0xa6, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74)
		if err != nil {
			return err
		}
		err = en.WriteInt(zzpf.Target)
		if err != nil {
			return
		}
		// write "Current"
		err = en.Append(0xa7, 0x43, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74)
		if err != nil {
			return err
		}
		err = en.WriteInt(zzpf.Current)
		if err != nil {
			return
		}
	}
	// write "Sensors"
	err = en.Append(0xa7, 0x53, 0x65, 0x6e, 0x73, 0x6f, 0x72, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.Sensors)))
	if err != nil {
		return
	}
	for zrfe, zgmo := range z.Sensors {
		err = en.WriteString(zrfe)
		if err != nil {
			return
		}
		err = en.WriteArrayHeader(uint32(len(zgmo)))
		if err != nil {
			return
		}
		for ztaf := range zgmo {
			err = en.WriteArrayHeader(uint32(len(zgmo[ztaf])))
			if err != nil {
				return
			}
			for zeth := range zgmo[ztaf] {
				err = en.WriteUint8(zgmo[ztaf][zeth])
				if err != nil {
					return
				}
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *DynastatState) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Motors"
	o = append(o, 0x82, 0xa6, 0x4d, 0x6f, 0x74, 0x6f, 0x72, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.Motors)))
	for zjpj, zzpf := range z.Motors {
		o = msgp.AppendString(o, zjpj)
		// map header, size 2
		// string "Target"
		o = append(o, 0x82, 0xa6, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74)
		o = msgp.AppendInt(o, zzpf.Target)
		// string "Current"
		o = append(o, 0xa7, 0x43, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74)
		o = msgp.AppendInt(o, zzpf.Current)
	}
	// string "Sensors"
	o = append(o, 0xa7, 0x53, 0x65, 0x6e, 0x73, 0x6f, 0x72, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.Sensors)))
	for zrfe, zgmo := range z.Sensors {
		o = msgp.AppendString(o, zrfe)
		o = msgp.AppendArrayHeader(o, uint32(len(zgmo)))
		for ztaf := range zgmo {
			o = msgp.AppendArrayHeader(o, uint32(len(zgmo[ztaf])))
			for zeth := range zgmo[ztaf] {
				o = msgp.AppendUint8(o, zgmo[ztaf][zeth])
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *DynastatState) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zzdc uint32
	zzdc, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zzdc > 0 {
		zzdc--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Motors":
			var zelx uint32
			zelx, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Motors == nil && zelx > 0 {
				z.Motors = make(map[string]MotorState, zelx)
			} else if len(z.Motors) > 0 {
				for key := range z.Motors {
					delete(z.Motors, key)
				}
			}
			for zelx > 0 {
				var zjpj string
				var zzpf MotorState
				zelx--
				zjpj, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				var zbal uint32
				zbal, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for zbal > 0 {
					zbal--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "Target":
						zzpf.Target, bts, err = msgp.ReadIntBytes(bts)
						if err != nil {
							return
						}
					case "Current":
						zzpf.Current, bts, err = msgp.ReadIntBytes(bts)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
				z.Motors[zjpj] = zzpf
			}
		case "Sensors":
			var zjqz uint32
			zjqz, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Sensors == nil && zjqz > 0 {
				z.Sensors = make(map[string]SensorState, zjqz)
			} else if len(z.Sensors) > 0 {
				for key := range z.Sensors {
					delete(z.Sensors, key)
				}
			}
			for zjqz > 0 {
				var zrfe string
				var zgmo SensorState
				zjqz--
				zrfe, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				var zkct uint32
				zkct, bts, err = msgp.ReadArrayHeaderBytes(bts)
				if err != nil {
					return
				}
				if cap(zgmo) >= int(zkct) {
					zgmo = (zgmo)[:zkct]
				} else {
					zgmo = make(SensorState, zkct)
				}
				for ztaf := range zgmo {
					var ztmt uint32
					ztmt, bts, err = msgp.ReadArrayHeaderBytes(bts)
					if err != nil {
						return
					}
					if cap(zgmo[ztaf]) >= int(ztmt) {
						zgmo[ztaf] = (zgmo[ztaf])[:ztmt]
					} else {
						zgmo[ztaf] = make([]uint8, ztmt)
					}
					for zeth := range zgmo[ztaf] {
						zgmo[ztaf][zeth], bts, err = msgp.ReadUint8Bytes(bts)
						if err != nil {
							return
						}
					}
				}
				z.Sensors[zrfe] = zgmo
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *DynastatState) Msgsize() (s int) {
	s = 1 + 7 + msgp.MapHeaderSize
	if z.Motors != nil {
		for zjpj, zzpf := range z.Motors {
			_ = zzpf
			s += msgp.StringPrefixSize + len(zjpj) + 1 + 7 + msgp.IntSize + 8 + msgp.IntSize
		}
	}
	s += 8 + msgp.MapHeaderSize
	if z.Sensors != nil {
		for zrfe, zgmo := range z.Sensors {
			_ = zgmo
			s += msgp.StringPrefixSize + len(zrfe) + msgp.ArrayHeaderSize
			for ztaf := range zgmo {
				s += msgp.ArrayHeaderSize + (len(zgmo[ztaf]) * (msgp.Uint8Size))
			}
		}
	}
	return
}
