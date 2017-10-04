package calcs

import "github.com/CodedInternet/godynastat/onboard"

func calculateCentroid(vals []int) float64 {
	var sum, X float64

	for i, val := range vals {
		sum += float64(val)
		X += float64(val * i)
	}

	return ((1 / sum) * X) / float64(len(vals))
}

func SensorCentroid(readings onboard.SensorState) (x, y float64) {
	sumRows := make([]int, len(readings))
	sumCols := make([]int, len(readings[0]))

	for ix, row := range readings {
		for iy, cell := range row {
			sumCols[iy] += cell
			sumRows[ix] += cell
		}
	}

	x = calculateCentroid(sumCols)
	y = calculateCentroid(sumRows)
	return
}
