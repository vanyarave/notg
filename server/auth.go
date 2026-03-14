package server

import (
	"encoding/json"
	"log"
	"net/http"

	"messenger/storage"
)

type loginRequest struct {
	Username string `json:"username"`
}

type loginResponse struct {
	Token  string `json:"token"`
	UserID int64  `json:"user_id"`
}

// LoginHandler handles POST /login.
// It creates the user if they do not exist, issues a new session token,
// and returns the token along with the user ID.
func LoginHandler(store *storage.MessageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		userID, err := store.CreateUser(req.Username)
		if err != nil {
			log.Printf("login CreateUser: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		token, err := store.CreateSession(userID)
		if err != nil {
			log.Printf("login CreateSession: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{Token: token, UserID: userID})
	}
}
