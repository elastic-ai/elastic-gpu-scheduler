package scheduler

import (
	"context"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/metrics"
	"time"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	extender "k8s.io/kube-scheduler/extender/v1"
)

type Prioritize struct {
	Name string
	Func func(pod *v1.Pod, nodeNames []string) (*extender.HostPriorityList, error)
}

func (p Prioritize) Handler(args extender.ExtenderArgs) (*extender.HostPriorityList, error) {
	pod := args.Pod
	nodeNames := *args.NodeNames
	return p.Func(pod, nodeNames)
}

func NewNanoGPUPrioritize(ctx context.Context, clientset *kubernetes.Clientset, d dealer.Dealer) *Prioritize {
	return &Prioritize{
		Name: "NanoGPUSorter",
		Func: func(pod *v1.Pod, nodeNames []string) (*extender.HostPriorityList, error) {
			start := time.Now()
			var priorityList extender.HostPriorityList
			priorityList = make([]extender.HostPriority, len(nodeNames))
			scores := d.Score(nodeNames, pod)
			for i, score := range scores {
				priorityList[i] = extender.HostPriority{
					Host:  nodeNames[i],
					Score: int64(score),
				}
			}
			metrics.NanoGPUSortingLatency.Observe(metrics.SinceInSeconds(start))
			return &priorityList, nil
		},
	}
}
