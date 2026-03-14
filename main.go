package main

import (
	"log"
	"net/http"

	"messenger/server"
	"messenger/storage"
)

func main() {
	store, err := storage.NewMessageStore("chat.db")
	if err != nil {
		log.Fatalf("open message store: %v", err)
	}
	defer store.Close()

	hub := server.NewHub(store)
	go hub.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWs(hub, store, w, r)
	})

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
