package server

import (
	"context"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/scheduler"
	v1 "k8s.io/api/core/v1"
	extender "k8s.io/kube-scheduler/extender/v1"
)

type Predicate struct {
	Name       string
	Schedulers map[v1.ResourceName]scheduler.ResourceScheduler
	Config     scheduler.ElasticSchedulerConfig
}

func (p Predicate) Handler(args extender.ExtenderArgs) *extender.ExtenderFilterResult {
	pod := args.Pod
	nodeNames := *args.NodeNames
	sch, err := scheduler.GetResourceScheduler(args.Pod, p.Config.RegisteredSchedulers)
	if err != nil {
		return &extender.ExtenderFilterResult{
			Error: err.Error(),
		}
	}

	filterdNodes, faildNodes, err := sch.Assume(nodeNames, pod)
	if err != nil {
		return &extender.ExtenderFilterResult{
			Error: err.Error(),
		}
	}

	result := extender.ExtenderFilterResult{
		NodeNames:   &filterdNodes,
		FailedNodes: faildNodes,
		Error:       "",
	}

	return &result
}

func NewElasticGPUPredicate(ctx context.Context, config scheduler.ElasticSchedulerConfig) *Predicate {
	return &Predicate{Name: "ElasticGPUPredicate", Config: config}
}
