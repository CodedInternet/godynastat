package comms

import (
	"github.com/CodedInternet/godynastat/onboard"
)

type Centroid struct {
	X, Y float64
}

type StatePayload struct {
	onboard.DynastatState
	Centroids map[string]Centroid
}
