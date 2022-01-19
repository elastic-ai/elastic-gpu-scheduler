package dealer

import "time"

const (
	ExtenderAtivePeriod    = 5 * time.Minute
	GPUCoreUsagePriority   = "gpu_core_usage_avg"
	GPUMemoryUsagePriority = "gpu_memory_usage_avg"
)

var (
	timeFormat = "2006-01-02T15:04:05Z"
	loc, _     = time.LoadLocation("Asia/Shanghai")
)

type Policy struct {
	Spec PolicySpec `yaml:"spec"`
}

type PolicySpec struct {
	SyncPeriod []Period          `yaml:"syncPeriod"`
	Priority   []PriorityPolicy  `yaml:"priority"`
}

type Period struct {
	Name   string        `yaml:"name"`
	Period time.Duration `yaml:"period"`
}

type PriorityPolicy struct {
	Name   string  `yaml:"name"`
	Weight float64 `yaml:"weight"`
}
