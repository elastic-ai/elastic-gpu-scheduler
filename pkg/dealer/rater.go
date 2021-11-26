package dealer

import (
	"fmt"
	"math"
	"sort"

	log "k8s.io/klog/v2"
)

const (
	ScoreMin = 0
	ScoreMax = 100
)

type Rater interface {
	Rate(GPUs, *Plan) int
	Choose(GPUs, Demand) ([]int, error)
}

type SampleRater struct {
}

func (sr *SampleRater) Rate(GPUs, *Plan) int {
	return ScoreMax
}

func (sr *SampleRater) Choose(gpus GPUs, d Demand) ([]int, error) {
	indexes := []int{}
	for _, r := range d {
		if r.Percent == 0 {
			indexes = append(indexes, NotNeedGPU)
			continue
		}
		for j := 0; j < len(gpus); j++ {
			if !gpus[j].CanAllocate(r) {
				continue
			}

			indexes = append(indexes, j)
			gpus[j].Percent -= r.Percent
			break
		}
	}

	if len(indexes) != len(d) {
		return nil, fmt.Errorf("unexpected failure of allocation  %s on %s", d, gpus)
	}
	return indexes, nil
}

type Binpack struct {
}

type Spread struct {
}

// binpack will rate higher score to nodes with more usage and less gpus
func (bp *Binpack) Rate(gpus GPUs, p *Plan) int {
	usage := gpus.Usage()

	return int(usage*100) - len(gpus)
}

// binpack will put as much conainters on same gpu card as possible, by
// choosing gpu card with more usage
func (bp *Binpack) Choose(gpus GPUs, d Demand) ([]int, error) {
	indexes := []int{}

	sortableGpus := gpus.ToSortableGPUs()

	// we need to allocate from larger request to avoid allocation failure
	sortableDemand := d.ToSortableGPUs()
	sort.Sort(sortableDemand)
	for j := len(sortableDemand) - 1; j >= 0; j-- {
		if sortableDemand[j].Percent == 0 {
			indexes = append(indexes, NotNeedGPU)
			continue
		}
		sort.Sort(sortableGpus)
		for i := 0; i < len(sortableGpus); i++ {
			if !sortableGpus[i].CanAllocate(*sortableDemand[j].GPUResource) {
				continue
			}
			indexes = append(indexes, sortableGpus[i].index)
			sortableGpus[i].Percent -= sortableDemand[j].Percent
			break
		}
	}

	if len(indexes) != len(d) {
		return nil, fmt.Errorf("can't allocate   %s on %s: indexes=%v", d, gpus, indexes)
	}

	resultIndexs := make([]int, len(d))
	for j := len(d) - 1; j >= 0; j-- {
		resultIndexs[sortableDemand[j].index] = indexes[len(d)-1-j]
	}

	log.Infof("d=%v,sortableDemand=%v, indexes=%v, resultIndexes=%v", d, sortableDemand, indexes, resultIndexs)

	return resultIndexs, nil
}

// Spread expect to choose the node with more free gpu cards, more total available gpu and less gpus
func (sp *Spread) Rate(gpus GPUs, p *Plan) int {
	totalAvailable, freeGpuCount := gpus.PercentAvailableAndFreeGpuCount()

	return 100*freeGpuCount + totalAvailable/10 - len(gpus)
}

// spread will spread conainters accross all gpu cards, by
// choosing gpu card with less  usage
func (sp *Spread) Choose(gpus GPUs, d Demand) ([]int, error) {
	indexes := []int{}

	sortableGpus := gpus.ToSortableGPUs()

	// we need to allocate from larger request to avoid allocation failure
	sortableDemand := d.ToSortableGPUs()
	sort.Sort(sortableDemand)
	for j := len(sortableDemand) - 1; j >= 0; j-- {
		if sortableDemand[j].Percent == 0 {
			indexes = append(indexes, NotNeedGPU)
			continue
		}
		sort.Sort(sortableGpus)
		for i := len(sortableGpus) - 1; i >= 0; i-- {
			if !sortableGpus[i].CanAllocate(*sortableDemand[j].GPUResource) {
				continue
			}
			indexes = append(indexes, sortableGpus[i].index)
			sortableGpus[i].Percent -= sortableDemand[j].Percent
			break
		}
	}

	if len(indexes) != len(d) {
		return nil, fmt.Errorf("can't allocate   %s on %s: indexes=%v", d, gpus, indexes)
	}

	resultIndexs := make([]int, len(d))
	for j := len(d) - 1; j >= 0; j-- {
		resultIndexs[sortableDemand[j].index] = indexes[len(d)-1-j]
	}

	log.Infof("d=%v,sortableDemand=%v, indexes=%v, resultIndexes=%v", d, sortableDemand, indexes, resultIndexs)

	return resultIndexs, nil
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
