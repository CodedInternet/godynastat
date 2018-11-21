package onboard

import (
	"github.com/go-gl/mathgl/mgl64"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/yaml.v2"
	"testing"
)

const testYaml = `
version: 2
platforms:
  coord_test:
    stdaddr: 0x123
    bus: can0
    actuators:
    - lower: [-1, -2, -3]
      upper: [1, 2, 3]
      min: 42
`

func TestActuatorConfigParsing(t *testing.T) {
	var err error
	var config DynastatConfig

	Convey("parsing is successful", t, func() {
		err = yaml.Unmarshal([]byte(testYaml), &config)
		So(err, ShouldBeNil)

		Convey("actuator coords are set", func() {
			actuator := config.Platforms["coord_test"].Actuators[0]
			So(actuator, ShouldNotBeNil)
			So(actuator.LowerCoords, ShouldResemble, mgl64.Vec3{-1, -2, -3})
		})
	})

}
