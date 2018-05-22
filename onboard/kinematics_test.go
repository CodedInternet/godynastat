package onboard

import (
	"github.com/go-gl/mathgl/mgl64"
	. "github.com/smartystreets/goconvey/convey"
	. "math"
	"testing"
)

func genActuatorConfig(numPoints int, baseRadius, platformRadius, minLegLength float64) (config []ActuatorConfig) {
	config = make([]ActuatorConfig, numPoints)

	minHeight := Sqrt(Pow(minLegLength, 2) - Pow(Abs(baseRadius-platformRadius), 2))

	slice := 2 * Pi / float64(numPoints)
	for i := 0; i < numPoints; i++ {
		f := float64(i)
		angle := slice * f
		c := ActuatorConfig{
			mgl64.Vec3{baseRadius * Sin(angle), baseRadius * Cos(angle), 0},
			mgl64.Vec3{platformRadius * Sin(angle), platformRadius * Cos(angle), minHeight},
			minLegLength,
		}

		config[i] = c
	}

	return
}

func TestRearfootPlatform(t *testing.T) {
	Convey("3dof platform created from radius", t, func() {
		// this test depends entirely on manual calcuation based on these parameters, DO NOT MODIFY
		config := genActuatorConfig(3, 50, 40, 105)
		platform := NewRearfootPlatform(config)
		So(platform, ShouldNotBeNil)
		So(platform.BasePoints, ShouldNotBeNil)
		So(platform.PlatformPoints, ShouldNotBeNil)

		Convey("top and bottom coordinates are calculated correctly", func() {

			baseTest := []mgl64.Vec3{{0, 50, 0}, {43.301, -25, 0}, {-43.301, -25, 0}}
			platformTest := []mgl64.Vec3{{0, 40, 104.523}, {34.641, -20, 104.523}, {-34.641, -20, 104.523}}
			for i := 0; i < 3; i++ {
				So(platform.BasePoints[i][0], ShouldAlmostEqual, baseTest[i][0], .001)
				So(platform.BasePoints[i][1], ShouldAlmostEqual, baseTest[i][1], .001)
				So(platform.BasePoints[i][2], ShouldAlmostEqual, baseTest[i][2], .001)

				So(platform.PlatformPoints[i][0], ShouldAlmostEqual, platformTest[i][0], .001)
				So(platform.PlatformPoints[i][1], ShouldAlmostEqual, platformTest[i][1], .001)
				So(platform.PlatformPoints[i][2], ShouldAlmostEqual, platformTest[i][2], .001)
			}
		})

		Convey("with 0 values, a zero leg extension is reported", func() {
			platform.SetRotation(0, 0, 0)
			platform.SetTranslation(0, 0, 0)
			actions := platform.CalculateActions()

			for i := 0; i < 3; i++ {
				So(actions[i], ShouldResemble, actuatorAction{0, 255})
			}
		})

		Convey("extremes of height are correct", func() {
			platform.SetRotation(0, 0, 0)
			platform.SetTranslation(0, 0, 100)
			actions := platform.CalculateActions()

			for i := 0; i < 3; i++ {
				So(actions[i], ShouldResemble, actuatorAction{100, 255})
			}
		})

		Convey("midrange height is correct", func() {
			platform.SetRotation(0, 0, 0)
			platform.SetTranslation(0, 0, 50)
			actions := platform.CalculateActions()

			for i := 0; i < 3; i++ {
				So(actions[i], ShouldResemble, actuatorAction{50, 255})
			}
		})

		Convey("10º roll", func() {
			platform.SetRotation(0, mgl64.DegToRad(10), 0)
			platform.SetTranslation(0, 0, 50)
			actions := platform.CalculateActions()

			// we can't be overly precise due to FPE
			So(actions[0].target, ShouldAlmostEqual, 50, 1)
			So(actions[1].target, ShouldAlmostEqual, 43, 1)
			So(actions[2].target, ShouldAlmostEqual, 57, 1)

			// moving form the centre
		})

		Convey("10º pitch", func() {
			platform.SetRotation(0, 0, mgl64.DegToRad(10))
			platform.SetTranslation(0, 0, 50)
			actions := platform.CalculateActions()

			// we can't be overly precise due to FPE
			So(actions[0].target, ShouldAlmostEqual, 59, 1)
			So(actions[1].target, ShouldAlmostEqual, 46, 1)
			So(actions[2].target, ShouldAlmostEqual, 46, 1)
		})

		Convey("10º pitch with offset origin by 40mm y", func() {
			// this isn't realistic for real world, but useful for tests
			platform.SetOrigin(0, 50, 0)

			platform.SetRotation(0, 0, mgl64.DegToRad(10))
			platform.SetTranslation(0, 0, 50)
			actions := platform.CalculateActions()

			// we can't be overly precise due to FPE
			So(actions[0].target, ShouldAlmostEqual, 50, 1)
			So(actions[1].target, ShouldAlmostEqual, 37, 1)
			So(actions[2].target, ShouldAlmostEqual, 37, 1)
		})
	})
}

func BenchmarkRearfootPlatform_SetRotation(b *testing.B) {
	p := new(RearfootPlatform)

	for n := 0; n < b.N; n++ {
		p.SetRotation(Pi/4, Pi/2, 3*Pi)
	}
}

func BenchmarkKinematicPlatform_SetTranslate(b *testing.B) {
	p := new(RearfootPlatform)

	for n := 0; n < b.N; n++ {
		p.SetTranslation(10, 20, 30)
	}
}

func BenchmarkKinematicPlatform_CalculateLegExtension(b *testing.B) {
	p := NewRearfootPlatform(genActuatorConfig(6, 50, 40, 105))
	p.SetTranslation(10, 20, 30)
	p.SetRotation(Pi/4, Pi/2, 3*Pi)

	for n := 0; n < b.N; n++ {
		p.CalculateActions()
	}
}
