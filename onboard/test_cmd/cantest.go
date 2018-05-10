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

	var versionCmd hardware.CMDVersion
	versionCmd.SetNode(node)

	resp, err := versionCmd.Process()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Success! Working with node version %s", resp.Data)
}
