package server

import (
	"context"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/scheduler"
	v1 "k8s.io/api/core/v1"
	log "k8s.io/klog/v2"
	extender "k8s.io/kube-scheduler/extender/v1"
)

type Prioritize struct {
	Name   string
	Func   func(pod *v1.Pod, nodeNames []string) (*extender.HostPriorityList, error)
	Config scheduler.ElasticSchedulerConfig
}

func (p Prioritize) Handler(args extender.ExtenderArgs) (*extender.HostPriorityList, error) {
	pod := args.Pod
	nodeNames := *args.NodeNames
	return p.Func(pod, nodeNames)
}

func NewElasticGPUPrioritize(ctx context.Context, config scheduler.ElasticSchedulerConfig) *Prioritize {
	return &Prioritize{
		Name: "ElasticGPUPrioritize",
		Func: func(pod *v1.Pod, nodeNames []string) (*extender.HostPriorityList, error) {
			priorityList := make(extender.HostPriorityList, len(nodeNames))
			sch, err := scheduler.GetResourceScheduler(pod, config.RegisteredSchedulers)
			if err != nil {
				return nil, err
			}

			scores := sch.Score(nodeNames, pod)
			for i, score := range scores {
				priorityList[i] = extender.HostPriority{
					Host:  nodeNames[i],
					Score: int64(score),
				}
			}
			log.Infof("node scores: %v", priorityList)
			return &priorityList, nil
		},
		Config: config,
	}
}
