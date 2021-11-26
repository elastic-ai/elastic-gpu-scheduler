package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/scheduler"

	log "k8s.io/klog/v2"
	extender "k8s.io/kube-scheduler/extender/v1"
)

const (
	versionPath      = "/version"
	apiPrefix        = "/scheduler"
	bindPrefix       = apiPrefix + "/bind"
	predicatesPrefix = apiPrefix + "/filter"
	prioritiesPrefix = apiPrefix + "/priorities"

	statusPrefix = "/status"
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

func PredicateRoute(predicate *scheduler.Predicate) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		checkBody(w, r)

		// mu.RLock()
		// defer mu.RUnlock()

		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)

		var extenderArgs extender.ExtenderArgs
		var extenderFilterResult *extender.ExtenderFilterResult

		if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {

			log.Warning("Failed to parse request due to error ", err)
			extenderFilterResult = &extender.ExtenderFilterResult{
				Nodes:       nil,
				FailedNodes: nil,
				Error:       err.Error(),
			}
		} else {
			log.V(5).Infof("GpuSharingFilter ExtenderArgs: %v", extenderArgs)
			if extenderArgs.NodeNames == nil {
				extenderFilterResult = &extender.ExtenderFilterResult{
					Nodes:       nil,
					FailedNodes: nil,
					Error:       "nano-gpu-scheduler extender must be configured with nodeCacheCapable=true",
				}
			} else {
				log.Infof("start filter for pod %s/%s", extenderArgs.Pod.Namespace, extenderArgs.Pod.Name)
				extenderFilterResult = predicate.Handler(extenderArgs)
			}
		}

		if resultBody, err := json.Marshal(extenderFilterResult); err != nil {
			// panic(err)
			log.Warningf("Failed due to %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
		} else {
			log.Info(predicate.Name, " extenderFilterResult = ", string(resultBody))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func PrioritizeRoute(prioritize *scheduler.Prioritize) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		checkBody(w, r)

		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)
		log.V(2).Info(prioritize.Name, " ExtenderArgs = ", buf.String())

		var extenderArgs extender.ExtenderArgs
		var hostPriorityList *extender.HostPriorityList

		if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
			panic(err)
		}

		log.Infof("start score for pod %s/%s", extenderArgs.Pod.Namespace, extenderArgs.Pod.Name)
		if list, err := prioritize.Handler(extenderArgs); err != nil {
			panic(err)
		} else {
			hostPriorityList = list
		}

		if resultBody, err := json.Marshal(hostPriorityList); err != nil {
			panic(err)
		} else {
			log.Info(prioritize.Name, " hostPriorityList = ", string(resultBody))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func BindRoute(bind *scheduler.Bind) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		checkBody(w, r)

		// mu.Lock()
		// defer mu.Unlock()
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
			log.Infof("start bind pod %s/%s to node %s", extenderBindingArgs.PodNamespace, extenderBindingArgs.PodName, extenderBindingArgs.Node)
			log.V(2).Info("GpuSharingBind ExtenderArgs =", extenderBindingArgs)
			extenderBindingResult = bind.Handler(extenderBindingArgs)
		}

		if len(extenderBindingResult.Error) > 0 {
			failed = true
		}

		if resultBody, err := json.Marshal(extenderBindingResult); err != nil {
			log.Warning("Failed due to ", err)
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
		log.V(4).Info(path, " request body = ", r.Body)
		h(w, r, p)
		log.V(4).Info(path, " response=", w)
	}
}

func AddPredicate(router *httprouter.Router, predicate *scheduler.Predicate) {
	router.POST(predicatesPrefix, DebugLogging(PredicateRoute(predicate), predicatesPrefix))
}

func AddPrioritize(router *httprouter.Router, prioritize *scheduler.Prioritize) {
	router.POST(prioritiesPrefix, DebugLogging(PrioritizeRoute(prioritize), prioritiesPrefix))
}

func AddBind(router *httprouter.Router, bind *scheduler.Bind) {
	if handle, _, _ := router.Lookup("POST", bindPrefix); handle != nil {
		log.Warning("AddBind was called more then once!")
	} else {
		router.POST(bindPrefix, DebugLogging(BindRoute(bind), bindPrefix))
	}
}

func AddStatus(router *httprouter.Router, d dealer.Dealer) {
	if handle, _, _ := router.Lookup("GET", statusPrefix); handle != nil {
		log.Warning("AddBind was called more then once!")
	} else {
		router.POST(statusPrefix, DebugLogging(StatusRoute(d), statusPrefix))
	}
}

func StatusRoute(d dealer.Dealer) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		nodeMaps, err := d.Status()
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			log.Warningf("failed to get status: %v", err)

			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
			return
		}

		if resultBody, err := json.Marshal(nodeMaps); err != nil {
			log.Warning("failed due to ", err)
			// panic(err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			w.Write(resultBody)
		}

	}
}
