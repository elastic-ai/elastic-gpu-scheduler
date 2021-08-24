package dealer

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGPUResource(t *testing.T) {
	type Cases struct {
		Origin GPUResource
		Target GPUResource
		Expect GPUResource
	}
	subTests := []Cases{
		{
			Origin: GPUResource{100, 20},
			Target: GPUResource{80, 20},
			Expect: GPUResource{20, 0},
		}, {
			Origin: GPUResource{100, 20},
			Target: GPUResource{20, 10},
			Expect: GPUResource{80, 10},
		}, {
			Origin: GPUResource{100, 20},
			Target: GPUResource{0, 0},
			Expect: GPUResource{100, 20},
		},
	}
	for _, tc := range subTests {
		tc.Origin.Sub(tc.Target)
		assert.Equal(t, tc.Origin, tc.Expect)
	}

	addTests := []Cases{
		{
			Origin: GPUResource{20, 0},
			Target: GPUResource{80, 100},
			Expect: GPUResource{100, 100},
		}, {
			Origin: GPUResource{10, 80},
			Target: GPUResource{80, 20},
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
			Expect: GPUResource{20, 0},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{80, 120},
			Expect: GPUResource{100, 100},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{100, 100},
			Expect: GPUResource{0, 0},
		}, {
			Origin: GPUResource{100, 100},
			Target: GPUResource{120, 100},
			Expect: GPUResource{100, 100},
		}, {
			Origin: GPUResource{30, 30},
			Target: GPUResource{100, 100},
			Expect: GPUResource{30, 30},
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
		utils.GPUAssume: "true",
	}

	for cidx, gidx := range plan.GPUIndexes {
		pod.Annotations[fmt.Sprintf(utils.AnnotationGPUContainerOn, strconv.Itoa(cidx))] = strconv.Itoa(gidx)
		pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
			Name: strconv.Itoa(cidx),
			Resources: v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					utils.ResourceGPUCore:   resource.MustParse(strconv.Itoa(plan.Demand[cidx].Core)),
					utils.ResourceGPUMemory: resource.MustParse(strconv.Itoa(plan.Demand[cidx].Memory)),
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
					utils.ResourceGPUCore:   resource.MustParse(strconv.Itoa(gpu.Core)),
					utils.ResourceGPUMemory: resource.MustParse(strconv.Itoa(gpu.Memory)),
				},
			},
		})
	}
	return pod
}

func TestNewDemandFromPod(t *testing.T) {
	demandList := []Demand{
		{{100, 12}, {100, 12}},
		{{100, 12}, {50, 1}, {50, 1}},
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
			Demand:     Demand{{100, 12}, {100, 12}},
			GPUIndexes: []int{0, 1},
			Score:      0,
		}, {
			Demand:     Demand{{50, 1}, {100, 12}},
			GPUIndexes: []int{0, 0},
			Score:      0,
		}, {
			Demand:     Demand{{100, 12}, {100, 12}},
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
		GPUs GPUs
		Demand Demand
		Success bool
	}
	chooses := []ChooseCase{
		{
			GPUs:    []*GPUResource{{100, 15}, {100, 15}},
			Demand:  []GPUResource{{50, 7}, {50, 7}},
			Success: true,
		}, {
			GPUs:    []*GPUResource{{100, 15}},
			Demand:  []GPUResource{{50, 7}, {50, 7}},
			Success: true,
		}, {
			GPUs:    []*GPUResource{{100, 15}},
			Demand:  []GPUResource{{50, 7}, {50, 9}},
			Success: false,
		}, {
			GPUs:    []*GPUResource{{100, 15}},
			Demand:  []GPUResource{{50, 7}, {50, 8}},
			Success: true,
		}, {
			GPUs:    []*GPUResource{{100, 15}, {100, 15}},
			Demand:  []GPUResource{{50, 7}, {50, 9}},
			Success: true,
		}, {
			GPUs:    []*GPUResource{{100, 7}, {100, 7}},
			Demand:  []GPUResource{{50, 7}, {50, 9}},
			Success: false,
		}, {
			GPUs:    []*GPUResource{{10, 15}, {100, 15}},
			Demand:  []GPUResource{{100, 7}, {10, 15}},
			Success: true,
		},
	}
	rater := &SampleRater{}
	for _, choose := range chooses {
		_, err := choose.GPUs.Choose(choose.Demand, rater)
		assert.Equal(t, choose.Success, err == nil)
	}
}