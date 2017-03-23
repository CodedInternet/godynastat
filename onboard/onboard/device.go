package onboard

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

type SensorInterface interface {
	SetScale(zero, half, full uint16)
	GetValue(row, col int) uint8
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

type MotorInterface interface {
	SetTarget(target int)
	GetPosition() (position int)
	Home(calibrationValue int)
}

type Dynastat struct {
	motors    map[string]MotorInterface
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

// Generic functions
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

func (mcu *UARTMCU) Close() {
	mcu.port.Close()
}

func (mcu *UARTMCU) Put(i2cAddr int, cmd uint8, value int32) {
	buf := fmt.Sprintf("M%d %d %d", i2cAddr, cmd, value)

	// Keep as little processing outside the critical section as possible
	mcu.lock.Lock()
	mcu.port.Write([]byte(buf))
	mcu.lock.Unlock()
}

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
func OpenI2C(dev string) *I2CBus {
	fd, err := os.Open(dev)
	if err != nil {
		panic(err)
	}

	return &I2CBus{fd: fd}
}

func ioctl(fd, cmd, arg uintptr) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_IOCTL, fd, cmd, arg, 0, 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}

func (bus *I2CBus) Connect(i2cAddr int) {
	if err := ioctl(bus.fd.Fd(), i2c_SLAVE, uintptr(i2cAddr)); err != nil {
		panic(err)
	}
	return
}

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

// Sensor Boards
func (sb *SensorBoard) Update() {
	for {
		sb.i2cBus.Get(sb.address, sb_REG_VALUES, sb.buf)
		time.Sleep(time.Second / FRAMERATE)
	}
}

func (sb *SensorBoard) getValue(reg int) uint16 {
	return binary.BigEndian.Uint16([]byte{sb.buf[reg], sb.buf[reg+1]})
}

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

// RMCS220xMotor
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

func (m *RMCS220xMotor) writePosition(pos int32) {
	m.bus.Put(m.address, m_REG_GOTO, pos)
}

func (m *RMCS220xMotor) readPosition() int32 {
	return m.bus.Get(m.address, m_REG_POSITION)
}

func (m *RMCS220xMotor) readControl() bool {
	buf := make([]byte, 2)
	m.controlBus.Get(m_CONTROL_ADDRESS, m_CONTROL_REG, buf)
	val := binary.LittleEndian.Uint16(buf)
	return val&m.control == 0
}

func (m *RMCS220xMotor) SetTarget(target int) {
	m.target = target
	m.writePosition(int32(m.scalePos(target, true)))
}

func (m *RMCS220xMotor) GetPosition() int {
	raw := m.readPosition()
	return m.scalePos(int(raw), false)
}

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
func NewDynastat(config DynastatConfig) (dynastat *Dynastat, err error) {
	switch config.Version {
	case 1:
		dynastat = new(Dynastat)
		dynastat.sensorBus = OpenI2C(fmt.Sprintf("/dev/i2c-%d", config.I2CBus.Sensor))
		dynastat.motorBus = OpenUARTMCU(config.UART.motor)

		for name, conf := range config.Motors {
			dynastat.motors[name] = NewRMCS220xMotor(
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
