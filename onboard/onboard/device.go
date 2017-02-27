package onboard

import (
	"github.com/goburrow/serial"
	"sync"
	"fmt"
	"log"
	"time"
	"os"
	"syscall"
)

const (
	FRAMERATE     = 20
	i2c_SLAVE     = 0x0703
	SB_REG_VALUES = 0x0100
	SB_ROWS       = 16
	SB_COLS       = 24
	SB_BITS	      = 16
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

func (mcu *UARTMCU) Put(i2cAddr int, cmd uint8, value uint32) {
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
