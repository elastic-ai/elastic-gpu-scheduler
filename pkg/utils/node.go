package utils

import (
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	"k8s.io/api/core/v1"
)

func GetTotalGPUMemory(node *v1.Node) int {
	val, ok := node.Status.Capacity[types.ResourceGPUMemory]

	if !ok {
		return 0
	}

	return int(val.Value())
}

func GetGPUDeviceCountOfNode(node *v1.Node) int {
	val, ok := node.Status.Capacity[types.ResourceGPUCore]
	if !ok {
		return 0
	}
	return int(val.Value()) / types.GPUCoreEachCard
}
