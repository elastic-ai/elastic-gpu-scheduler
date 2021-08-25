# Nano GPU Scheduler
![](static/linkedin_banner_image_1.png)
<!-- ABOUT THE PROJECT -->
## About This Project
With the continuous evolution of cloud native AI scenarios, more and more users run AI tasks on Kubernetes, which also brings more and more challenges to GPU resource scheduling. 

*Nano gpu scheduler* is a gpu scheduling framework based on Kubernetes, which focuses on fine-grained gpu resource scheduling and allocation.

You may also be interested in *Nano GPU Agent* which is a Kubernetes device plugin implement.

## Motivation
In the GPU container field, GPU providers such as nvidia have introduced a docker-based gpu containerization project that allows users to use GPU cards in Kubernetes Pods via the Kubernetes extended resource with the nvidia k8s device plugin. However, this project focuses more on how containers use GPU cards on Kubernetes nodes, and not on GPU resource scheduling.

Nano GPU scheduler is based on Kubernetes extended scheduler, which can schedule gpu cores and memories, share gpu with multiple containers and even spread containers of pod to different gpus. The scheduling algorithm supports binpack, spread, random and other policies. In addition, through the supporting nano gpu agent, it can be adapted to nvidia docker, gpushare, qgpu and other gpu container solutions. Nano GPU scheduler mainly satisfies the GPU resources scheduling and allocation requirements in Kubernetes.

## Architecture
![](static/nano-gpu-scheduler-arch.png)

## Prerequisites
- Kubernetes v1.17+
- golang 1.16+
- [NVIDIA drivers](https://github.com/NVIDIA/nvidia-docker/wiki/Frequently-Asked-Questions#how-do-i-install-the-nvidia-driver) 
- [nvidia-docker](https://github.com/NVIDIA/nvidia-docker) 
  
## Getting Started
1. Create RBAC
```
$ kubectl apply -f deploy/nano-gpu-rbac.yaml
```
2. Deploy Nano GPU Agent
```
$ kubectl apply -f deploy/nano-gpu-agent-ds.yaml
```
3. Deploy Nano GPU Scheduler
```
$ kubectl apply -f deploy/nano-gpu-scheduler-deploy.yaml
```
4. Enable Kubernetes scheduler extender
Add the following configuration to `extender` in `/etc/kubernetes/scheduler-policy-config.json`:
```
{
  "urlPrefix": "http://<kube-apiserver-svc>/api/v1/namespaces/kube-system/services/<nano-gpu-scheduler-svc>/proxy/scheduler",
  "filterVerb": "filter",
  "prioritizeVerb": "priorities",
  "bindVerb": "bind",
  "weight": 10,
  "enableHttps": false,
  "nodeCacheCapable": true,
  "managedResources": [
    {
      "name": "nano-gpu.io/gpu-core",
      "ignoredByScheduler": false
    },
    {
      "name": "nano-gpu.io/gpu-memory",
      "ignoredByScheduler": false
    }
  ]
}
```

You can set a scheduling policy by running `kube-scheduler --policy-config-file <filename>` or `kube-scheduler --policy-configmap <ConfigMap>`. Here is a [scheduler policy config sample](https://github.com/kubernetes/examples/blob/master/staging/scheduler-policy/scheduler-policy-config.json).

5. Create GPU pod
```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cuda-10c
  labels:
    app: gpu-test
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gpu-test
  template:
    metadata:
      labels:
        app: gpu-test
    spec:
      containers:
        - name: cuda
          image: nvidia/cuda:10.0-base
          command: [ "sleep", "100000" ]
          resources:
            requests:
              nano-gpu.io/gpu-core: "20"
              nano-gpu.io/gpu-memory: "1"
            limits:
              nano-gpu.io/gpu-core: "20"
              nano-gpu.io/gpu-memory: "1"

```

<!-- ROADMAP -->
## Roadmap
- Integrate with nvidia-docker and support gpushare
- Support Kubernetes scheduler framework
- Push Kubernetes community to improve device plugin mechanism to support multiple resources

<!-- LICENSE -->
## License
Distributed under the Apache License.

<!-- CONTACT -->
## Contact
- @xiaoxubeii xiaoxubeii@gmail.com
- @payall4u payall4u@qq.com
