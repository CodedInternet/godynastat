package main

import (
	"github.com/abiosoft/ishell"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

func main() {
	filename, _ := filepath.Abs("./bbb_config.yaml")
	if os.Getenv("RESIN") == "1" {
		filename = "/go/bbb_config.yaml"
	}
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(err)
	}

	var config DynastatConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		panic(err)
	}

	var dynastat *Dynastat
	dynastat, err = NewDynastat(config)
	if err != nil {
		panic(err)
	}

	shell := ishell.New()
	shell.Println("Dynastat development shell")
	shell.AddCmd(&ishell.Cmd{
		Name: "move",
		Help: "move <motor> <position (0-255)>",
		Func: func(c *ishell.Context) {
			name := c.Args[0]
			position, _ := strconv.Atoi(c.Args[1])
			c.Printf("Moving motor %s to %d", name, position)
			dynastat.SetMotor(name, position)
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "home",
		Help: "home <motor>",
		Func: func(c *ishell.Context) {
			name := string(c.Args[0])
			c.Printf("Homing motor", name)
			dynastat.Motors[name].Home(config.Motors[name].Cal)
		},
	})
	shell.Start()
}
