package onboard

import (
	"github.com/go-gl/mathgl/mgl64"
	. "math"
)

const (
	mMin = 0
	mMax = 10000
)

type ActuatorConfig struct {
	LowerCoords mgl64.Vec3
	UpperCoords mgl64.Vec3 // only X and Y, Z is calculated
	MinLength   float64
}

type actuatorAction struct {
	target uint8
	speed  uint8
}

func (c *ActuatorConfig) minHeight() float64 {
	displacement := Sqrt(Pow(c.LowerCoords[0]-c.UpperCoords[0], 2) + Pow(c.LowerCoords[1]-c.UpperCoords[1], 2))

	return Sqrt(Pow(c.MinLength, 2)-displacement) + c.LowerCoords[2]
}

type RearfootPlatform struct {
	BasePoints     []mgl64.Vec3 // anchorage points for the actuators
	PlatformPoints []mgl64.Vec3 // achorage points for the actuators at minimum extension
	LE             []int        // leg extension
	minL           []float64
	Rotation       mgl64.Quat // current rotation matrix
	Translation    mgl64.Mat4 // current translation matrix
	Origin         mgl64.Mat4 // translation matrix for the rotation origin
}

func NewRearfootPlatform(config []ActuatorConfig) (rp *RearfootPlatform) {
	rp = &RearfootPlatform{
		BasePoints:     make([]mgl64.Vec3, len(config)),
		PlatformPoints: make([]mgl64.Vec3, len(config)),
		//Rotation: new(RotationMatrix),
		//Translation: new(Coords),
		LE:     make([]int, len(config)),
		minL:   make([]float64, len(config)),
		Origin: mgl64.Translate3D(0, 0, 0),
	}

	for i, c := range config {
		rp.BasePoints[i] = c.LowerCoords
		rp.PlatformPoints[i] = c.UpperCoords
		rp.minL[i] = c.minHeight()
	}

	return
}

// Sets the rotation for the current platform in radians.
func (p *RearfootPlatform) SetRotation(z, y, x float64) {
	p.Rotation = mgl64.AnglesToQuat(z, y, x, mgl64.ZYX)
}

func (p *RearfootPlatform) SetTranslation(x, y, z float64) {
	p.Translation = mgl64.Translate3D(x, y, z)
}

func (p *RearfootPlatform) SetOrigin(x, y, z float64) {
	p.Origin = mgl64.Translate3D(x, y, z)
}

func (p *RearfootPlatform) CalculateActions() (actions []actuatorAction) {
	// create our return value
	actions = make([]actuatorAction, len(p.minL))

	// create a few local copies of useful variables in slightly different formats
	oInv := p.Origin.Inv()
	rotate := p.Rotation.Mat4()
	transform := mgl64.Ident4() // start with identity matrix as a safe baseline

	// apply origin translation, rotate and reverse translation
	transform = transform.Mul4(p.Origin)
	transform = transform.Mul4(rotate)
	transform = transform.Mul4(oInv)

	// apply final translation
	transform = transform.Mul4(p.Translation)

	var maxTarget uint8
	for i := 0; i < len(p.LE); i++ {
		bp := p.BasePoints[i]
		pp := p.PlatformPoints[i]

		tp := mgl64.TransformCoordinate(pp, transform)

		delta := tp.Sub(bp)
		target := uint8(Round(Sqrt(Pow(delta.X(), 2)+Pow(delta.Y(), 2)+Pow(delta.Z(), 2)) - p.minL[i]))
		if target > maxTarget {
			maxTarget = target
		}

		actions[i] = actuatorAction{
			target: target,
		}
	}

	var speedSlope float64
	speedSlope = 255.0 / float64(maxTarget+1)

	for i, a := range actions {
		actions[i].speed = uint8(Round(float64(a.target+1) * speedSlope))
	}

	return actions
}
