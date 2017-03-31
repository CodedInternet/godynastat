package main

import (
	"fmt"
	"github.com/abiosoft/ishell"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

func main() {
	var filename string
	var err error
	if os.Getenv("RESIN") == "1" {
		println("Running on resin")
		filename = "/go/bbb_config.yaml"
	} else {
		filename, err = filepath.Abs("./bbb_config.yaml")
	}
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
	dynastat, err = NewDynastat(config)
	if err != nil {
		panic(fmt.Sprintf("Unable to initialize dynastat: %v", err))
	}

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
			dynastat.Motors[name].Home(config.Motors[name].Cal)
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "state",
		Func: func(c *ishell.Context) {
			c.Println("Getting state")
			c.Printf("#v", dynastat.GetState())
		},
	})
	shell.Start()
}
