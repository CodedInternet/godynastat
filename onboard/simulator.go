package onboard

import (
	"fmt"
	"math/rand"
	"time"
)

const SENSOR_DELTA = 5
const SENSOR_INTERVAL = time.Second / 10

type SimulatedSensor struct {
	values     []uint8
	rows, cols int
}

func (s *SimulatedSensor) SetScale(zero, half, full uint16) {
	fmt.Errorf("SetScale is not implemented nor required on SimulatedSensor")
}

func (s *SimulatedSensor) GetValue(row, col int) uint8 {
	return s.values[(row*s.cols)+col]
}

func (s *SimulatedSensor) GetState() (state SensorState) {
	state = make(SensorState, s.rows)
	for i := 0; i < s.rows; i++ {
		state[i] = make([]uint8, s.cols)
		for j := 0; j < s.cols; j++ {
			state[i][j] = s.GetValue(i, j)
		}
	}
	return state
}

func (s *SimulatedSensor) update() {
	for {
		for i := 0; i < len(s.values); i++ {
			val := s.values[i]
			delta := rand.Intn(SENSOR_DELTA*2) - SENSOR_DELTA
			if delta < 0 {
				val = val - uint8(delta)
			} else {
				val = val + uint8(delta)
			}
			s.values[i] = val
		}
		time.Sleep(SENSOR_INTERVAL)
	}
}

func NewSimulatedSensor(rows, cols int) (sensor *SimulatedSensor) {
	sensor = new(SimulatedSensor)
	sensor.rows = rows
	sensor.cols = cols
	sensor.values = make([]uint8, rows*cols)
	go sensor.update()
	return
}

func NewDynastatSimulator(config *DynastatConfig) (dynastat *Dynastat) {
	dynastat = new(Dynastat)
	dynastat.config = config

	switch config.Version {
	case 1:
		// initialise
		dynastat.Motors = make(map[string]MotorInterface, len(config.Motors))
		dynastat.sensors = make(map[string]SensorInterface, len(config.Sensors))

		for name, conf := range config.Sensors {
			dynastat.sensors[name] = NewSimulatedSensor(conf.Rows, conf.Cols)
		}
		// establish standard device
	default:
		panic("Unkown version number")
	}

	return dynastat
}
