package onboard

import (
	"fmt"
	"math/rand"
	"time"
)

const SENSOR_DELTA = 5
const SENSOR_INTERVAL = time.Second / 10

const MOTOR_DELTA = 5
const MOTOR_INTERVAL = time.Second / 5

type SimulatedSensor struct {
	values     []uint8
	rows, cols int
}

type SimulatedMotor struct {
	name            string
	current, target int
}

func (s *SimulatedSensor) SetScale(zero, half, full uint16) {
	panic("[NotImplemented][SimulatedSensor] SetScale is not implemented nor required on SimulatedSensor")
}

func (s *SimulatedSensor) GetValue(row, col int) uint8 {
	return s.values[(row*s.cols)+col]
}

func (s *SimulatedSensor) GetState() (state SensorState) {
	state = make(SensorState, s.rows)
	for i := 0; i < s.rows; i++ {
		state[i] = make([]uint8, s.cols)
		for j := 0; j < s.cols; j++ {
			if i == j {
				state[i][j] = 0
			} else {
				state[i][j] = s.GetValue(i, j)
			}
		}
	}
	return state
}

func (s *SimulatedSensor) update() {
	for {
		for i := 0; i < len(s.values); i++ {
			s.values[i] += uint8(rand.Intn(SENSOR_DELTA*2) - SENSOR_DELTA)
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

func (m *SimulatedMotor) SetTarget(target int) {
	fmt.Printf("Setting motor %s target to %d\n", m.name, target)
	m.target = target
	return
}

func (m *SimulatedMotor) GetPosition() (position int, err error) {
	return m.current, nil
}

func (m *SimulatedMotor) Home(calibrationValue int) {
	fmt.Printf("Homing to %d \n", calibrationValue)
	return
}

func (m *SimulatedMotor) GetState() (state MotorState, err error) {
	state.Current = m.current
	state.Target = m.target
	return state, nil
}

func (m *SimulatedMotor) getRaw(reg uint8) (int, error) {
	panic("[NotImplemented][SimulatedMotor] getRaw not implemented in SimulatedMotor")
}

func (m *SimulatedMotor) putRaw(reg uint8, val int) {
	panic("[NotImplemented][SimulatedMotor] putRaw not implemented in SimulatedMotor")
}

func (m *SimulatedMotor) findHome(reverse bool) {
	panic("[NotImplemented][SimulatedMotor] findHome not implemented in SimulatedMotor")
}

func (m *SimulatedMotor) update() {
	for {
		if m.current != m.target {
			delta := m.target - m.current
			// make sure we go in the correct direction and value is less than the const in the respective direction
			if delta > 0 {
				if delta > MOTOR_DELTA {
					delta = MOTOR_DELTA
				}

			} else {
				if delta < -MOTOR_DELTA {
					delta = -MOTOR_DELTA
				}
			}

			// apply movement
			m.current += delta
		}

		time.Sleep(MOTOR_INTERVAL)
	}
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

		for name := range config.Motors {
			m := &SimulatedMotor{name: name}
			go m.update()
			dynastat.Motors[name] = m
		}
	default:
		panic("Unkown version number")
	}

	return dynastat
}
