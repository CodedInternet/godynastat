package onboard

import (
	"errors"
	"fmt"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	"github.com/CodedInternet/godynastat/onboard/hardware"
	"github.com/go-gl/mathgl/mgl64"
)

var (
	ErrBadAxis = errors.New("bad axis name")
)

type Dynastat interface {
	SetRotation(platform string, degFront, degInc float64) (err error)
	SetHeight(platform string, height float64) (err error)
	SetFirstRay(platform string, angle float64) (err error)
}

type ActuatorDynastat struct {
	Platforms map[string]KPlatform
	can       map[string]*canbus.CANBus

	setMap map[string]interface{}
}

func NewActuatorDynastat(config DynastatConfig) (d *ActuatorDynastat, err error) {
	d = new(ActuatorDynastat)
	d.can = make(map[string]*canbus.CANBus)

	switch config.Version {
	case 2:
		// create Platforms
		d.Platforms = make(map[string]KPlatform, len(config.Platforms))
		for name, pConf := range config.Platforms {
			var bus *canbus.CANBus
			var node *hardware.MotorControlNode

			// create appropriate node
			bus, err = d.getBus(pConf.Bus)
			if err != nil {
				return
			}

			node, err = hardware.NewControlNode(bus, pConf.StdAddr)
			if err != nil {
				return
			}

			actuators := make([]PlatformActuator, len(pConf.Actuators))

			for mIndex, actuator := range pConf.Actuators {
				actuator.Actuator = &hardware.LinearActuator{
					Node:  node,
					Index: uint8(mIndex + 1), // control boards use 1 based indexing
				}
				actuators[mIndex] = actuator
			}

			d.Platforms[name] = NewKinematicPlatform(node, actuators)
		}

	default:
		err = fmt.Errorf("unable to work with version %d", config.Version)
	}

	return
}

func (d *ActuatorDynastat) SetRotation(platform string, degFront, degInc float64) (err error) {
	p, ok := d.Platforms[platform]
	if !ok {
		return fmt.Errorf("unable to find platform '%s'", platform)
	}

	p.SetRotation(0, mgl64.DegToRad(degFront), mgl64.DegToRad(degInc))
	return p.Set()
}

func (d *ActuatorDynastat) SetHeight(platform string, height float64) (err error) {
	p, ok := d.Platforms[platform]
	if !ok {
		return fmt.Errorf("unable to find platform '%s'", platform)
	}

	p.SetTranslation(0, 0, height)
	return p.Set()
}

func (d *ActuatorDynastat) SetFirstRay(platform string, angle float64) (err error) {
	p, ok := d.Platforms[platform]
	if !ok {
		return fmt.Errorf("unable to find platform '%s'", platform)
	}

	p.SetFRDrop(angle)
	return p.Set()
}

func (d *ActuatorDynastat) HomePlatform(platform string) (err error) {
	p, ok := d.Platforms[platform]
	if !ok {
		return fmt.Errorf("unable to find platform '%s'", platform)
	}

	return p.Home()
}

func (d *ActuatorDynastat) getBus(name string) (bus *canbus.CANBus, err error) {
	bus, ok := d.can[name]
	if !ok {
		// need to create bus
		bus, err = canbus.NewCANBus(name)
		if err != nil {
			return
		}
		d.can[name] = bus
	}

	return
}
