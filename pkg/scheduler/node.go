package scheduler

import (
	"elasticgpu.io/elastic-gpu-scheduler/pkg/utils"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

type GPUIDs [][]int

type NodeAllocator struct {
	Rater     Rater
	GPUs      GPUs
	Pods      []v1.Pod
	Node      *v1.Node
	allocated map[string]*GPUOption
	CoreName  v1.ResourceName
	MemName   v1.ResourceName
}

func NewNodeAllocator(pods []v1.Pod, node *v1.Node, core v1.ResourceName, mem v1.ResourceName, rater Rater) *NodeAllocator {
	coreAvail := node.Status.Allocatable[core]
	// TODO: GB only
	memAvail := node.Status.Allocatable[mem]
	gpuCount := int(coreAvail.Value() / utils.GPUCoreEachCard)
	gpus := make(GPUs, 0)

	for i := 0; i < gpuCount; i++ {
		gpus = append(gpus, &GPU{
			CoreAvailable:   utils.GPUCoreEachCard,
			CoreTotal:       utils.GPUCoreEachCard,
			MemoryAvailable: int(memAvail.Value()) / gpuCount,
			MemoryTotal:     int(memAvail.Value()) / gpuCount,
		})
	}

	na := &NodeAllocator{
		GPUs:      gpus,
		Rater:     rater,
		allocated: make(map[string]*GPUOption),
		Pods:      pods,
		Node:      node,
		CoreName:  core,
		MemName:   mem,
	}

	for _, pod := range pods {
		na.AddPod(pod)
	}

	return na
}

func (ni *NodeAllocator) Assume(request GPURequest) (GPUIDs, error) {
	key := request.Hash()
	if option, ok := ni.allocated[key]; ok {
		return option.Allocated, nil
	}
	option, err := ni.GPUs.Trade(ni.Rater, request)
	if err != nil {
		return nil, err
	}
	klog.Infof("Assume: %s, option: %#v", key, option)
	ni.allocated[key] = option
	return option.Allocated, nil
}

func (ni *NodeAllocator) Score(request GPURequest) int {
	key := request.Hash()
	option, ok := ni.allocated[key]
	if !ok {
		if ids, _ := ni.Assume(request); len(ids) == 0 {
			return ScoreMin
		}
	}
	return option.Score
}

func (ni *NodeAllocator) Bind(request GPURequest) (ids GPUIDs, err error) {
	key := request.Hash()
	option, ok := ni.allocated[key]
	if !ok {
		return nil, fmt.Errorf("assume %s on %s failed", request, ni.GPUs)
	}
	return option.Allocated, nil
}

//
//func (ni *NodeAllocator) Allocate(request GPURequest) (ids GPUIDs, err error) {
//	defer func() {
//		ni.Clean(request)
//	}()
//	key := request.Hash()
//	option, ok := ni.allocated[key]
//	if !ok {
//		if option, err = ni.GPUs.Trade(ni.Rater, request); err != nil {
//			return nil, fmt.Errorf("assume %s on %s failed", request, ni.GPUs)
//		}
//	}
//	if err := ni.GPUs.Transact(option); err != nil {
//		return nil, err
//	}
//
//	return option.Allocated, nil
//}

//func (ni *NodeAllocator) Release(request GPURequest) error {
//	return ni.GPUs.Cancel(ni.Clean(request))
//}

func (ni *NodeAllocator) ForgetPod(pod v1.Pod) error {
	request := NewGPURequest(&pod, ni.CoreName, ni.MemName)
	option := NewGPUOption(request)
	for i, c := range pod.Spec.Containers {
		if k, ok := pod.Annotations[fmt.Sprintf(utils.AnnotationEGPUContainer, c.Name)]; ok {
			ids := strings.Split(pod.Annotations[k], ",")
			idsInt := make([]int, 0)
			for _, s := range ids {
				id, _ := strconv.Atoi(s)
				idsInt = append(idsInt, id)
			}
			option.Allocated[i] = idsInt
		}
	}
	ni.GPUs.Cancel(option)
	return nil
}

//func (ni *NodeAllocator) Clean(request GPURequest) (option *GPUOption) {
//	option = ni.allocated[request.Hash()]
//	ni.allocated[request.Hash()] = nil
//	return
//}

func (ni *NodeAllocator) AddPod(pod v1.Pod) {
	request := NewGPURequest(&pod, ni.CoreName, ni.MemName)
	option := NewGPUOption(request)
	for i, c := range pod.Spec.Containers {
		if k, ok := pod.Annotations[fmt.Sprintf(utils.AnnotationEGPUContainer, c.Name)]; ok {
			ids := strings.Split(pod.Annotations[k], ",")
			idsInt := make([]int, 0)
			for _, s := range ids {
				id, _ := strconv.Atoi(s)
				idsInt = append(idsInt, id)
			}
			option.Allocated[i] = idsInt
		}
	}

	ni.GPUs.Transact(option)
}

func (ni *NodeAllocator) AllocatedStatus() map[string]*GPUOption {
	return ni.allocated
}
