package types

import (
	v1 "k8s.io/api/core/v1"
)

const (
	NodeNameField                      = "spec.nodeName"
	ResourceGPUPercent v1.ResourceName = "nano-gpu/gpu-percent"
	GPUPercentEachCard                 = 100

	GPUAssume                = "nano-gpu/assume"
	AnnotationGPUAssume      = GPUAssume
	LabelGPUAssume           = GPUAssume
	AnnotationGPUContainerOn = "nano-gpu/container-%s"
)

const (
	PriorityBinPack string = "binpack"
	PrioritySpread  string = "spread"
)
