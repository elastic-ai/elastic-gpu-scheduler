package scheduler

import (
	"elasticgpu.io/elastic-gpu-scheduler/pkg/utils"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type GPUIDs [][]int

type NodeAllocator struct {
	Rater     Rater
	GPUs      GPUs
	podsMap   map[types.UID]*v1.Pod
	Node      *v1.Node
	allocated map[string]*GPUOption
	CoreName  v1.ResourceName
	MemName   v1.ResourceName
}

func NewNodeAllocator(pods []v1.Pod, node *v1.Node, core v1.ResourceName, mem v1.ResourceName, rater Rater) (*NodeAllocator, error) {
	coreAvail := node.Status.Allocatable[core]
	// TODO: GB only
	memAvail := node.Status.Allocatable[mem]
	gpuCount := int(coreAvail.Value() / utils.GPUCoreEachCard)
	if gpuCount == 0 {
		return nil, fmt.Errorf("no gpu available on node %s", node.Name)
	}

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
		podsMap:   make(map[types.UID]*v1.Pod),
		Node:      node,
		CoreName:  core,
		MemName:   mem,
	}

	for i, _ := range pods {
		na.Add(&pods[i], nil)
	}

	klog.V(5).Infof("gpus of node %s: %+v", node.Name, na.GPUs)

	return na, nil
}

func (ni *NodeAllocator) Assume(pod *v1.Pod) (GPUIDs, error) {
	req := NewGPURequest(pod, ni.CoreName, ni.MemName)
	key := req.Hash()
	if option, ok := ni.allocated[key]; ok {
		return option.Allocated, nil
	}
	option, err := ni.GPUs.Trade(ni.Rater, req)
	if err != nil {
		return nil, err
	}
	ni.allocated[key] = option
	return option.Allocated, nil
}

func (ni *NodeAllocator) Score(pod *v1.Pod) int {
	req := NewGPURequest(pod, ni.CoreName, ni.MemName)
	key := req.Hash()
	option, ok := ni.allocated[key]
	if !ok {
		if ids, _ := ni.Assume(pod); len(ids) == 0 {
			return ScoreMin
		}
	}
	return option.Score
}

func (ni *NodeAllocator) Allocate(pod *v1.Pod) (ids GPUIDs, err error) {
	req := NewGPURequest(pod, ni.CoreName, ni.MemName)
	key := req.Hash()
	option, ok := ni.allocated[key]
	if !ok {
		return nil, fmt.Errorf("allocate %s on %s failed", req, ni.GPUs)
	}

	klog.Infof("allocated option: %+v", option)
	if err := ni.Add(pod, option); err != nil {
		return nil, err
	}

	defer func() {
		delete(ni.allocated, key)
	}()
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

func (ni *NodeAllocator) Forget(pod *v1.Pod) error {
	klog.V(5).Infof("start to forget pod: %s, allocated pods cache: %+v", pod.Name, ni.podsMap)
	if _, ok := ni.podsMap[pod.UID]; ok {
		option := NewGPUOptionFromPod(pod, ni.CoreName, ni.MemName)
		klog.V(5).Infof("cancel option %+v on %+v", option, ni.GPUs)
		ni.GPUs.Cancel(option)
		klog.V(5).Infof("gpus details: %+v", ni.GPUs)
		delete(ni.podsMap, pod.UID)
	}

	return nil
}

//func (ni *NodeAllocator) Clean(request GPURequest) (option *GPUOption) {
//	option = ni.allocated[request.Hash()]
//	ni.allocated[request.Hash()] = nil
//	return
//}

func (ni *NodeAllocator) Add(pod *v1.Pod, option *GPUOption) error {
	if _, ok := ni.podsMap[pod.UID]; !ok {
		ni.podsMap[pod.UID] = pod
		if option == nil {
			option = NewGPUOptionFromPod(pod, ni.CoreName, ni.MemName)
		}
		return ni.GPUs.Transact(option)
	}

	return nil
}
