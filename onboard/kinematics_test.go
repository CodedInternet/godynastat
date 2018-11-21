package onboard

import (
	"github.com/CodedInternet/godynastat/onboard/hardware"
	"github.com/go-gl/mathgl/mgl64"
	. "github.com/smartystreets/goconvey/convey"
	. "math"
	"testing"
)

type TestActuator struct {
	target, speed uint8
}

func (t *TestActuator) GetTarget() (target uint8) {
	return t.target
}

func (t *TestActuator) SetTarget(target, speed uint8) {
	t.target = target
	t.speed = speed
}

type TestControlNode struct {
	commited bool
}

func (*TestControlNode) Send(cmd hardware.NodeCommand) (hardware.NodeCommand, error) {
	panic("implement me")
}

func (*TestControlNode) StageReset() (err error) {
	panic("implement me")
}

func (t *TestControlNode) StageCommit() (err error) {
	t.commited = true
	return
}

func genActuatorConfig(numPoints int, baseRadius, platformRadius, minLegLength float64) (config []PlatformActuator, testActuators []*TestActuator) {
	config = make([]PlatformActuator, numPoints)
	testActuators = make([]*TestActuator, numPoints)

	minHeight := Sqrt(Pow(minLegLength, 2) - Pow(Abs(baseRadius-platformRadius), 2))

	slice := 2 * Pi / float64(numPoints)
	for i := 0; i < numPoints; i++ {
		f := float64(i)
		angle := slice * f
		testActuators[i] = new(TestActuator)
		c := PlatformActuator{
			Actuator:    testActuators[i],
			LowerCoords: mgl64.Vec3{baseRadius * Sin(angle), baseRadius * Cos(angle), 0},
			UpperCoords: mgl64.Vec3{platformRadius * Sin(angle), platformRadius * Cos(angle), minHeight},
			MinLength:   minLegLength,
		}

		config[i] = c
	}

	return
}

func TestRearfootPlatform(t *testing.T) {
	Convey("3dof platform created from radius", t, func() {
		testControlNode := new(TestControlNode)
		// this test depends entirely on manual calcuation based on these parameters, DO NOT MODIFY
		config, testActuators := genActuatorConfig(3, 50, 40, 105)
		platform := NewRearfootPlatform(config)
		platform.Node = testControlNode
		So(platform, ShouldNotBeNil)

		Convey("top and bottom coordinates are calculated correctly", func() {

			baseTest := []mgl64.Vec3{{0, 50, 0}, {43.301, -25, 0}, {-43.301, -25, 0}}
			platformTest := []mgl64.Vec3{{0, 40, 104.523}, {34.641, -20, 104.523}, {-34.641, -20, 104.523}}
			for i := 0; i < 3; i++ {
				So(platform.Actuators[i].LowerCoords[0], ShouldAlmostEqual, baseTest[i][0], .001)
				So(platform.Actuators[i].LowerCoords[1], ShouldAlmostEqual, baseTest[i][1], .001)
				So(platform.Actuators[i].LowerCoords[2], ShouldAlmostEqual, baseTest[i][2], .001)

				So(platform.Actuators[i].UpperCoords[0], ShouldAlmostEqual, platformTest[i][0], .001)
				So(platform.Actuators[i].UpperCoords[1], ShouldAlmostEqual, platformTest[i][1], .001)
				So(platform.Actuators[i].UpperCoords[2], ShouldAlmostEqual, platformTest[i][2], .001)
			}
		})

		Convey("with 0 values, a zero leg extension is reported", func() {
			platform.SetRotation(0, 0, 0)
			platform.SetTranslation(0, 0, 0)
			platform.Set()

			So(testControlNode.commited, ShouldEqual, true)

			for i := 0; i < 3; i++ {
				So(testActuators[i].target, ShouldEqual, 0)
			}
		})

		Convey("extremes of height are correct", func() {
			platform.SetRotation(0, 0, 0)
			platform.SetTranslation(0, 0, 100)
			platform.Set()

			So(testControlNode.commited, ShouldEqual, true)

			for i := 0; i < 3; i++ {
				So(testActuators[i].target, ShouldEqual, 100)
				So(testActuators[i].speed, ShouldEqual, 255)
			}
		})

		Convey("midrange height is correct for angle calculations", func() {
			platform.SetRotation(0, 0, 0)
			platform.SetTranslation(0, 0, 50)
			platform.Set()

			So(testControlNode.commited, ShouldEqual, true)

			for i := 0; i < 3; i++ {
				So(testActuators[i].target, ShouldEqual, 50)
				So(testActuators[i].speed, ShouldEqual, 255)
			}

			Convey("10º roll", func() {
				platform.SetRotation(0, mgl64.DegToRad(10), 0)
				platform.Set()

				So(testControlNode.commited, ShouldEqual, true)

				// we can't be overly precise due to FPE
				So(testActuators[0].target, ShouldAlmostEqual, 50, 1)
				So(testActuators[0].speed, ShouldAlmostEqual, 0, 1)
				So(testActuators[1].target, ShouldAlmostEqual, 43, 1)
				So(testActuators[1].speed, ShouldAlmostEqual, 255, 1)
				So(testActuators[2].target, ShouldAlmostEqual, 57, 1)
				So(testActuators[1].speed, ShouldAlmostEqual, 255, 1)

				// moving form the centre
			})

			Convey("10º pitch", func() {
				platform.SetRotation(0, 0, mgl64.DegToRad(10))
				platform.Set()

				So(testControlNode.commited, ShouldEqual, true)

				// we can't be overly precise due to FPE
				So(testActuators[0].target, ShouldAlmostEqual, 59, 1)
				So(testActuators[0].speed, ShouldEqual, 255)
				So(testActuators[1].target, ShouldAlmostEqual, 46, 1)
				So(testActuators[1].speed, ShouldAlmostEqual, 113, 1)
				So(testActuators[2].target, ShouldAlmostEqual, 46, 1)
				So(testActuators[2].speed, ShouldAlmostEqual, 113, 1)

			})

			Convey("10º pitch with offset origin by 40mm y", func() {
				// this isn't realistic for real world, but useful for tests
				platform.SetOrigin(0, 50, 0)

				platform.SetRotation(0, 0, mgl64.DegToRad(10))
				platform.Set()

				So(testControlNode.commited, ShouldEqual, true)

				// we can't be overly precise due to FPE
				So(testActuators[0].target, ShouldAlmostEqual, 50, 1)
				So(testActuators[0].speed, ShouldAlmostEqual, 0, 1)
				So(testActuators[1].target, ShouldAlmostEqual, 37, 1)
				So(testActuators[1].speed, ShouldAlmostEqual, 255, 1)
				So(testActuators[2].target, ShouldAlmostEqual, 37, 1)
				So(testActuators[2].speed, ShouldAlmostEqual, 255, 1)

			})
		})
	})
}

func BenchmarkRearfootPlatform_SetRotation(b *testing.B) {
	p := new(KinematicPlatform)

	for n := 0; n < b.N; n++ {
		p.SetRotation(Pi/4, Pi/2, 3*Pi)
	}
}

func BenchmarkKinematicPlatform_SetTranslate(b *testing.B) {
	p := new(KinematicPlatform)

	for n := 0; n < b.N; n++ {
		p.SetTranslation(10, 20, 30)
	}
}

func BenchmarkKinematicPlatform_PerformSet(b *testing.B) {
	config, _ := genActuatorConfig(6, 50, 40, 105)
	p := NewRearfootPlatform(config)
	p.Node = new(TestControlNode)
	p.SetTranslation(10, 20, 30)
	p.SetRotation(Pi/4, Pi/2, 3*Pi)

	for n := 0; n < b.N; n++ {
		p.Set()
	}
}
