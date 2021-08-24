/*
 * Copyright 2019 THL A29 Limited, a Tencent company.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package scheduler

import (
	"context"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/cache"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	extender "k8s.io/kube-scheduler/extender/v1"
)

type Prioritize struct {
	Name  string
	Func  func(pod *v1.Pod, nodeNames []string) (*extender.HostPriorityList, error)
	cache *cache.SchedulerCache
}

func (p Prioritize) Handler(args extender.ExtenderArgs) (*extender.HostPriorityList, error) {
	pod := args.Pod
	nodeNames := *args.NodeNames
	return p.Func(pod, nodeNames)
}

func NewNanoGPUPrioritize(ctx context.Context, clientset *kubernetes.Clientset, d dealer.Dealer) *Prioritize {
	return &Prioritize{
		Name: "NanoGPUSorter",
		Func: func(pod *v1.Pod, nodeNames []string) (*extender.HostPriorityList, error) {
			var priorityList extender.HostPriorityList
			priorityList = make([]extender.HostPriority, len(nodeNames))
			scores := d.Score(nodeNames, pod)
			for i, score := range scores {
				priorityList[i] = extender.HostPriority{
					Host:  nodeNames[i],
					Score: int64(score),
				}
			}
			return &priorityList, nil
		},
	}
}
