package scheduler

import (
	"encoding/json"
	"fmt"
	"k8s.io/klog/v2"
)

type GPUUnit struct {
	Core     int
	Memory   int
	GPUCount int
}

func (g *GPUUnit) String() string {
	return fmt.Sprintf("(core: %d, memory: %d, gpu count: %d)", g.Core, g.Memory, g.GPUCount)
}

type GPU struct {
	CoreAvailable   int
	MemoryAvailable int
	CoreTotal       int
	MemoryTotal     int
	//GPUUnits        []GPUUnit
}

//func (g *GPU) String() string {
//	return fmt.Sprintf("(%d, %d, %d)", g.CoreAvailable, g.MemoryAvailable, len(g.GPUUnits))
//}

func (g *GPU) Add(resource GPUUnit) {
	if resource.GPUCount > 0 {
		g.CoreAvailable = 0
		g.MemoryAvailable = 0
	} else {
		g.CoreAvailable -= resource.Core
		g.MemoryAvailable -= resource.Memory
	}
}

func (g *GPU) Sub(resource GPUUnit) {
	if resource.GPUCount > 0 {
		g.CoreAvailable = g.CoreTotal
		g.MemoryAvailable = g.MemoryTotal
	} else {
		g.CoreAvailable += resource.Core
		g.MemoryAvailable += resource.Memory
	}
}

func (g *GPU) CanAllocate(resource GPUUnit) bool {
	if resource.GPUCount > 0 {
		return g.CoreAvailable == g.CoreTotal && g.MemoryAvailable == g.MemoryTotal
	}
	return g.CoreAvailable >= resource.Core && g.MemoryAvailable >= resource.Memory
}

type GPUs []*GPU

func (g GPUs) String() string {
	r, _ := json.Marshal(g)
	return string(r)
}

func (g GPUs) Trade(rater Rater, request GPURequest) (option *GPUOption, err error) {
	var (
		dfs     func(i int)
		indexes = make([][]int, len(request))
		found   = false
	)
	option = NewGPUOption(request)
	dfs = func(containerIndex int) {
		if containerIndex == len(request) {
			found = true
			currScore := 0
			rateInexes := make([]int, len(indexes))
			for i := range indexes {
				if len(indexes[i]) == 1 {
					rateInexes[i] = indexes[i][0]
				} else {
					rateInexes[i] = NotNeedRate
				}
			}
			currScore = rater.Rate(g, rateInexes)
			if option.Score > currScore {
				return
			}
			for i, gpuIndex := range indexes {
				option.Allocated[i] = gpuIndex
			}
			option.Score = currScore
			return
		}
		klog.V(5).Infof("Start to allocate request on %d container: %+v, current gpus: %+v", containerIndex, request[containerIndex], g)
		if request[containerIndex].GPUCount > 0 {
			freeGPUs := g.GetFreeGPUs()
			if len(freeGPUs) < request[containerIndex].GPUCount {
				return
			}
			indexes[containerIndex] = freeGPUs[:request[containerIndex].GPUCount]
			for _, gpuIndex := range indexes[containerIndex] {
				g[gpuIndex].Add(request[containerIndex])
			}
			dfs(containerIndex + 1)
			for _, gpuIndex := range indexes[containerIndex] {
				g[gpuIndex].Sub(request[containerIndex])
			}
			return
		}
		for i, gpu := range g {
			if !gpu.CanAllocate(request[containerIndex]) {
				klog.Infof("Can't allocate request of %d container: %+v, current gpu: %+v", containerIndex, request[containerIndex], gpu)
				continue
			}
			klog.Infof("Start to allocate request of %d container: %+v, current gpu: %+v", containerIndex, request[containerIndex], gpu)
			gpu.Add(request[containerIndex])
			indexes[containerIndex] = make([]int, 1)
			indexes[containerIndex][0] = i
			dfs(containerIndex + 1)
			gpu.Sub(request[containerIndex])

		}
	}
	dfs(0)
	if !found {
		return nil, fmt.Errorf("no enough resource to allocate")
	}
	return option, nil
}

//func (gpus GPUs) CoreUsage() float64 {
//	coreUsed, coreAvailable := 0, 0
//	for _, g := range gpus {
//		for _, r := range g.GPUUnits {
//			coreUsed += r.Core
//		}
//		coreAvailable += g.CoreAvailable
//	}
//	return float64(coreUsed) / (float64(coreUsed+coreAvailable+1) + 0.1)
//}

//func (gpus GPUs) MemoryUsage() float64 {
//	memUsed, memAvailable := 0, 0
//	for _, g := range gpus {
//		for _, r := range g.GPUUnits {
//			memUsed += r.Memory
//		}
//		memAvailable += g.MemoryAvailable
//	}
//	return float64(memUsed) / (float64(memUsed+memAvailable) + 0.1)
//}

func (g GPUs) Transact(option *GPUOption) error {
	klog.V(5).Infof("GPU %+v transacts %+v", g, option)
	for i := 0; i < len(option.Allocated); i++ {
		if option.Request[i].GPUCount > 0 {
			for j := 0; j < len(option.Allocated[i]); j++ {
				if !g[option.Allocated[i][j]].CanAllocate(option.Request[i]) {
					klog.Errorf("Fail to trade option %+v on %s because the GPU's residual memory or core can't satisfy the container", option, g)
					return fmt.Errorf("can't trade option %+v on %+v because the GPU's residual memory or core can't satisfy the container", option, g)
				}
				g[option.Allocated[i][j]].Add(option.Request[i])
			}
		} else {
			if len(option.Allocated[i]) > 0 {
				if !g[option.Allocated[i][0]].CanAllocate(option.Request[i]) {
					klog.Errorf("Fail to trade option %+v on %+v because the GPU's residual memory or core can't satisfy the container", option, g)
					return fmt.Errorf("can't trade option %+v on %+v because the GPU's residual memory or core can't satisfy the container", option, g)
				}
				g[option.Allocated[i][0]].Add(option.Request[i])
			}
		}
	}
	return nil
}

func (g GPUs) Cancel(option *GPUOption) error {
	klog.V(5).Infof("Cancel option %+v on GPU %+v", option, g)
	for i := 0; i < len(option.Request); i++ {
		if option.Request[i].GPUCount > 0 {
			for _, gpuIndex := range option.Allocated[i] {
				g[gpuIndex].Sub(option.Request[i])
			}
		} else {
			if len(option.Allocated[i]) > 0 {
				g[option.Allocated[i][0]].Sub(option.Request[i])
			}
		}
	}
	return nil
}

func (g GPUs) GetFreeGPUs() []int {
	indexes := make([]int, 0)
	for i := 0; i < len(g); i++ {
		if g[i].CoreAvailable == g[i].CoreTotal && g[i].MemoryAvailable == g[i].MemoryTotal {
			indexes = append(indexes, i)
		}
	}

	return indexes
}
