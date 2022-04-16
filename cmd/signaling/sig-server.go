package main

import (
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/suutaku/sshx/pkg/types"
)

var (
	res = map[string]chan types.SignalingInfo{}
	mu  sync.RWMutex
)

func main() {
	http.Handle("/pull/", http.StripPrefix("/pull/", pullData()))
	http.Handle("/push/", http.StripPrefix("/push/", pushData()))

	port := os.Getenv("SSHX_SIGNALING_PORT")
	if port == "" {
		port = "11095"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func pushData() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var info types.SignalingInfo
		if err := gob.NewDecoder(r.Body).Decode(&info); err != nil {
			log.Print("binary decode failed:", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		mu.Lock()
		if res[r.URL.Path] == nil {
			log.Println("crete resource for ", r.URL.Path)
			res[r.URL.Path] = make(chan types.SignalingInfo, 64)
		}
		mu.Unlock()

		res[r.URL.Path] <- info
		log.Println("push from ", info.Source, " to ", r.URL.Path)
	})
}

func pullData() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if res[r.URL.Path] == nil {
			log.Println("crete resource for ", r.URL.Path, " 2")
			res[r.URL.Path] = make(chan types.SignalingInfo, 64)
		}
		mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		select {
		case <-ctx.Done():
			return
		case v := <-res[r.URL.Path]:
			log.Println("pull from ", r.URL.Path)
			w.Header().Add("Content-Type", "application/binary")
			if err := gob.NewEncoder(w).Encode(v); err != nil {
				log.Print("binary encode failed:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
	})
}
