package utils

const (
	NotNeedGPU      = -1
	NodeNameField   = "spec.nodeName"
	GPUCoreEachCard = 100

	EGPUAssumed                   = "elasticgpu.io/assumed"
	AnnotationEGPUContainerPrefix = "elasticgpu.io/container-"
	AnnotationEGPUContainer       = "elasticgpu.io/container-%s"

	PriorityBinPack string = "binpack"
	PrioritySpread  string = "spread"

	OptimisticLockErrorMsg       = "the object has been modified; please apply your changes to the latest version and try again"
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
)
