package types

import (
	"k8s.io/api/core/v1"
)

const (
	NodeNameField                     = "spec.nodeName"
	ResourceGPUMemory v1.ResourceName = "nano-gpu.io/gpu-memory"
	ResourceGPUCore   v1.ResourceName = "nano-gpu.io/gpu-core"
	GPUCoreEachCard                   = 100

	GPUAssume                = "nano-gpu.io/gpu"
	AnnotationGPUAssume      = GPUAssume
	LabelGPUAssume           = GPUAssume
	AnnotationGPUContainerOn = "nano-gpu.io/gpu-%s"
)

const (
	PriorityBinPack string = "binpack"
	PrioritySpread  string = "spread"
	PriorityRandom  string = "random"
)
