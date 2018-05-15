// +build linux

package main

import (
	"fmt"
	"github.com/CodedInternet/godynastat/onboard/canbus"
	"github.com/CodedInternet/godynastat/onboard/hardware"
)

func main() {
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
}
