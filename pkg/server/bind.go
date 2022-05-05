package server

import (
	"context"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/scheduler"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	log "k8s.io/klog/v2"
	extender "k8s.io/kube-scheduler/extender/v1"
)

// Bind is responsible for binding node and pod
type Bind struct {
	Name   string
	Func   func(podName string, podNamespace string, podUID types.UID, node string) error
	Config scheduler.ElasticSchedulerConfig
}

// Handler handles the Bind request
func (b Bind) Handler(args extender.ExtenderBindingArgs) *extender.ExtenderBindingResult {
	err := b.Func(args.PodName, args.PodNamespace, args.PodUID, args.Node)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return &extender.ExtenderBindingResult{
		Error: errMsg,
	}
}

func NewElasticGPUBind(ctx context.Context, config scheduler.ElasticSchedulerConfig) *Bind {
	return &Bind{
		Name: "ElasticGPUBinder",
		Func: func(name string, namespace string, podUID types.UID, node string) error {
			pod, err := scheduler.GetPod(ctx, name, namespace, podUID, config.Clientset)
			if err != nil {
				log.Warningf("warn: Failed to handle pod %s in ns %s due to error %v", name, namespace, err)
				return err
			}
			if scheduler.IsCompletedPod(pod) {
				err = fmt.Errorf("pod %s/%s already deleted or completed", name, namespace)
				log.Warningf("warn: Failed to handle pod %s in ns %s due to error %v", name, namespace, err)
				return err
			}
			sch, err := scheduler.GetResourceScheduler(pod, config.RegisteredSchedulers)
			if err != nil {
				return err
			}

			return sch.Bind(node, pod)
		},
		Config: config,
	}
}
