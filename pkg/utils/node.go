package utils

import (
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	v1 "k8s.io/api/core/v1"
)

func GetGPUDeviceCountOfNode(node *v1.Node) int {
	val, ok := node.Status.Capacity[types.ResourceGPUPercent]
	if !ok {
		return 0
	}
	return int(val.Value()) / types.GPUPercentEachCard
}
