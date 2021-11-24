package dealer

import (
	"fmt"

	schetypes "github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"
	v1 "k8s.io/api/core/v1"
)

type NodeInterface interface {
	Assume(demand Demand) (bool, error)
	Score(demand Demand) int
	Bind(demand Demand) (*Plan, error)
	Allocate(plan *Plan) error
	Release(plan *Plan) error
}

type NodeInfo struct {
	Rater     Rater
	Name      string
	GPUs      GPUs
	PlanCache map[string]*Plan
}

func NewNodeInfo(name string, node *v1.Node, rater Rater) *NodeInfo {
	var (
		count     = utils.GetGPUDeviceCountOfNode(node)
		resources = make(GPUs, count)
	)
	for i := 0; i < count; i++ {
		resources[i] = &GPUResource{
			Percent:      schetypes.GPUPercentEachCard,
			PercentTotal: schetypes.GPUPercentEachCard,
		}
	}
	return &NodeInfo{
		Rater:     rater,
		Name:      name,
		GPUs:      resources,
		PlanCache: make(map[string]*Plan),
	}
}

func (ni *NodeInfo) Assume(demand Demand) (bool, error) {
	key := demand.Hash()

	if _, ok := ni.PlanCache[key]; ok {
		return true, nil
	}

	plan, err := ni.GPUs.Choose(demand, ni.Rater)
	if err != nil {
		return false, err
	}
	ni.PlanCache[key] = plan
	return true, nil
}

func (ni *NodeInfo) Score(demands Demand) int {
	key := demands.Hash()
	_, ok := ni.PlanCache[key]
	if !ok {
		if assumed, _ := ni.Assume(demands); !assumed {
			return ScoreMin
		}
	}
	return ni.PlanCache[key].Score
}

func (ni *NodeInfo) Bind(demands Demand) (*Plan, error) {
	key := demands.Hash()
	_, ok := ni.PlanCache[key]
	if !ok {
		if assumed, _ := ni.Assume(demands); !assumed {
			return nil, fmt.Errorf("assume %s on %s failed", demands, ni.GPUs)
		}
	}
	plan := ni.PlanCache[key]
	if err := ni.GPUs.Allocate(plan); err != nil {
		return nil, err
	}
	ni.cleanPlan()
	return plan, nil
}

func (ni *NodeInfo) Allocate(plan *Plan) error {
	ni.cleanPlan()
	return ni.GPUs.Allocate(plan)
}

func (ni *NodeInfo) Release(plan *Plan) error {
	ni.cleanPlan()
	return ni.GPUs.Release(plan)
}

func (ni *NodeInfo) cleanPlan() {
	ni.PlanCache = make(map[string]*Plan)
}
