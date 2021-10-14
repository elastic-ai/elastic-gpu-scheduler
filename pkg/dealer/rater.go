package dealer

import "math"

const (
	ScoreMin = 0
	ScoreMax = 100
)

type Rater interface {
	Rate(GPUs, *Plan) int
}

type SampleRater struct {
}

func (sr *SampleRater) Rate(GPUs, *Plan) int {
	return ScoreMax
}

type Binpack struct {
}

type Spread struct {
}

func (bp *Binpack) Rate(gpus GPUs, p *Plan) int {
	usage := gpus.Usage()

	return int(usage * 100)
}

// Spread expect to choose the node with less gpu usage
// u: gpu usage of the node [0%, 100%]
// v: variance of gpu usage of the node [0, 1]
// g: transfer gpu number to a percent [0%, 100%]
// spread = 7 * (1 - u) + 2 * (1 - g) + (1 - v)
func (bp *Spread) Rate(gpus GPUs, p *Plan) int {
	set := map[int]struct{}{}
	for _, number := range p.GPUIndexes {
		if number == -1 {
			continue
		}
		set[number] = struct{}{}
	}

	var (
		u = gpus.Usage()
		v = gpus.UsageVariance()
		g = float64(len(set)) / float64(len(gpus))
	)
	return int(7*(1-u) + 2*(1-g) + (1 - v))
}

func Variance(value []float64) float64 {
	if len(value) == 1 {
		return 0.0
	}
	sum := 0.0
	for _, i := range value {
		sum += i
	}
	avg := sum / float64(len(value))
	res := 0.0
	for _, i := range value {
		res += math.Pow(i-avg, 2)
	}
	return res / float64(len(value))
}
