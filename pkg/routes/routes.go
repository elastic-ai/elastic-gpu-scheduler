package routes

import (
	"bytes"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/scheduler"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/server"
	"encoding/json"
	"fmt"
	"io"
	v1 "k8s.io/api/core/v1"
	"net/http"

	"github.com/julienschmidt/httprouter"

	log "k8s.io/klog/v2"
	extender "k8s.io/kube-scheduler/extender/v1"
)

const (
	versionPath      = "/version"
	apiPrefix        = "/scheduler"
	bindPrefix       = apiPrefix + "/bind"
	predicatesPrefix = apiPrefix + "/filter"
	prioritiesPrefix = apiPrefix + "/priorities"
	statusPrefix     = apiPrefix + "/status"
)

var (
	version = "0.1.0"
)

func checkBody(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return
	}
}

func PredicateRoute(predicate *server.Predicate) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		checkBody(w, r)

		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)

		var extenderArgs extender.ExtenderArgs
		var extenderFilterResult *extender.ExtenderFilterResult

		if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {

			log.Warning("Failed to parse request due to error: %v", err)
			extenderFilterResult = &extender.ExtenderFilterResult{
				Nodes:       nil,
				FailedNodes: nil,
				Error:       err.Error(),
			}
		} else {
			log.V(5).Infof("GpuSharingFilter ExtenderArgs: %+v", extenderArgs)
			if extenderArgs.NodeNames == nil {
				extenderFilterResult = &extender.ExtenderFilterResult{
					Nodes:       nil,
					FailedNodes: nil,
					Error:       "elastic-gpu-scheduler extender must be configured with nodeCacheCapable=true",
				}
			} else {
				log.Infof("Start to filter for pod %s/%s", extenderArgs.Pod.Namespace, extenderArgs.Pod.Name)
				extenderFilterResult = predicate.Handler(extenderArgs)
			}
		}

		if resultBody, err := json.Marshal(extenderFilterResult); err != nil {
			log.Warningf("Failed to parse filter result: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
		} else {
			log.Infof("%s extenderFilterResult: %s", predicate.Name, string(resultBody))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func PrioritizeRoute(prioritize *server.Prioritize) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		checkBody(w, r)

		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)
		log.V(5).Info(prioritize.Name, " ExtenderArgs = ", buf.String())

		var extenderArgs extender.ExtenderArgs
		var hostPriorityList *extender.HostPriorityList

		if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
			panic(err)
		}

		log.Infof("Start to score for pod %s/%s", extenderArgs.Pod.Namespace, extenderArgs.Pod.Name)
		if list, err := prioritize.Handler(extenderArgs); err != nil {
			panic(err)
		} else {
			hostPriorityList = list
		}

		if resultBody, err := json.Marshal(hostPriorityList); err != nil {
			panic(err)
		} else {
			log.Info("%s hostPriorityList: %s", prioritize.Name, string(resultBody))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func BindRoute(bind *server.Bind) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		checkBody(w, r)

		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)

		var extenderBindingArgs extender.ExtenderBindingArgs
		var extenderBindingResult *extender.ExtenderBindingResult
		failed := false
		if err := json.NewDecoder(body).Decode(&extenderBindingArgs); err != nil {
			extenderBindingResult = &extender.ExtenderBindingResult{
				Error: err.Error(),
			}
			failed = true
		} else {
			log.Infof("Start to bind pod %s/%s to node %s", extenderBindingArgs.PodNamespace, extenderBindingArgs.PodName, extenderBindingArgs.Node)
			log.V(5).Info("GpuSharingBind ExtenderArgs =", extenderBindingArgs)
			extenderBindingResult = bind.Handler(extenderBindingArgs)
		}

		if len(extenderBindingResult.Error) > 0 {
			failed = true
		}

		if resultBody, err := json.Marshal(extenderBindingResult); err != nil {
			log.Warning("Fail to parse bind result: %+v", err)
			// panic(err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
		} else {
			log.Info("extenderBindingResult = ", string(resultBody))
			w.Header().Set("Content-Type", "application/json")
			if failed {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}

			w.Write(resultBody)
		}
	}
}

func VersionRoute(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, fmt.Sprint(version))
}

func AddVersion(router *httprouter.Router) {
	router.GET(versionPath, DebugLogging(VersionRoute, versionPath))
}

func DebugLogging(h httprouter.Handle, path string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		log.V(5).Infof("%s request method: %s, body: %+v", path, r.Method, r.Body)
		h(w, r, p)
		log.V(5).Infof("%s response: %+v", path, w)
	}
}

func AddPredicate(router *httprouter.Router, predicate *server.Predicate) {
	router.POST(predicatesPrefix, DebugLogging(PredicateRoute(predicate), predicatesPrefix))
}

func AddPrioritize(router *httprouter.Router, prioritize *server.Prioritize) {
	router.POST(prioritiesPrefix, DebugLogging(PrioritizeRoute(prioritize), prioritiesPrefix))
}

func AddBind(router *httprouter.Router, bind *server.Bind) {
	if handle, _, _ := router.Lookup("POST", bindPrefix); handle != nil {
		log.Warning("AddBind was called more then once")
	} else {
		router.POST(bindPrefix, DebugLogging(BindRoute(bind), bindPrefix))
	}
}

func AddStatus(router *httprouter.Router, sches map[v1.ResourceName]scheduler.ResourceScheduler) {
	router.GET(statusPrefix, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		result := make(map[string]string)
		for k, v := range sches {
			result[string(k)] = v.Status()
		}
		w.Header().Set("Content-Type", "application/json")
		if resultBody, err := json.Marshal(result); err != nil {
			log.Warning(" due to ", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			w.Write(resultBody)
		}

	})
}
