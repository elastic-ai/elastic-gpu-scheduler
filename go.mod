module elasticgpu.io/elastic-gpu-scheduler

go 1.16

require (
	elasticgpu.io/elastic-gpu v0.0.0
	github.com/julienschmidt/httprouter v1.3.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/kube-scheduler v0.18.0
)

replace elasticgpu.io/elastic-gpu => github.com/elastic-ai/elastic-gpu v0.0.0-20220606065143-94fc37efd8cc
