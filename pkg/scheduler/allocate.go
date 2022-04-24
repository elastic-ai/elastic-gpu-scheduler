package scheduler

import (
	"bytes"
	"crypto/sha256"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/utils"
	"encoding/hex"
	v1 "k8s.io/api/core/v1"
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
