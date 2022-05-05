package scheduler

import (
	"bytes"
	"crypto/sha256"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/utils"
	"encoding/hex"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

const (
	NotNeedGPU  = -1
	NotNeedRate = -2
)

type GPURequest []GPUUnit

func (d GPURequest) String() string {
	buffer := bytes.Buffer{}
	for _, r := range d {
		buffer.Write([]byte(r.String()))
	}
	return buffer.String()
}

func (d GPURequest) Hash() string {
	to := func(bs [32]byte) []byte { return bs[0:32] }
	return hex.EncodeToString(to(sha256.Sum256([]byte(d.String()))))[0:8]
}

func NewGPURequest(pod *v1.Pod, core v1.ResourceName, mem v1.ResourceName) GPURequest {
	request := make([]GPUUnit, len(pod.Spec.Containers))
	for i, c := range pod.Spec.Containers {
		core := GetGPUCoreFromContainer(&c, core)
		mem := GetGPUMemoryFromContainer(&c, mem)
		klog.V(5).Infof("container %s core: %d, memory: %d", c.Name, core, mem)
		if core == 0 && mem == 0 {
			request[i].Core = NotNeedGPU
			request[i].Memory = NotNeedGPU
			continue
		}
		if core >= utils.GPUCoreEachCard {
			request[i].GPUCount = core / utils.GPUCoreEachCard
			continue
		}
		request[i] = GPUUnit{
			Core:   core,
			Memory: mem,
		}
	}

	klog.V(5).Infof("pod %s gpu request: %+v", pod.Name, request)
	return request
}

type GPUOption struct {
	Request   GPURequest
	Allocated [][]int
	Score     int
}

func NewGPUOption(request GPURequest) *GPUOption {
	opt := &GPUOption{
		Request:   request,
		Allocated: make([][]int, len(request)),
		Score:     0,
	}
	return opt
}

func NewGPUOptionFromPod(pod *v1.Pod, core v1.ResourceName, mem v1.ResourceName) *GPUOption {
	request := NewGPURequest(pod, core, mem)
	option := NewGPUOption(request)
	for i, c := range pod.Spec.Containers {
		if v, ok := pod.Annotations[fmt.Sprintf(utils.AnnotationEGPUContainer, c.Name)]; ok {
			klog.V(5).Infof("container %s gpu key: %s", c.Name, v)
			ids := strings.Split(v, ",")
			idsInt := make([]int, 0)
			for _, s := range ids {
				id, _ := strconv.Atoi(s)
				idsInt = append(idsInt, id)
			}
			option.Allocated[i] = idsInt
		}
	}
	klog.V(5).Infof("pod %s/%s allocated gpu: %d", pod.Namespace, pod.Name, option.Allocated)

	return option
}
