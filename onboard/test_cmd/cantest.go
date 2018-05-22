// +build linux

package main

import (
	"fmt"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	"github.com/CodedInternet/godynastat/onboard/hardware"
	"os"
	"strconv"
	"time"
)

func main() {
	var pos, speed byte = 127, 127
	args := os.Args[1:]

	if len(args) >= 1 {
		p, e := strconv.Atoi(args[0])
		if e != nil {
			panic(e)
		}
		pos = byte(p)
	}

	if len(args) >= 2 {
		s, e := strconv.Atoi(args[1])
		if e != nil {
			panic(e)
		}
		speed = byte(s)
	}

	bus, err := canbus.NewCANBus("can0")
	if err != nil {
		panic(err)
	}

	node, err := hardware.NewControlNode(bus, 0x0001)
	if err != nil {
		panic(err)
	}

	cmd, err := node.Send(&hardware.CMDVersion{})
	if err != nil {
		panic(err)
	}

	version := cmd.(*hardware.CMDVersion)

	fmt.Printf("Success! Working with node version %#v\n", version)

	fmt.Println("Staging some movements")

	moveCmd := &hardware.CMDStagePos{
		Position: pos,
		Speed:    speed,
	}

	for i := 1; i <= 3; i++ {
		moveCmd.Index = byte(i)
		_, err = node.Send(moveCmd)
		if err != nil {
			node.StageReset()
			panic(err)
		}
	}

	time.Sleep(time.Second)
	fmt.Println("Committing movement")
	node.StageCommit()
}
