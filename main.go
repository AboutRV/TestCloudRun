package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
)

// User represents our document in Firestore
type User struct {
	ID    string `json:"id" firestore:"-"` // ID usually comes from the doc name
	Name  string `json:"name" firestore:"name"`
	Email string `json:"email" firestore:"email"`
}

// APIResponse is a standard wrapper for all JSON responses
type APIResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

var client *firestore.Client

func main() {
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		// Stop hard-failing if it's not set. Fallback to a default or log a severe warning,
		// or ensure you pass it via your CI/CD pipeline.
		log.Fatal("GOOGLE_CLOUD_PROJECT environment variable is missing.")
	}

	var err error
	// UNCOMMENT THIS LINE:
	client, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("⚠️ Firestore init failed: %v", err) // Use Fatalf here. If the DB fails, the app shouldn't run.
	} else {
		log.Println("✅ Firestore connected")
	}

	log.Println("✅ Firestore initialized")

	http.HandleFunc("/", handler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/update", updateHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("🚀 Server running on port", port)

	// IMPORTANT: this must not silently fail
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, APIResponse{Status: "success", Data: "Firestore API running 🚀"})
}

// 🚀 THE NEW ADD HANDLER: Accepts JSON, enforces POST, writes to "users"
func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, 405, APIResponse{Status: "error", Error: "Only POST allowed"})
		return
	}

	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		sendJSON(w, 400, APIResponse{Status: "error", Error: "Invalid JSON payload"})
		return
	}

	if u.Name == "" {
		sendJSON(w, 400, APIResponse{Status: "error", Error: "Name is required"})
		return
	}

	ctx := r.Context()
	// IMPORTANT: Writing to "users" collection
	ref, _, err := client.Collection("users").Add(ctx, u)
	if err != nil {
		log.Printf("Add error: %v", err)
		sendJSON(w, 500, APIResponse{Status: "error", Error: "Failed to save"})
		return
	}

	u.ID = ref.ID
	sendJSON(w, 201, APIResponse{Status: "success", Data: u})
}

// 📋 THE LIST HANDLER: Reads from "users", returns [] instead of null
func listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// IMPORTANT: Reading from "users" collection
	iter := client.Collection("users").Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		sendJSON(w, 500, APIResponse{Status: "error", Error: "Failed to fetch users"})
		return
	}

	// Fixes the "null" bug. This initializes an empty array.
	users := []User{}
	for _, doc := range docs {
		var u User
		doc.DataTo(&u)
		u.ID = doc.Ref.ID
		users = append(users, u)
	}

	sendJSON(w, 200, APIResponse{Status: "success", Data: users})
}

// ✏️ THE UPDATE HANDLER: Updates "users"
func updateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.URL.Query().Get("id")
	if id == "" {
		sendJSON(w, 400, APIResponse{Status: "error", Error: "Missing ID parameter"})
		return
	}

	// IMPORTANT: Updating "users" collection
	_, err := client.Collection("users").Doc(id).Set(ctx, map[string]interface{}{
		"name": "Updated Name",
	}, firestore.MergeAll)

	if err != nil {
		sendJSON(w, 500, APIResponse{Status: "error", Error: err.Error()})
		return
	}

	sendJSON(w, 200, APIResponse{Status: "success", Data: "User updated"})
}

func sendJSON(w http.ResponseWriter, status int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}