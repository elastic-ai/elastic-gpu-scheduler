package dealer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"
)

const (
	NotNeedGPU = -1
)

// GPUResource ─┬─> GPUs
//              └─> Demand ─> Plan

type Plan struct {
	Demand     Demand
	GPUIndexes []int
	Score      int
}

func NewPlanFromPod(pod *v1.Pod) (*Plan, error) {
	if !utils.IsAssumed(pod) {
		return nil, fmt.Errorf("pod %s/%s is not assumed", pod.Namespace, pod.Name)
	}
	plan := &Plan{
		Demand:     make(Demand, len(pod.Spec.Containers)),
		GPUIndexes: make([]int, len(pod.Spec.Containers)),
		Score:      0,
	}
	for i, c := range pod.Spec.Containers {
		plan.Demand[i] = GPUResource{
			Percent: utils.GetGPUPercentFromContainer(&c),
		}
		idx, err := utils.GetContainerAssignIndex(pod, c.Name)
		if err != nil {
			idx = 0
		}
		plan.GPUIndexes[i] = idx
	}

	return plan, nil
}

type Demand []GPUResource

func NewDemandFromPod(pod *v1.Pod) Demand {
	ans := make(Demand, len(pod.Spec.Containers))
	for i, container := range pod.Spec.Containers {
		ans[i] = GPUResource{
			Percent: utils.GetGPUPercentFromContainer(&container),
		}
	}
	return ans
}

func (d *Demand) String() string {
	buffer := bytes.Buffer{}
	for _, resource := range *d {
		buffer.Write([]byte(resource.String()))
	}
	return buffer.String()
}

func (d *Demand) Hash() string {
	to := func(bs [32]byte) []byte { return bs[0:32] }
	return hex.EncodeToString(to(sha256.Sum256([]byte(d.String()))))[0:8]
}

func (d *Demand) ToSortableGPUs() SortableGPUs {
	sortableGpus := make(SortableGPUs, 0)
	for i, gpu := range *d {
		sortableGpu := &GPUResourceWithIndex{
			GPUResource: &GPUResource{gpu.Percent, gpu.PercentTotal},
			index:       i,
		}
		sortableGpus = append(sortableGpus, sortableGpu)
	}

	return sortableGpus
}

type GPUs []*GPUResource

func (g GPUs) Choose(demand Demand, rater Rater) (ans *Plan, err error) {
	ans = &Plan{
		Demand: demand,
	}
	ans.Score = rater.Rate(g, ans)
	ans.GPUIndexes, err = rater.Choose(g, demand)

	return
}

func (g GPUs) Allocate(plan *Plan) error {
	for i := 0; i < len(plan.GPUIndexes); i++ {
		// no gpu needed
		if plan.GPUIndexes[i] < 0 {
			continue
		}
		if !g[plan.GPUIndexes[i]].CanAllocate(plan.Demand[i]) {
			// restore
			for j := 0; j < i; j++ {
				g[plan.GPUIndexes[j]].Add(plan.Demand[i])
			}
			return fmt.Errorf("can't apply plan %v on %s", plan, g)
		}
		g[plan.GPUIndexes[i]].Sub(plan.Demand[i])
	}
	return nil
}

func (g GPUs) Release(plan *Plan) error {
	for i := 0; i < len(plan.Demand); i++ {
		if plan.GPUIndexes[i] < 0 {
			continue
		}
		if plan.GPUIndexes[i] >= len(g) {
			return fmt.Errorf("allocate plan's GPU index %d bigger then GPU resource", plan.GPUIndexes[i])
		}
		g[plan.GPUIndexes[i]].Add(plan.Demand[i])
	}
	return nil
}

func (g GPUs) String() string {
	buffer := bytes.Buffer{}
	for _, resource := range g {
		buffer.Write([]byte(resource.String()))
	}
	return buffer.String()
}

type GPUResource struct {
	Percent      int
	PercentTotal int
}

func (g GPUResource) String() string {
	return fmt.Sprintf("(%d)", g.Percent)
}

func (g *GPUResource) Add(resource GPUResource) {
	g.Percent += resource.Percent
}

func (g *GPUResource) Sub(resource GPUResource) {
	g.Percent -= resource.Percent
}

func (g *GPUResource) CanAllocate(resource GPUResource) bool {
	return g.Percent >= resource.Percent
}

// return gpu usage of current node, [0%, 100%]
func (gpus GPUs) Usage() float64 {
	percentSum, percentUsed := 0, 0
	for _, r := range gpus {
		percentSum += r.PercentTotal
		percentUsed += r.PercentTotal - r.Percent
	}
	return float64(percentUsed) / float64(percentSum)
}

func (gpus GPUs) PercentUsed() int {
	totalPercentUsed := 0
	for _, r := range gpus {
		totalPercentUsed += r.PercentTotal - r.Percent
	}
	return totalPercentUsed
}

func (gpus GPUs) PercentAvailableAndFreeGpuCount() (totalAvailable int, freeGpuCount int) {
	for _, g := range gpus {
		totalAvailable += g.Percent
		if g.Percent == g.PercentTotal {
			freeGpuCount++
		}
	}
	return
}

func (gpus GPUs) UsageVariance() float64 {
	var (
		percentUsages = []float64{}
	)
	for _, r := range gpus {
		percentUsages = append(percentUsages, (float64(r.PercentTotal)-float64(r.Percent))/float64(r.PercentTotal))
	}
	return Variance(percentUsages)
}

func (gpus GPUs) ToSortableGPUs() SortableGPUs {
	sortableGpus := make(SortableGPUs, 0)
	for i, gpu := range gpus {
		sortableGpu := &GPUResourceWithIndex{
			GPUResource: &GPUResource{gpu.Percent, gpu.PercentTotal},
			index:       i,
		}
		sortableGpus = append(sortableGpus, sortableGpu)
	}

	return sortableGpus
}

type GPUResourceWithIndex struct {
	*GPUResource
	index int
}

type SortableGPUs []*GPUResourceWithIndex

func (g SortableGPUs) Len() int           { return len(g) }
func (g SortableGPUs) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }
func (g SortableGPUs) Less(i, j int) bool { return g[i].Percent < g[j].Percent }
