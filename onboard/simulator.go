package onboard

import (
	"github.com/CodedInternet/godynastat/onboard/errors"
	"github.com/abiosoft/ishell"
)

//import (
//	"errors"
//	"fmt"
//	"math/rand"
//	"time"
//)
//
//const SENSOR_DELTA = 5
//const SENSOR_INTERVAL = time.Second / 10
//
//const MOTOR_DELTA = 5
//const MOTOR_INTERVAL = time.Second / 5
//
//type SimulatedSensor struct {
//	values     []uint8
//	rows, cols int
//}
//
//func (s *SimulatedSensor) SetScale(zero, half, full uint16) {
//	panic("[NotImplemented][SimulatedSensor] SetScale is not implemented nor required on SimulatedSensor")
//}
//
//func (s *SimulatedSensor) GetValue(row, col int) uint8 {
//	return s.values[(row*s.cols)+col]
//}
//
//func (s *SimulatedSensor) GetState() (state SensorState) {
//	state = make(SensorState, s.rows)
//	for i := 0; i < s.rows; i++ {
//		state[i] = make([]int, s.cols)
//		for j := 0; j < s.cols; j++ {
//			if i == j {
//				state[i][j] = 0
//			} else {
//				state[i][j] = int(s.GetValue(i, j))
//			}
//		}
//	}
//	return state
//}
//
//func (s *SimulatedSensor) update() {
//	for {
//		for i := 0; i < len(s.values); i++ {
//			s.values[i] += uint8(rand.Intn(SENSOR_DELTA*2) - SENSOR_DELTA)
//		}
//		time.Sleep(SENSOR_INTERVAL)
//	}
//}
//
//func NewSimulatedSensor(rows, cols int) (sensor *SimulatedSensor) {
//	sensor = new(SimulatedSensor)
//	sensor.rows = rows
//	sensor.cols = cols
//	sensor.values = make([]uint8, rows*cols)
//	go sensor.update()
//	return
//}
//
//func (m *SimulatedMotor) SetTarget(target int) {
//	fmt.Printf("Setting motor %s target to %d\n", m.name, target)
//	m.target = target
//	return
//}
//
//func (m *SimulatedMotor) GetPosition() (position int, err error) {
//	return m.current, nil
//}
//
//func (m *SimulatedMotor) Home(calibrationValue int) error {
//	fmt.Printf("Homing to %d \n", calibrationValue)
//	return nil
//}
//
//func (m *SimulatedMotor) GetState() (state MotorState, err error) {
//	state.Current = m.current
//	state.Target = m.target
//	return state, nil
//}
//
//func (m *SimulatedMotor) getRaw(reg uint8) (int, error) {
//	panic("[NotImplemented][SimulatedMotor] getRaw not implemented in SimulatedMotor")
//}
//
//func (m *SimulatedMotor) putRaw(reg uint8, val int) {
//	panic("[NotImplemented][SimulatedMotor] putRaw not implemented in SimulatedMotor")
//}
//
//func (m *SimulatedMotor) findHome(reverse bool) error {
//	return errors.New("[NotImplemented][SimulatedMotor] findHome not implemented in SimulatedMotor")
//}
//
//func (m *SimulatedMotor) update() {
//	for {
//		if m.current != m.target {
//			delta := m.target - m.current
//			// make sure we go in the correct direction and value is less than the const in the respective direction
//			if delta > 0 {
//				if delta > MOTOR_DELTA {
//					delta = MOTOR_DELTA
//				}
//
//			} else {
//				if delta < -MOTOR_DELTA {
//					delta = -MOTOR_DELTA
//				}
//			}
//
//			// apply movement
//			m.current += delta
//		}
//
//		time.Sleep(MOTOR_INTERVAL)
//	}
//}

const (
	rearfootPlatorm int = iota
	forefootPlatform
)

type simulatedDynastat struct {
	out       *ishell.Shell
	platforms map[string]int
}

func (d simulatedDynastat) SetRotation(platform string, degFront, degSag float64) (err error) {
	if _, ok := d.platforms[platform]; !ok {
		return errors.PlatformNameError{Name: platform}
	}
	d.out.Printf("[OK] Setting %s rotation to %.1fº frontal: %.1fº sag\n", platform, degFront, degSag)
	return
}

func (d simulatedDynastat) SetHeight(platform string, height float64) (err error) {
	if _, ok := d.platforms[platform]; !ok {
		return errors.PlatformNameError{Name: platform}
	}
	d.out.Printf("[OK] Setting %s height to %.2fmm\n", platform, height)
	return
}

func (d simulatedDynastat) SetFirstRay(platform string, angle float64) (err error) {
	pType, ok := d.platforms[platform]
	if !ok {
		return errors.PlatformNameError{Name: platform}
	}

	if pType != forefootPlatform {
		return errors.IncorrectPlatformError{
			Name:   platform,
			Action: "first_ray",
		}
	}

	d.out.Printf("[OK] Setting %s first ray to %.1fº\n", platform, angle)
	return
}

func (d simulatedDynastat) HomePlatform(platform string) (err error) {
	if _, ok := d.platforms[platform]; !ok {
		return errors.PlatformNameError{Name: platform}
	}
	d.out.Printf("[OK] Homing %s\n", platform)
	return
}

func (d simulatedDynastat) GetPlatformNames() (names []string) {
	for n := range d.platforms {
		names = append(names, n)
	}
	return names
}

func NewDynastatSimulator(config *DynastatConfig, output *ishell.Shell) Dynastat {
	d := new(simulatedDynastat)
	d.out = output

	switch config.Version {
	case 2:
		d.platforms = make(map[string]int, len(config.Platforms))

		for name, pConf := range config.Platforms {
			if len(pConf.Actuators) == 3 {
				d.platforms[name] = rearfootPlatorm
			} else {
				d.platforms[name] = forefootPlatform
			}

		}
	default:
		panic("unkown version number")
	}

	return d
}
