package scheduler

const (
	ScoreMin = 0
	ScoreMax = 10
)

type Rater interface {
	Rate(g GPUs, indexes []int) int
}

type SampleRater struct {
}

type Binpack struct {
}

func (bp *Binpack) Rate(g GPUs, indexes []int) int {
	gpuIndex := make([]int, len(g))
	gpuCount := 0
	for _, g := range indexes {
		if g < 0 {
			continue
		}
		if gpuIndex[g] == 0 {
			gpuIndex[g]++
			gpuCount++
		}
	}
	maxMemoryLeft := g[0].MemoryAvailable
	minMemoryLeft := g[0].MemoryAvailable
	maxCoreLeft := g[0].CoreAvailable
	minCoreLeft := g[0].CoreAvailable
	for _, gpu := range g {
		if gpu.MemoryAvailable > maxMemoryLeft {
			maxMemoryLeft = gpu.MemoryAvailable
		}
		if gpu.MemoryAvailable < minMemoryLeft {
			minMemoryLeft = gpu.MemoryAvailable
		}
		if gpu.CoreAvailable > maxCoreLeft {
			maxCoreLeft = gpu.CoreAvailable
		}
		if gpu.CoreAvailable < minCoreLeft {
			minCoreLeft = gpu.CoreAvailable
		}
	}
	Range := (maxMemoryLeft + maxCoreLeft - minMemoryLeft - minCoreLeft) / 2
	res := Range / (gpuCount + 1) * 100
	return res
}

type Spread struct {
}

func (s *Spread) Rate(g GPUs, indexes []int) int {
	// TODO
	return 0
}
