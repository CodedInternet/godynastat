package onboard

import "github.com/go-gl/mathgl/mgl64"

type DynastatConfig struct {
	Version          int
	SignalingServers []string
	Platforms        map[string]struct {
		StdAddr   uint32
		Bus       string
		Actuators []PlatformActuator
	}
}

type YAMLActuator struct {
	LowerCoords []float64 `yaml:"lower,flow"`
	UpperCoords []float64 `yaml:"upper,flow"` // only X and Y, Z is calculated by minHeight()
	MinLength   float64   `yaml:"min"`
}

func (pa PlatformActuator) MarshalYAML() (*YAMLActuator, error) {
	return &YAMLActuator{
		[]float64{pa.LowerCoords.X(), pa.LowerCoords.Y(), pa.LowerCoords.Z()},
		[]float64{pa.UpperCoords.X(), pa.UpperCoords.Y(), pa.UpperCoords.Z()},
		pa.MinLength,
	}, nil
}

func (pa *PlatformActuator) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var ya YAMLActuator
	if err := unmarshal(&ya); err != nil {
		return err
	}
	pa.LowerCoords = mgl64.Vec3{ya.LowerCoords[0], ya.LowerCoords[1], ya.LowerCoords[2]}
	pa.UpperCoords = mgl64.Vec3{ya.UpperCoords[0], ya.UpperCoords[1], ya.UpperCoords[2]}
	pa.MinLength = ya.MinLength
	return nil
}
