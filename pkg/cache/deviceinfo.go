package cache

import (
	"sync"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"

	log "k8s.io/klog/v2"
)

type DeviceInfo struct {
	idx            int
	totalGPUMemory uint
	availGPUMemory uint
	totalGPUCore   uint
	availGPUCore   uint
	podMap         map[string]bool
	mutex          *sync.Mutex
}

func newDeviceInfo(index int, totalMemory uint) *DeviceInfo {
	return &DeviceInfo{
		idx:            index,
		totalGPUMemory: totalMemory,
		availGPUMemory: totalMemory,
		totalGPUCore:   types.GPUCoreEachCard,
		availGPUCore:   types.GPUCoreEachCard,
		podMap:         make(map[string]bool),
		mutex:          new(sync.Mutex),
	}
}

func (d *DeviceInfo) GetTotalGPUCore() uint {
	return d.totalGPUCore
}

func (d *DeviceInfo) GetTotalGPUMemory() uint {
	return d.totalGPUMemory
}

func (d *DeviceInfo) RemovePod(podName string, gpuCore, gpuMemory uint) {
	log.Infof("DeviceInfo deviceID %d available GPU Core %d, GPU Memory %d, Remove Pod %s request GPU Core %d, GPU Memory %d",
		d.idx, d.availGPUCore, d.availGPUMemory, podName, gpuCore, gpuMemory)

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if _, exists := d.podMap[podName]; !exists {
		log.Warningf("Pod %s has not been bound to deviceID %d", podName, d.idx)
		return
	}

	delete(d.podMap, podName)
	if d.availGPUCore+gpuCore <= d.totalGPUCore && d.availGPUMemory+gpuMemory >= d.totalGPUMemory {
		d.availGPUCore += gpuCore
		d.availGPUMemory += gpuMemory
	} else {
		log.Errorf("DeviceInfo DecGPUPercent Fail deviceID %d available GPU Core %d GPU Memory %d, request GPU Core %d GPU Memory %d",
			d.idx, d.availGPUCore, d.availGPUMemory, gpuCore, gpuMemory)
	}
}

func (d *DeviceInfo) AddPod(podName string, gpuCore, gpuMemory uint) {
	log.Infof("DeviceInfo deviceID %d available GPU Core %d, GPU Memory %d, Add Pod %s request GPU Core %d, GPU Memory %d",
		d.idx, d.availGPUCore, d.availGPUMemory, podName, gpuCore, gpuMemory)

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if _, exists := d.podMap[podName]; exists {
		log.Warningf("Pod %s has already been bound to deviceID %d", podName, d.idx)
		return
	}

	d.podMap[podName] = true
	if d.availGPUCore >= gpuCore && d.availGPUMemory >= gpuMemory {
		d.availGPUCore -= gpuCore
		d.availGPUMemory -= gpuMemory
	} else {
		log.Errorf("DeviceInfo DecGPUPercent Fail deviceID %d available GPU Core %d GPU Memory %d, request GPU Core %d GPU Memory %d",
			d.idx, d.availGPUCore, d.availGPUMemory, gpuCore, gpuMemory)
	}
}
