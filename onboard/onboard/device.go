package onboard

import (
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
	FRAMERATE     = 20
	i2c_SLAVE     = 0x0703
	SB_REG_VALUES = 0x0100
	SB_ROWS       = 16
	SB_COLS       = 24
	SB_BITS       = 16

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

type I2CBus struct {
	fd   *os.File
	lock sync.Mutex
}

type SensorBoard struct {
	i2cBus      I2CBus
	address     int
	buf         []byte
	zeroValue   uint16
	scaleFactor float64
}

type RMCS220xMotor struct {
	bus        *UARTMCU
	controlBus *I2CBus
	address    int
	control    uint16
	rawLow     int
	rawHigh    int
	target     int
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
func (mcu *UARTMCU) Open(ttyName string) {
	port, err := serial.Open(&serial.Config{
		Address:  ttyName,
		BaudRate: 115200,
		Timeout:  0.5 * time.Second,
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	mcu.port = port
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
		sb.i2cBus.Get(sb.address, SB_REG_VALUES, sb.buf)
		time.Sleep(time.Second / FRAMERATE)
	}
}

func (sb *SensorBoard) SetScale(zero, half, full uint16) {
	var max, m1, m2 float64

	// Calculate the proportions of the scaling
	max = (2 ^ SB_BITS) - 1
	m1 = float64(half) / (max / 2)
	m2 = float64(full) / max

	// Assign struct values
	sb.zeroValue = zero
	sb.scaleFactor = (m1 + m2) / 2
}

func (sb *SensorBoard) GetValue(row, col int) uint16 {
	i := row*SB_COLS + col
	return uint16(sb.buf[i])<<8 + uint16(sb.buf[i+1])
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

		return translateValue(val, m.rawLow, m.rawHigh, 0, max)
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
	val := uint16(buf)
	return bool(val ^ 0&m.control)
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
