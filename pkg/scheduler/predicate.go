package scheduler

import (
	"context"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/metrics"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog/v2"
	extender "k8s.io/kube-scheduler/extender/v1"
)

type Predicate struct {
	Name   string
	Func   func(pod *v1.Pod, nodeNames []string, d dealer.Dealer) ([]bool, []error)
	Dealer dealer.Dealer
}

func (p Predicate) Handler(args extender.ExtenderArgs) *extender.ExtenderFilterResult {
	pod := args.Pod
	nodeNames := *args.NodeNames
	canSchedule := make([]string, 0, len(nodeNames))
	canNotSchedule := make(map[string]string)

	can, res := p.Func(pod, nodeNames, p.Dealer)
	for i := 0; i < len(can); i++ {
		if can[i] {
			canSchedule = append(canSchedule, nodeNames[i])
		} else {
			canNotSchedule[nodeNames[i]] = res[i].Error()
		}
	}

	result := extender.ExtenderFilterResult{
		NodeNames:   &canSchedule,
		FailedNodes: canNotSchedule,
		Error:       "",
	}

	return &result
}

func NewNanoGPUPredicate(ctx context.Context, clientset *kubernetes.Clientset, d dealer.Dealer) *Predicate {
	return &Predicate{
		Name: "NanoGPUFilter",
		Func: func(pod *v1.Pod, nodeNames []string, d dealer.Dealer) ([]bool, []error) {
			start := time.Now()
			log.Infof("Check if the pod name %s can be scheduled on nodes %v", pod.Name, nodeNames)
			assume, error := d.Assume(nodeNames, pod)
			metrics.NanoGPUAssumingLatency.Observe(metrics.SinceInSeconds(start))
			return assume, error
		},
		Dealer: d,
	}
}
