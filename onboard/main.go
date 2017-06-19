package onboard

import (
	"encoding/binary"
	"fmt"
	"github.com/abiosoft/ishell"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

func yamlFilename() (filename string, err error) {
	if os.Getenv("RESIN") == "1" {
		println("Running on resin")
		filename = "/data/bbb_config.yaml"
	} else {
		filename, err = filepath.Abs("./bbb_config.yaml")
	}

	return
}

func main() {
	filename, err := yamlFilename()
	if err != nil {
		panic(fmt.Sprintf("Unable to find file: %v", err))
	}
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(fmt.Sprintf("Unable to read yaml file: %v", err))
	}

	var config DynastatConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		panic(fmt.Sprintf("Unable to unmarshal yaml: %v", err))
	}

	var dynastat *Dynastat
	fmt.Printf("Establishing device with config %#v\n", config)
	dynastat, err = NewDynastat(&config)
	if err != nil {
		panic(fmt.Sprintf("Unable to initialize dynastat: %v", err))
	}

	//conductor := new(Conductor)
	//conductor.device = dynastat
	//
	//for _, wsUrl := range config.SignalingServers {
	//	conductor.AddSignalingServer(wsUrl)
	//}
	//
	//go conductor.UpdateClients()

	shell := ishell.New()
	shell.Println("Dynastat development shell")
	shell.AddCmd(&ishell.Cmd{
		Name: "move",
		Help: "move <Motor> <position (0-255)>",
		Func: func(c *ishell.Context) {
			name := c.Args[0]
			position, _ := strconv.Atoi(c.Args[1])
			c.Printf("Moving Motor %s to %d\n", name, position)
			dynastat.SetMotor(name, position)
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "home",
		Help: "home <Motor>",
		Func: func(c *ishell.Context) {
			name := string(c.Args[0])
			c.Printf("Homing Motor %s\n", name)
			motor := dynastat.Motors[name]
			motor.Home(config.Motors[name].Cal)
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "state",
		Func: func(c *ishell.Context) {
			c.Println("Getting state")
			state, err := dynastat.GetState()
			c.Printf("#v #v", state, err)
		},
	})
	//shell.AddCmd(&ishell.Cmd{
	//	Name: "offer",
	//	Func: func(c *ishell.Context) {
	//		offer := string(c.Args[0])
	//		conductor.ReceiveOffer(offer)
	//	},
	//})
	shell.AddCmd(&ishell.Cmd{
		Name: "control",
		Func: func(c *ishell.Context) {
			buf := make([]byte, 2)
			dynastat.sensorBus.Get(m_CONTROL_ADDRESS, m_CONTROL_REG, buf)
			val := binary.BigEndian.Uint16(buf)
			c.Printf("0x%X\n", val)

			c.Printf("Match: #v\n", val&10 == 0)
		},
	})

	{
		motorNames := func([]string) []string {
			keys := make([]string, len(dynastat.Motors))
			for k := range dynastat.Motors {
				keys = append(keys, k)
			}
			return keys
		}

		calCmd := &ishell.Cmd{
			Name: "cal",
			Help: "calibrate a motor",
		}

		calCmd.AddCmd(&ishell.Cmd{
			Name:      "move",
			Help:      "Move a motor to a specified absolute value",
			Completer: motorNames,
			Func: func(c *ishell.Context) {
				name := c.Args[0]
				position, _ := strconv.Atoi(c.Args[1])

				dynastat.GotoMotorRaw(name, position)
			},
		})

		calCmd.AddCmd(&ishell.Cmd{
			Name:      "write",
			Help:      "Write the current absolute value for a motor",
			Completer: motorNames,
			Func: func(c *ishell.Context) {
				name := c.Args[0]
				position, _ := strconv.Atoi(c.Args[1])

				dynastat.WriteMotorRaw(name, position)
			},
		})

		calCmd.AddCmd(&ishell.Cmd{
			Name:      "low",
			Help:      "Set the current position as the low value for a motor",
			Completer: motorNames,
			Func: func(c *ishell.Context) {
				name := c.Args[0]
				dynastat.RecordMotorLow(name)
			},
		})
		calCmd.AddCmd(&ishell.Cmd{
			Name:      "high",
			Help:      "Set the current position as the high value for a motor",
			Completer: motorNames,
			Func: func(c *ishell.Context) {
				name := c.Args[0]
				dynastat.RecordMotorHigh(name)
			},
		})

		calCmd.AddCmd(&ishell.Cmd{
			Name:      "home",
			Help:      "Locate the home position and record the value in the config",
			Completer: motorNames,
			Func: func(c *ishell.Context) {
				name := c.Args[0]
				reverse, _ := strconv.ParseBool(c.Args[1])

				c.ProgressBar().Indeterminate(true)
				c.ProgressBar().Start()

				dynastat.RecordMotorHome(name, reverse)

				c.ProgressBar().Stop()
			},
		})

		calCmd.AddCmd(&ishell.Cmd{
			Name: "commit",
			Help: "Commit the current config to disk",
			Func: func(c *ishell.Context) {
				filename, _ := yamlFilename()
				yml, _ := yaml.Marshal(dynastat.GetConfig())
				ioutil.WriteFile(filename, yml, 0744)
			},
		})

		shell.AddCmd(calCmd)
	}

	shell.Run()
}
