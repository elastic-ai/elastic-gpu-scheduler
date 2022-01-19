package dealer

import (
	"errors"
	"k8s.io/klog"
	"strconv"
	"time"
)

type GPUCoreUsage struct {
	CoreUsage    string
	UpdateTime   string
}

type GPUMemoryUsage struct {
	MemoryUsage  string
	UpdateTime   string
}

func NewGPUCoreUsage(coreUsage, updateTime string) GPUCoreUsage {
	return GPUCoreUsage{
		CoreUsage:     coreUsage,
		UpdateTime:    updateTime,
	}
}

func NewGPUMemoryUsage(memoryUsage, updateTime string) GPUMemoryUsage {
	return GPUMemoryUsage{
		MemoryUsage:   memoryUsage,
		UpdateTime:    updateTime,
	}
}

func (d *DealerImpl) GetCoreUsageLock(nodeName string) (map[int]GPUCoreUsage, bool) {
	d.Lock.Lock()
	defer d.Lock.Unlock()
	coreUsage, exist :=  d.CoreUsage[nodeName]
	return coreUsage, exist
}

func (d *DealerImpl) GetMemoryUsageLock(nodeName string) (map[int]GPUMemoryUsage, bool) {
	d.Lock.Lock()
	defer d.Lock.Unlock()
	memoryUsage, exist :=  d.MemoryUsage[nodeName]
	return memoryUsage, exist
}

func (d *DealerImpl) GetCoreUsage(nodeName string) (map[int]GPUCoreUsage, bool) {
	coreUsage, exist :=  d.CoreUsage[nodeName]
	return coreUsage, exist
}

func (d *DealerImpl) GetMemoryUsage(nodeName string) (map[int]GPUMemoryUsage, bool) {
	memoryUsage, exist :=  d.MemoryUsage[nodeName]
	return memoryUsage, exist
}

func (d *DealerImpl) AddCoreUsage(nodeName string)  {
	d.Lock.Lock()
	defer d.Lock.Unlock()
	d.CoreUsage[nodeName] = make(map[int]GPUCoreUsage)
}

func (d *DealerImpl) AddMemoryUsage(nodeName string)  {
	d.Lock.Lock()
	defer d.Lock.Unlock()
	d.MemoryUsage[nodeName] = make(map[int]GPUMemoryUsage)
}

func (d *DealerImpl) UpdateCoreUsage(nodeName, coreUsage, updateTime string, cardNum int)  {
	d.Lock.Lock()
	defer d.Lock.Unlock()
	d.CoreUsage[nodeName][cardNum] = NewGPUCoreUsage(coreUsage, updateTime)
}

func (d *DealerImpl) UpdateMemoryUsage(nodeName, memoryUsage, updateTime string, cardNum int)  {
	d.Lock.Lock()
	defer d.Lock.Unlock()
	d.MemoryUsage[nodeName][cardNum] = NewGPUMemoryUsage(memoryUsage, updateTime)
}

func (d *DealerImpl) GetUsage(nodeName, key string, card int, activeDuration time.Duration) (bool, float64, error) {
	var usage, time string
	if key == GPUCoreUsagePriority {
		_, exist :=  d.GetCoreUsage(nodeName)
		if !exist {
			return exist, 0, nil
		}
		usage = d.CoreUsage[nodeName][card].CoreUsage
		time = d.CoreUsage[nodeName][card].UpdateTime
	} else {
		_, exist :=  d.GetMemoryUsage(nodeName)
		if !exist {
			return exist, 0, nil
		}
		usage = d.MemoryUsage[nodeName][card].MemoryUsage
		time = d.MemoryUsage[nodeName][card].UpdateTime
	}
	if !inUpdateTimePeriod(time, activeDuration) {
		return true, 0, errors.New(key + "not in update period")
	}
	UsedValue, err := strconv.ParseFloat(usage, 64)
	if err != nil {
		return true, 0, errors.New(key + "strconv.ParseFloat error")
	}
	if UsedValue < 0 || UsedValue > 1 {
		klog.Info("UsedValue:",UsedValue)
		return true, 0, errors.New(key + " usage < 0 || usage > 1")
	}
	return true, UsedValue, nil
}
