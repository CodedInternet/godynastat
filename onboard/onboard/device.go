package onboard

import (
	"github.com/goburrow/serial"
	"sync"
	"fmt"
	"log"
	"time"
)

type UARTMCU struct {
	port serial.Port
	lock sync.Mutex
}

func (mcu *UARTMCU) Open(ttyName string) {
	port, err := serial.Open(&serial.Config{
		Address:  ttyName,
		BaudRate: 115200,
		Timeout: 0.5*time.Second,
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
