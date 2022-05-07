package main

import (
	"encoding/gob"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

type Server struct {
	port string
	dm   *DManager
}

func NewServer(port string) *Server {
	return &Server{
		port: port,
		dm:   NewDManager(),
	}
}

func (sv *Server) Start() {

	r := mux.NewRouter()
	r.Handle("/pull/{self_id}", sv.pull())
	r.Handle("/push/{target_id}", sv.push())

	http.Handle("/", r)

	logrus.Infof("Listening on port %s", sv.port)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", sv.port), nil))
}

func (sv *Server) pull() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		select {
		case v := <-sv.dm.Get(vars["self_id"]):
			logrus.Debug("pull from ", vars["self_id"], v.Flag)
			w.Header().Add("Content-Type", "application/binary")
			if err := gob.NewEncoder(w).Encode(v); err != nil {
				logrus.Error("binary encode failed:", err)
				return
			}
		default:
		}
	})
}

func (sv *Server) push() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var info types.SignalingInfo
		if err := gob.NewDecoder(r.Body).Decode(&info); err != nil {
			logrus.Error("binary decode failed:", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		vars := mux.Vars(r)
		sv.dm.Set(vars["target_id"], info)
		logrus.Debug("push from ", info.Source, " to ", vars["target_id"], info.Flag)
	})
}
