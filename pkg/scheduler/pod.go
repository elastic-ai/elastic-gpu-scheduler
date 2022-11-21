package scheduler

import (
	"context"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/utils"
	"elasticgpu.io/elastic-gpu/apis/elasticgpu/v1alpha1"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
)

func IsCompletedPod(pod *v1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return true
	}

	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		return true
	}
	return false
}

func IsGPUPod(pod *v1.Pod) bool {
	if GetResourceRequests(pod, v1alpha1.ResourceGPUCore) > 0 || GetResourceRequests(pod, v1alpha1.ResourceGPUMemory) > 0 {
		return true
	}

	if GetResourceRequests(pod, v1alpha1.ResourceQGPUCore) > 0 || GetResourceRequests(pod, v1alpha1.ResourceQGPUMemory) > 0 {
		return true
	}

	if GetResourceRequests(pod, v1alpha1.ResourcePGPU) > 0 {
		return true
	}

	return false
}

func GetResourceRequests(pod *v1.Pod, resourceName v1.ResourceName) uint {
	containers := pod.Spec.Containers
	requests := uint(0)
	for _, container := range containers {
		if val, ok := container.Resources.Limits[resourceName]; ok {
			requests += uint(val.Value())
		}
	}
	return requests
}

// GetUpdatedPodAnnotationSpec updates pod annotation with devId
func GetUpdatedPodAnnotationSpec(oldPod *v1.Pod, ids [][]int) (newPod *v1.Pod) {
	newPod = oldPod.DeepCopy()
	if len(newPod.Labels) == 0 {
		newPod.Labels = map[string]string{}
	}
	if len(newPod.Annotations) == 0 {
		newPod.Annotations = map[string]string{}
	}
	for i, container := range newPod.Spec.Containers {
		if ids[i][0] == NotNeedGPU {
			continue
		}
		var idsStr []string
		for _, id := range ids[i] {
			idsStr = append(idsStr, strconv.Itoa(id))
		}
		newPod.Annotations[fmt.Sprintf(utils.AnnotationEGPUContainer, container.Name)] = strings.Join(idsStr, ",")
	}
	newPod.Annotations[utils.EGPUAssumed] = "true"
	newPod.Labels[utils.EGPUAssumed] = "true"
	return newPod
}

func IsAssumed(pod *v1.Pod) bool {
	return pod.ObjectMeta.Annotations[utils.EGPUAssumed] == "true"
}

func GetContainerAssignIndex(pod *v1.Pod, containerName string) ([]string, error) {
	key := fmt.Sprintf(utils.AnnotationEGPUContainer, containerName)
	indexArray, ok := pod.Annotations[key]
	if !ok {
		return nil, fmt.Errorf("pod's annotation %v doesn't contain container %s", pod.Annotations, containerName)
	}
	indexStr := strings.Split(indexArray, ",")
	return indexStr, nil
}

func GetGPUCoreFromContainer(container *v1.Container, resource v1.ResourceName) int {
	val, ok := container.Resources.Requests[resource]
	if !ok {
		return 0
	}
	return int(val.Value())
}

func GetGPUMemoryFromContainer(container *v1.Container, resource v1.ResourceName) int {
	val, ok := container.Resources.Requests[resource]
	if !ok {
		return 0
	}
	return int(val.Value())
}

func GetPod(ctx context.Context, name string, namespace string, podUID types.UID, clientset *kubernetes.Clientset) (pod *v1.Pod, err error) {
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

func GetContainerGPUResource(pod v1.Pod) map[string]GPUUnit {
	maps := make(map[string]GPUUnit)
	for _, container := range pod.Spec.Containers {
		if container.Name == "" {
			continue
		}
		unit := GPUUnit{}
		core1, _ := container.Resources.Requests[v1alpha1.ResourceGPUCore]
		core2, _ := container.Resources.Requests[v1alpha1.ResourceQGPUCore]
		if int(core1.Value()) >= utils.GPUCoreEachCard || int(core2.Value()) >= utils.GPUCoreEachCard {
			unit.GPUCount += int(core1.Value())/utils.GPUCoreEachCard + int(core2.Value())/utils.GPUCoreEachCard
			continue
		}
		unit.Core += int(core1.Value() + core2.Value())
		mem1, _ := container.Resources.Requests[v1alpha1.ResourceGPUMemory]
		mem2, _ := container.Resources.Requests[v1alpha1.ResourceQGPUMemory]
		unit.Memory += int(mem1.Value() + mem2.Value())
		maps[container.Name] = unit
	}

	return maps
}
