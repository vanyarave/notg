package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"messenger/server"
	"messenger/storage"
)

// cors is a development middleware that allows cross-origin requests from any
// origin. Tighten the Allow-Origin value before deploying to production.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	store, err := storage.NewMessageStore("chat.db")
	if err != nil {
		log.Fatalf("open message store: %v", err)
	}
	defer store.Close()

	go func() {
		log.Println("pprof debug server listening on :6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Println("pprof server error:", err)
		}
	}()

	manager := server.NewChatManager(store)
	hub := server.NewHub(manager)
	go hub.Run()

	http.HandleFunc("/login", server.LoginHandler(store))
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWs(hub, store, w, r)
	})

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, cors(http.DefaultServeMux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
