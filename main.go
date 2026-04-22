package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	fmt.Fprintf(w, "Firestore connected 🚀")
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	_, _, err := client.Collection("test").Add(ctx, map[string]interface{}{
		"name": "Aditya",
	})

	if err != nil {
		log.Printf("Add error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	fmt.Fprintf(w, "Data added")
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // Better practice than context.Background() in handlers

	iter := client.Collection("users").Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		sendJSON(w, 500, APIResponse{Status: "error", Error: "Failed to fetch users"})
		return
	}

	var users []User
	for _, doc := range docs {
		var u User
		doc.DataTo(&u)
		u.ID = doc.Ref.ID // Capture the Firestore Auto-ID
		users = append(users, u)
	}

	sendJSON(w, 200, APIResponse{Status: "success", Data: users})
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.URL.Query().Get("id")
	if id == "" {
		sendJSON(w, 400, APIResponse{Status: "error", Error: "Missing ID parameter"})
		return
	}

	// Example: Hardcoding an update for now
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
