package scheduler

import (
	"context"
	"fmt"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog/v2"
	extender "k8s.io/kube-scheduler/extender/v1"
)

// Bind is responsible for binding node and pod
type Bind struct {
	Name   string
	Func   func(podName string, podNamespace string, podUID types.UID, node string, d dealer.Dealer) error
	Dealer dealer.Dealer
}

// Handler handles the Bind request
func (b Bind) Handler(args extender.ExtenderBindingArgs) *extender.ExtenderBindingResult {
	err := b.Func(args.PodName, args.PodNamespace, args.PodUID, args.Node, b.Dealer)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return &extender.ExtenderBindingResult{
		Error: errMsg,
	}
}

func NewNanoGPUBind(ctx context.Context, clientset *kubernetes.Clientset, d dealer.Dealer) *Bind {
	return &Bind{
		Name: "NanoGPUBinder",
		Func: func(name string, namespace string, podUID types.UID, node string, d dealer.Dealer) error {
			pod, err := getPod(ctx, name, namespace, podUID, clientset)
			if err != nil {
				log.Warningf("warn: Failed to handle pod %s in ns %s due to error %v", name, namespace, err)
				return err
			}
			if utils.IsCompletedPod(pod) {
				err = fmt.Errorf("pod %s/%s already deleted or completed", name, namespace)
				log.Warningf("warn: Failed to handle pod %s in ns %s due to error %v", name, namespace, err)
				return err
			}

			err = d.Bind(node, pod)
			d.PrintStatus(pod, "bind")

			return err
		},
		Dealer: d,
	}
}

func getPod(ctx context.Context, name string, namespace string, podUID types.UID, clientset *kubernetes.Clientset) (pod *v1.Pod, err error) {
	pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if pod.UID != podUID {
		pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if pod.UID != podUID {
			return nil, fmt.Errorf("pod %s in ns %s's uid is %v, and it's not equal with expected %v",
				name,
				namespace,
				pod.UID,
				podUID)
		}
	}

	return pod, nil
}
