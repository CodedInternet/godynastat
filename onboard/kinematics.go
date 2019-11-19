package onboard

import (
	"fmt"
	"github.com/CodedInternet/godynastat/onboard/hardware"
	"github.com/go-gl/mathgl/mgl64"
	. "math"
)

const (
	mMin = 0
	mMax = 75
)

type KPlatform interface {
	SetRotation(z, y, x float64)
	SetTranslation(x, y, z float64)
	SetOrigin(x, y, z float64)
	SetFRDrop(r float64)
	Set() error
	Home() error
}

type PlatformActuator struct {
	hardware.Actuator
	LowerCoords mgl64.Vec3
	UpperCoords mgl64.Vec3 // only X and Y, Z is calculated by minHeight()
	MinLength   float64
	_cMinHeight float64 // cached value for minHeight()
}

type actuatorAction struct {
	actuator hardware.Actuator
	target   float64
	change   float64
}

func (pa *PlatformActuator) minHeight() float64 {
	if pa._cMinHeight == 0 {
		displacement := Sqrt(Pow(pa.LowerCoords[0]-pa.UpperCoords[0], 2) + Pow(pa.LowerCoords[1]-pa.UpperCoords[1], 2))
		pa._cMinHeight = Sqrt(Pow(pa.MinLength, 2)-displacement) + pa.LowerCoords[2]
	}

	return pa._cMinHeight
}

type KinematicPlatform struct {
	Actuators   []PlatformActuator
	Node        hardware.ControlNode
	Rotation    mgl64.Quat // current rotation matrix
	Translation mgl64.Mat4 // current translation matrix
	Origin      mgl64.Mat4 // translation matrix for the rotation origin

	// first ray options for if len(actuators) == 4
	FROrigin   mgl64.Mat4
	FRRotation mgl64.Quat
}

func NewRearfootPlatform(actuators []PlatformActuator) (rp *KinematicPlatform) {
	rp = &KinematicPlatform{
		Actuators:   actuators,
		Rotation:    mgl64.QuatIdent(),
		Translation: mgl64.Translate3D(0.0, 0.0, 0.0),
		Origin:      mgl64.Translate3D(0.0, 0.0, 0.0),
	}

	return
}

func NewKinematicPlatform(node hardware.ControlNode, actuators []PlatformActuator) (kp *KinematicPlatform) {
	return &KinematicPlatform{
		Node:        node,
		Actuators:   actuators,
		Rotation:    mgl64.QuatIdent(),
		Translation: mgl64.Translate3D(0.0, 0.0, 0.0),
		Origin:      mgl64.Translate3D(0.0, 0.0, 0.0),
	}
}

func (p *KinematicPlatform) GetTranslation() (x, y, z float64) {
	panic("implement me")
}

func (p *KinematicPlatform) GetFRDrop() (r float64) {
	panic("implement me")
}

// Sets the rotation for the current platform in radians.
func (p *KinematicPlatform) SetRotation(z, y, x float64) {
	p.Rotation = mgl64.AnglesToQuat(z, y, x, mgl64.ZYX)
}

// sets the drop angle for the first ray in radians
func (p *KinematicPlatform) SetFRDrop(r float64) {
	p.FRRotation = mgl64.AnglesToQuat(0, 0, r, mgl64.ZYX)
}

func (p *KinematicPlatform) SetTranslation(x, y, z float64) {
	p.Translation = mgl64.Translate3D(x, y, z)
}

func (p *KinematicPlatform) SetOrigin(x, y, z float64) {
	p.Origin = mgl64.Translate3D(x, y, z)
}

func (p *KinematicPlatform) Set() (err error) {
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

	var maxChange float64
	actions := make([]actuatorAction, len(p.Actuators))
	for i, actuator := range p.Actuators {
		lp := actuator.LowerCoords
		up := actuator.UpperCoords

		// if we are working with the 4 acuator version this is the first ray
		// calculate the correct upper position of the rotation
		if i == 3 {
			frTrans := mgl64.Ident4()
			frTrans = frTrans.Mul4(p.FROrigin)
			frTrans = frTrans.Mul4(p.FRRotation.Mat4())
			frTrans = frTrans.Mul4(p.FROrigin.Inv())

			up = mgl64.TransformCoordinate(up, frTrans)
		}

		tp := mgl64.TransformCoordinate(up, transform)

		delta := tp.Sub(lp)
		target := Round(Sqrt(Pow(delta.X(), 2)+Pow(delta.Y(), 2)+Pow(delta.Z(), 2)) - actuator.minHeight())

		if target < 0 {
			return fmt.Errorf("impossible position request, actuator %d require negative position %f0", i, target)
		}

		var action = actuatorAction{
			actuator: actuator,
			target:   target,
		}
		action.change = Abs(action.target - float64(actuator.GetTarget()))

		if action.change > maxChange {
			maxChange = action.change
		}

		actions[i] = action
	}

	var speedSlope float64
	speedSlope = 255.0 / maxChange

	for _, action := range actions {
		speed := uint8(Round(action.change * speedSlope))
		action.actuator.SetTarget(action.target)
		action.actuator.SetSpeed(speed)
	}

	err = p.Node.SetSpeeds()
	if err != nil {
		return
	}
	err = p.Node.SetTargets()

	return
}

func (p *KinematicPlatform) Home() (err error) {
	return p.Node.Home()
}
