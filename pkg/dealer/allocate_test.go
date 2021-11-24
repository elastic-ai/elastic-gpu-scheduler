package dealer

import (
	"fmt"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
)

func TestGPUResource(t *testing.T) {
	type Cases struct {
		Origin GPUResource
		Target GPUResource
		Expect GPUResource
	}
	subTests := []Cases{
		{
			Origin: GPUResource{100, 100},
			Target: GPUResource{80, 100},
			Expect: GPUResource{20, 100},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{20, 100},
			Expect: GPUResource{80, 100},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{0, 100},
			Expect: GPUResource{100, 100},
		},
	}
	for _, tc := range subTests {
		tc.Origin.Sub(tc.Target)
		assert.Equal(t, tc.Origin, tc.Expect)
	}

	addTests := []Cases{
		{
			Origin: GPUResource{20, 100},
			Target: GPUResource{80, 100},
			Expect: GPUResource{100, 100},
		}, {
			Origin: GPUResource{10, 100},
			Target: GPUResource{80, 100},
			Expect: GPUResource{90, 100},
		},
	}
	for _, tc := range addTests {
		tc.Origin.Add(tc.Target)
		assert.Equal(t, tc.Origin, tc.Expect)
	}
	subIfAvailedTests := []Cases{
		{
			Origin: GPUResource{100, 100},
			Target: GPUResource{80, 100},
			Expect: GPUResource{20, 100},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{80, 100},
			Expect: GPUResource{20, 100},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{100, 100},
			Expect: GPUResource{0, 100},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{120, 100},
			Expect: GPUResource{100, 100},
		}, {
			Origin: GPUResource{30, 100},
			Target: GPUResource{100, 100},
			Expect: GPUResource{30, 100},
		},
	}
	for _, tc := range subIfAvailedTests {
		if tc.Origin.CanAllocate(tc.Target) {
			tc.Origin.Sub(tc.Target)
		}
		assert.Equal(t, tc.Expect, tc.Origin)
	}
}

func MockPodWithPlan(plan *Plan) *v1.Pod {
	pod := &v1.Pod{}
	pod.Annotations = map[string]string{
		types.GPUAssume: "true",
	}

	for cidx, gidx := range plan.GPUIndexes {
		pod.Annotations[fmt.Sprintf(types.AnnotationGPUContainerOn, strconv.Itoa(cidx))] = strconv.Itoa(gidx)
		pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
			Name: strconv.Itoa(cidx),
			Resources: v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					types.ResourceGPUPercent: resource.MustParse(strconv.Itoa(plan.Demand[cidx].Percent)),
				},
			},
		})
	}
	return pod
}

func MockPodWithDemand(demand Demand) *v1.Pod {
	pod := &v1.Pod{}
	pod.Annotations = map[string]string{}

	for _, gpu := range demand {
		pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
			Resources: v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					types.ResourceGPUPercent: resource.MustParse(strconv.Itoa(gpu.Percent)),
				},
			},
		})
	}
	return pod
}

func TestNewDemandFromPod(t *testing.T) {
	demandList := []Demand{
		{{100, 0}, {100, 0}},
		{{100, 0}, {50, 0}, {50, 0}},
		{},
	}
	for _, demand := range demandList {
		pod := MockPodWithDemand(demand)
		assert.Equal(t, NewDemandFromPod(pod), demand)
	}
}

func TestNewPlanFromPod(t *testing.T) {
	plans := []Plan{
		{
			Demand:     Demand{{100, 0}, {100, 0}},
			GPUIndexes: []int{0, 1},
			Score:      0,
		}, {
			Demand:     Demand{{50, 0}, {100, 0}},
			GPUIndexes: []int{0, 0},
			Score:      0,
		}, {
			Demand:     Demand{{100, 0}, {100, 0}},
			GPUIndexes: []int{0, 0},
			Score:      0,
		},
	}
	for _, tc := range plans {
		pod := MockPodWithPlan(&tc)
		npl, err := NewPlanFromPod(pod)
		assert.Nil(t, err)
		assert.Equal(t, npl, &tc)
	}
}

func TestChoose(t *testing.T) {
	type ChooseCase struct {
		GPUs    GPUs
		Demand  Demand
		Success bool
	}
	chooses := []ChooseCase{
		{
			GPUs:    []*GPUResource{{100, 0}, {100, 0}},
			Demand:  []GPUResource{{50, 0}, {50, 0}},
			Success: true,
		}, {
			GPUs:    []*GPUResource{{100, 0}},
			Demand:  []GPUResource{{50, 0}, {50, 0}},
			Success: true,
		}, {
			GPUs:    []*GPUResource{{100, 0}},
			Demand:  []GPUResource{{50, 0}, {60, 0}},
			Success: false,
		}, {
			GPUs:    []*GPUResource{{100, 0}, {100, 0}},
			Demand:  []GPUResource{{100, 0}, {10, 0}},
			Success: true,
		},
	}
	rater := &SampleRater{}
	for _, choose := range chooses {
		_, err := choose.GPUs.Choose(choose.Demand, rater)
		assert.Equal(t, choose.Success, err == nil)
	}
}

func TestToSortableGPUs(t *testing.T) {
	gpus := GPUs{
		&GPUResource{80, 100},
		&GPUResource{100, 100},
		&GPUResource{30, 100},
		&GPUResource{50, 100},
	}

	expected := SortableGPUs{
		&GPUResourceWithIndex{&GPUResource{80, 100}, 0},
		&GPUResourceWithIndex{&GPUResource{100, 100}, 1},
		&GPUResourceWithIndex{&GPUResource{30, 100}, 2},
		&GPUResourceWithIndex{&GPUResource{50, 100}, 3},
	}

	sortableGpus := gpus.ToSortableGPUs()

	assert.Equal(t, len(sortableGpus), len(gpus))
	assert.Equal(t, expected, sortableGpus)
}

func TestSortableGPUs(t *testing.T) {
	gpus := SortableGPUs{
		&GPUResourceWithIndex{&GPUResource{80, 100}, 0},
		&GPUResourceWithIndex{&GPUResource{100, 100}, 1},
		&GPUResourceWithIndex{&GPUResource{30, 100}, 2},
		&GPUResourceWithIndex{&GPUResource{50, 100}, 3},
	}
	expected := SortableGPUs{
		&GPUResourceWithIndex{&GPUResource{30, 100}, 2},
		&GPUResourceWithIndex{&GPUResource{50, 100}, 3},
		&GPUResourceWithIndex{&GPUResource{80, 100}, 0},
		&GPUResourceWithIndex{&GPUResource{100, 100}, 1},
	}

	sort.Sort(gpus)

	assert.Equal(t, expected, gpus)
}
