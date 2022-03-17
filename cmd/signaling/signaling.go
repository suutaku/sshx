package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/suutaku/sshx/internal/node"
)

var (
	res = map[string]chan node.ConnectInfo{}
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
		log.Println("push callback")
		var info node.ConnectInfo
		if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
			log.Print("json decode failed:", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		mu.Lock()
		if res[r.URL.Path] == nil {
			log.Println("crete resource for ", r.URL.Path)
			res[r.URL.Path] = make(chan node.ConnectInfo, 64)
		}
		mu.Unlock()

		res[r.URL.Path] <- info
		log.Println("push from ", info.Source, r.URL.Path)
	})
}

func pullData() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("pull callback")
		mu.Lock()
		log.Println("pull lock")
		if res[r.URL.Path] == nil {
			log.Println("crete resource for ", r.URL.Path, " 2")
			res[r.URL.Path] = make(chan node.ConnectInfo, 64)
		}
		mu.Unlock()
		log.Println("pull unlock")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		select {
		case <-ctx.Done():
			return
		case v := <-res[r.URL.Path]:
			log.Println("pull from ", r.URL.Path, v.Source)
			w.Header().Add("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(v); err != nil {
				log.Print("json encode failed:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
	})
}
