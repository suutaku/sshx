package main

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

var (
	res = map[string]chan types.SignalingInfo{}
	mu  sync.RWMutex
)

func debugOn() bool {
	str := os.Getenv("SSHX_DEBUG")
	if str == "" {
		return false
	}
	lowStr := strings.ToLower(str)
	if lowStr == "1" || lowStr == "true" || lowStr == "yes" {
		return true
	}
	return false
}

func main() {
	if debugOn() {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	port := os.Getenv("SSHX_SIGNALING_PORT")
	if port == "" {
		port = "11095"
		logrus.Infof("Defaulting to port %s", port)
	}

	r := mux.NewRouter()
	r.Handle("/pull/{self_id}", pullData())
	r.Handle("/push/{target_id}", pushData())

	http.Handle("/", r)

	logrus.Infof("Listening on port %s", port)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func pushData() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var info types.SignalingInfo
		if err := gob.NewDecoder(r.Body).Decode(&info); err != nil {
			logrus.Error("binary decode failed:", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		vars := mux.Vars(r)
		mu.Lock()
		if res[vars["target_id"]] == nil {
			logrus.Debug("crete resource for ", vars["target_id"])
			res[vars["target_id"]] = make(chan types.SignalingInfo, 64)
		}
		mu.Unlock()

		res[vars["target_id"]] <- info
		logrus.Debug("push from ", info.Source, " to ", vars["target_id"], info.Flag)
	})
}

func pullData() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mu.Lock()
		if res[vars["self_id"]] == nil {
			logrus.Debug("crete resource for ", vars["self_id"], " 2")
			res[vars["self_id"]] = make(chan types.SignalingInfo, 64)
		}
		mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		select {
		case <-ctx.Done():
			return
		case v := <-res[vars["self_id"]]:
			logrus.Debug("pull from ", vars["self_id"], v.Flag)
			w.Header().Add("Content-Type", "application/binary")
			if err := gob.NewEncoder(w).Encode(v); err != nil {
				logrus.Error("binary encode failed:", err)
				return
			}
		}
	})
}
