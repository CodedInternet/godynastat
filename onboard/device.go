// Copyright 2017 Coded Internet Ltd. All rights reserved.

/*
	A set of features related directly to the interaction with the device hardware.
*/

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/goburrow/serial"
	"log"
	"math"
	"os"
	"sync"
	"syscall"
	"time"
)

const (
	FRAMERATE = 20
	i2c_SLAVE = 0x0703

	sb_REG_VALUES = 0x0100
	sb_REG_ADDR   = 0x0004
	sb_ROWS       = 16
	sb_COLS       = 24
	sb_BITS       = 8
	s_BANK1_COLS  = 16
	s_BANK2_COLS  = 8

	// Motor constants
	m_BITS            = 8
	m_REG_MAX_SPEED   = 0
	m_REG_MANUAL      = 1
	m_REG_DAMPING     = 2
	m_REG_POSITION    = 3
	m_REG_GOTO        = 4
	m_CONTROL_ADDRESS = 0x20
	m_CONTROL_REG     = 3
)

type UARTMCU struct {
	port serial.Port
	lock sync.Mutex
}

type UARTMCUInterface interface {
	Put(i2cAddr int, cmd uint8, value int32)
	Get(i2cAddr int, cmd uint8) (value int32)
}

type I2CBus struct {
	fd   *os.File
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

type RMCS220xMotor struct {
	bus        UARTMCUInterface
	controlBus I2CBusInterface
	address    int
	control    uint16
	rawLow     int
	rawHigh    int
	target     int
}

type MotorState struct {
	target, current int
}

type MotorInterface interface {
	SetTarget(target int)
	GetPosition() (position int)
	Home(calibrationValue int)
	GetState() MotorState
}

type Dynastat struct {
	Motors    map[string]MotorInterface
	sensors   map[string]SensorInterface
	sensorBus I2CBusInterface
	motorBus  UARTMCUInterface
}

type DynastatConfig struct {
	Version          int
	SignalingServers []string
	I2CBus           struct {
		Sensor int
	}
	UART struct {
		motor string
	}
	Motors map[string]struct {
		Address        int
		Cal, Low, High int
		Speed, Damping int32
		Control        uint16
	}
	Sensors map[string]struct {
		Address, Mode                   int
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
	GetState() DynastatState
	SetMotor(name string, position int) (err error)
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
	port, err := serial.Open(&serial.Config{
		Address:  ttyName,
		BaudRate: 115200,
		Timeout:  time.Second / 2,
	})

	if err != nil {
		log.Fatal(err)
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
	buf := fmt.Sprintf("M%d %d %d", i2cAddr, cmd, value)

	// Keep as little processing outside the critical section as possible
	mcu.lock.Lock()
	mcu.port.Write([]byte(buf))
	mcu.lock.Unlock()
}

// Get returns values from the MCU on the specified registry
func (mcu *UARTMCU) Get(i2cAddr int, cmd uint8) (value int32) {
	// Create buffers and format strings outside of cricial section
	var input []byte
	buf := fmt.Sprintf("M%d %d", i2cAddr, cmd)

	// Perform read/write in critical section - keep to minimum to prevent excessive locking between threads
	mcu.lock.Lock()
	mcu.port.Write([]byte(buf))
	mcu.port.Read(input)
	mcu.lock.Unlock()

	// Process response for return
	fmt.Sscanf(string(input), "%d", value)
	return value
}

// I2C related functions

// OpenI2C performs the actions necessary to create the I2CBus object.
// This opens the file and does very basic error checking on it
func OpenI2C(dev string) *I2CBus {
	fd, err := os.Open(dev)
	if err != nil {
		panic(err)
	}

	return &I2CBus{fd: fd}
}

// ioctl proxy to appropriate syscall method.
// This is part of our own i2c library
func ioctl(fd, cmd, arg uintptr) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_IOCTL, fd, cmd, arg, 0, 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}

// Connect send the commands to put the receiving device into slave mode so it can accept commands from the BBB
func (bus *I2CBus) Connect(i2cAddr int) {
	if err := ioctl(bus.fd.Fd(), i2c_SLAVE, uintptr(i2cAddr)); err != nil {
		panic(err)
	}
	return
}

// Get performs write/read to get a value from an I2C device in a 16bit registry space.
// Thread-safe.
func (bus *I2CBus) Get(i2cAddr int, reg uint16, buf []byte) {
	// perform bitbashing to get write command first
	var wbuf []byte
	wbuf[0] = byte(reg >> 8 & 0xff)
	wbuf[1] = byte(reg & 0xff)

	bus.lock.Lock()
	bus.Connect(i2cAddr)
	// Do write/read inside critical section
	bus.fd.Write(wbuf)
	bus.fd.Read(buf)

	bus.lock.Unlock()
}

// Put performs write to I2C device in a 16bit registry space.
// Thread-safe
func (bus *I2CBus) Put(i2cAddr int, reg uint16, buf []byte) {
	wbuf := make([]byte, len(buf)+2)
	wbuf[0] = byte(reg >> 8 & 0xff)
	wbuf[1] = byte(reg & 0xff)

	bus.lock.Lock()
	bus.Connect(i2cAddr)
	bus.fd.Write(wbuf)
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

// getValue returns a uint16 from the appropriate reg and +1 to ease with this process
func (sb *SensorBoard) getValue(reg int) uint16 {
	return binary.BigEndian.Uint16([]byte{sb.buf[reg], sb.buf[reg+1]})
}

// changeAddress updates the address register on the sensor board.
// This update to the hardware requires a reboot of the sensor board.
func (sb *SensorBoard) changeAddress(newAddr int) {
	if newAddr < 0x00 || newAddr > 0x7f {
		log.Fatalf("Invalid address: %x", newAddr)
		return
	}

	// Read old address to sanity check
	buf := make([]byte, 1)
	sb.i2cBus.Get(sb.address, sb_REG_ADDR, buf)
	oldAddr := int(buf[0])

	if oldAddr != sb.address {
		log.Fatalf("Stored address %x does not match current device %x", oldAddr, sb.address)
	}

	buf[0] = byte(newAddr)
	sb.i2cBus.Put(sb.address, sb_REG_ADDR, buf)
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

// RMCS220xMotor

// scalePos takes in a value and either scales it up to 16bit motor range or down to 0-255 application range.
func (m *RMCS220xMotor) scalePos(val int, up bool) int {
	max := int(math.Pow(2, float64(m_BITS)))
	if up {
		return translateValue(val, 0, max, m.rawLow, m.rawHigh)
	} else {
		if val < m.rawLow {
			val = m.rawLow
		} else if val > m.rawHigh {
			val = m.rawHigh
		}

		val := translateValue(val, m.rawLow, m.rawHigh, 0, max)
		if val > 255 { // clamp value to 255
			val = 255
		}
		return val
	}
}

// writePosition performs the write to the motor.
func (m *RMCS220xMotor) writePosition(pos int32) {
	m.bus.Put(m.address, m_REG_GOTO, pos)
}

// readPosition gets the current position from the motors encoder.
func (m *RMCS220xMotor) readPosition() int32 {
	return m.bus.Get(m.address, m_REG_POSITION)
}

// readControl looks at the control pin for the current motor and determines if it has been pressed.
func (m *RMCS220xMotor) readControl() bool {
	buf := make([]byte, 2)
	m.controlBus.Get(m_CONTROL_ADDRESS, m_CONTROL_REG, buf)
	val := binary.LittleEndian.Uint16(buf)
	return val&m.control == 0
}

// SetTarget updates the current target in software and issues the write to the motor with the scaled value.
func (m *RMCS220xMotor) SetTarget(target int) {
	m.target = target
	m.writePosition(int32(m.scalePos(target, true)))
}

// GetPosition reads the position from motor and scales it to application range.
func (m *RMCS220xMotor) GetPosition() int {
	raw := m.readPosition()
	return m.scalePos(int(raw), false)
}

// Home gradually moves the motor until it is pressing its home pin.
// To avoid crashes being potentially destructive to hardware, this in done in small increments so the motor will not
// continue unless the software deems it safe and reissues the move command.
func (m *RMCS220xMotor) Home(cal int) {
	inc := int32(math.Abs(float64(m.rawHigh-m.rawLow))) / 10
	// Invert increment so we go in the right direction
	if cal < 0 {
		inc = -inc
	}

	for !m.readControl() {
		m.writePosition(m.readPosition() + inc)
	}

	m.bus.Put(m.address, m_REG_POSITION, int32(cal))

	m.writePosition(0)
}

// GetState provides information on the desired and current position of the motor.
// This can be used to determine if the motor is currently at its target or is in transit
func (m *RMCS220xMotor) GetState() (state MotorState) {
	state.target = m.target
	state.current = m.GetPosition()
	return
}

// NewRMCS220xMotor sets up all the necessary components to run a motor through the custom MCU.
// Also sets up the onboard controller of the motor to include desired speed and damping parameters.
func NewRMCS220xMotor(bus UARTMCUInterface, controlBus I2CBusInterface, control uint16,
	address, rawLow, rawHigh int, speed, damping int32) (motor *RMCS220xMotor) {

	motor = new(RMCS220xMotor)
	motor.bus = bus
	motor.controlBus = controlBus
	motor.control = control
	motor.address = address
	motor.rawLow = rawLow
	motor.rawHigh = rawHigh

	motor.bus.Put(motor.address, m_REG_MAX_SPEED, speed)
	motor.bus.Put(motor.address, m_REG_DAMPING, damping)
	return
}

// Device level functions

// NewDynastat sets up all the components of the device ready to go based on the config provided.
func NewDynastat(config DynastatConfig) (dynastat *Dynastat, err error) {
	switch config.Version {
	case 1:
		dynastat = new(Dynastat)
		dynastat.sensorBus = OpenI2C(fmt.Sprintf("/dev/i2c-%d", config.I2CBus.Sensor))
		dynastat.motorBus = OpenUARTMCU(config.UART.motor)

		for name, conf := range config.Motors {
			dynastat.Motors[name] = NewRMCS220xMotor(
				dynastat.motorBus,
				dynastat.sensorBus,
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
					dynastat.sensorBus,
					conf.Address,
					make([]byte, sb_ROWS*sb_COLS),
				}
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

// readSensors calls GetState on each sensor to build a dictionary of the current sensor readings.
func (d *Dynastat) readSensors() (result map[string]SensorState) {
	result = make(map[string]SensorState)
	for name, sensor := range d.sensors {
		result[name] = sensor.GetState()
	}
	return
}

// readMotors calls GetState on each motor to build a dictionary of the current motor states.
func (d *Dynastat) readMotors() (result map[string]MotorState) {
	result = make(map[string]MotorState)
	for name, motor := range d.Motors {
		result[name] = motor.GetState()
	}
	return
}

// GetState builds a complete state of the device including sensor and motor states.
func (d *Dynastat) GetState() (result DynastatState) {
	result.Motors = d.readMotors()
	result.Sensors = d.readSensors()
	return
}
