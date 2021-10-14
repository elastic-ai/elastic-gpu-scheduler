package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"

	v1 "k8s.io/api/core/v1"
	log "k8s.io/klog/v2"
)

// IsCompletedPod determines if the pod is complete
func IsCompletedPod(pod *v1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return true
	}

	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		return true
	}
	return false
}

// IsGPUSharingPod determines if it's the pod for GPU sharing
func IsGPUSharingPod(pod *v1.Pod) bool {
	return GetGPUPercentFromPodResource(pod) > 0
}

// GetGPUIDFromAnnotation gets GPU ID from Annotation
func GetGPUIDFromAnnotation(pod *v1.Pod) (gpuIDs []int) {
	if len(pod.ObjectMeta.Annotations) > 0 {
		value, found := pod.ObjectMeta.Annotations[fmt.Sprintf(types.AnnotationGPUContainerOn, pod.Spec.Containers[0].Name)]
		if found {
			gpuIDStrs := strings.Split(value, ",")
			for _, idStr := range gpuIDStrs {
				id, err := strconv.Atoi(idStr)
				if err != nil {
					log.Warningf("warn: Failed due to %v for pod %s in ns %s", err, pod.Name, pod.Namespace)
				}
				gpuIDs = append(gpuIDs, id)
			}
		}
	}

	return gpuIDs
}

func GetGPUPercentFromPodResource(pod *v1.Pod) (gpuPercent uint) {
	containers := pod.Spec.Containers
	for _, container := range containers {
		if val, ok := container.Resources.Limits[types.ResourceGPUPercent]; ok {
			gpuPercent += uint(val.Value())
		}
	}
	return gpuPercent
}

func arrayToString(array []int, delim string) string {
	return strings.Trim(strings.Join(strings.Fields(fmt.Sprint(array)), ","), "[]")
}

// GetUpdatedPodAnnotationSpec updates pod annotation with devId
func GetUpdatedPodAnnotationSpec(oldPod *v1.Pod, indexes []int) (newPod *v1.Pod) {
	newPod = oldPod.DeepCopy()
	if len(newPod.Labels) == 0 {
		newPod.Labels = map[string]string{}
	}
	if len(newPod.Annotations) == 0 {
		newPod.Annotations = map[string]string{}
	}
	for i, container := range newPod.Spec.Containers {
		newPod.Annotations[fmt.Sprintf(types.AnnotationGPUContainerOn, container.Name)] = strconv.Itoa(indexes[i]) // 1,2,3
	}
	newPod.Annotations[types.AnnotationGPUAssume] = "true"
	newPod.Labels[types.LabelGPUAssume] = "true"
	return newPod
}

func IsAssumed(pod *v1.Pod) bool {
	return pod.ObjectMeta.Annotations[types.AnnotationGPUAssume] == "true"
}

func GetContainerAssignIndex(pod *v1.Pod, containerName string) (int, error) {
	key := fmt.Sprintf(types.AnnotationGPUContainerOn, containerName)
	val, ok := pod.Annotations[key]
	if !ok {
		return 0, fmt.Errorf("pod's annotation %v doesn't contain container %s", pod.Annotations, containerName)
	}
	return strconv.Atoi(val)
}

func GetGPUPercentFromContainer(container *v1.Container) int {
	val, ok := container.Resources.Limits[types.ResourceGPUPercent]
	if !ok {
		return 0
	}
	return int(val.Value())
}
