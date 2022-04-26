package scheduler

import (
	"elasticgpu.io/elastic-gpu/api/v1alpha1"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func TestAssume(t *testing.T) {
	pods := generatePods("test-pod-", 4)
	node := &v1.Node{
		Status: v1.NodeStatus{
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1alpha1.ResourceGPUCore:   resource.MustParse("400"),
				v1alpha1.ResourceGPUMemory: resource.MustParse("48"),
			},
		},
	}
	ni, _ := NewNodeAllocator(nil, node, v1alpha1.ResourceGPUCore, v1alpha1.ResourceGPUMemory, &Spread{})
	option, _ := ni.Allocate(&pods[0])
	t.Logf("gpus: %v, allocated: %#v", ni.GPUs, option)
}

func generatePods(namePrefix string, count int) []v1.Pod {
	pods := []v1.Pod{}
	for i := 0; i < count; i++ {
		pod := v1.Pod{Spec: v1.PodSpec{}}
		pod.Name = fmt.Sprintf("%s-%d", namePrefix, i)
		pod.Spec.Containers = []v1.Container{{
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1alpha1.ResourceGPUCore:   resource.MustParse("0"),
					v1alpha1.ResourceGPUMemory: resource.MustParse("4"),
				},
			},
		}}
		pods = append(pods, pod)
	}

	return pods
}
