package onboard

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/yaml.v2"
	"math"
	"testing"
	"time"
)

const (
	rows  = 5
	cols  = 10
	count = 2
)

func TestSimulatedSensor(t *testing.T) {
	Convey("General simulated sensor", t, func() {
		sensor := NewSimulatedSensor(rows, cols)

		So(len(sensor.values), ShouldEqual, rows*cols)

		Convey("Values change over time", func() {
			time.Sleep(SENSOR_INTERVAL * count) // give the goroutine time to do some changes

			zeros := 0
			state := sensor.GetState()
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					if state[r][c] == 0 {
						zeros += 1
					}
				}
			}

			So(zeros, ShouldBeLessThan, math.Max(rows, cols)+5)
		})

		Convey("Setting scale panics", func() {
			So(func() { sensor.SetScale(0, 10, 20) }, ShouldPanic)
		})
	})

	Convey("Specific sensor cell addressing works", t, func() {
		sensor := new(SimulatedSensor)
		sensor.rows = rows
		sensor.cols = cols
		sensor.values = make([]uint8, rows*cols)

		// address cell 22 - row 3 col 2 - r 2 c 1
		sensor.values[21] = uint8(48)
		So(sensor.GetValue(2, 1), ShouldEqual, 48)
	})
}

func TestSimulatedMotor(t *testing.T) {
	motor := &SimulatedMotor{name: "TEST"}

	Convey("GetPosition returns current value", t, func() {
		motor.current = 42
		pos, _ := motor.GetPosition()
		So(pos, ShouldEqual, 42)
	})

	Convey("SetTarget updates correctly", t, func() {
		motor.SetTarget(42)
		So(motor.target, ShouldEqual, 42)
	})

	Convey("GetState gives the correct values in the state object", t, func() {
		motor.target = 12
		motor.current = 24
		state, err := motor.GetState()
		So(err, ShouldBeNil)
		So(state.Target, ShouldEqual, 12)
		So(state.Current, ShouldEqual, 24)
	})

	Convey("Updater moves towards the target", t, func() {
		motor.current = 0
		motor.target = 255
		go motor.update()

		time.Sleep(MOTOR_INTERVAL * count)
		So(motor.current, ShouldBeBetweenOrEqual, MOTOR_DELTA*(count-1), MOTOR_DELTA*(count+1))

		Convey("Test in reverse", func() {
			motor.current = 255
			motor.target = 0

			time.Sleep(MOTOR_INTERVAL * count)
			So(motor.current, ShouldBeBetweenOrEqual, 255-MOTOR_DELTA*(count-1), 255-MOTOR_DELTA*(count+1))
		})
	})

	Convey("Not implemented methods panic correctly", t, func() {
		So(func() { motor.getRaw(4) }, ShouldPanic)
		So(func() { motor.putRaw(4, 28) }, ShouldPanic)
		So(func() { motor.findHome(true) }, ShouldPanic)
	})
}

func TestNewDynastatSimulator(t *testing.T) {
	yamlFile := fmt.Sprintf(`---
version: 1
motors:
  TESTM:
    address: 0x12
sensors:
  TESTS:
    rows: %d
    cols: %d
`, rows, cols)

	var config *DynastatConfig
	err := yaml.Unmarshal([]byte(yamlFile), &config)
	if err != nil {
		panic(err)
	}

	Convey("Device gets create successfully", t, func() {
		dynastat := NewDynastatSimulator(config)
		So(len(dynastat.Motors), ShouldEqual, 1)
		So(len(dynastat.sensors), ShouldEqual, 1)

		So(dynastat.Motors["TESTM"], ShouldHaveSameTypeAs, &SimulatedMotor{})
		So(dynastat.sensors["TESTS"], ShouldHaveSameTypeAs, &SimulatedSensor{})
		So(func() { dynastat.sensors["TESTS"].GetValue(rows-1, cols-1) }, ShouldNotPanic)

		Convey("Worker threads have started successfully", func() {
			time.Sleep(SENSOR_INTERVAL * count)
			So(dynastat.sensors["TESTS"].GetValue(rows-1, cols-1), ShouldBeGreaterThan, 0)

			motor := dynastat.Motors["TESTM"]
			pos, _ := motor.GetPosition()
			target := pos + MOTOR_DELTA*count
			motor.SetTarget(target)
			time.Sleep(MOTOR_INTERVAL * (count + 1))
			pos, _ = motor.GetPosition()
			So(pos, ShouldEqual, target)
		})
	})

	Convey("Invalid version number panics", t, func() {
		config.Version = -48
		So(func() { NewDynastatSimulator(config) }, ShouldPanic)
	})
}
