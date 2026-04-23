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

// UpdateRequest represents a partial update for a specific document
type UpdateRequest struct {
	ID   string                 `json:"id"`
	Data map[string]interface{} `json:"data"`
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

	// Unprotected routes (e.g., health checks)
	http.HandleFunc("/", handler)

	// 🛡️ Protected routes wrapped in middleware
	http.HandleFunc("/list", authMiddleware(listHandler))
	http.HandleFunc("/add", authMiddleware(addHandler))
	http.HandleFunc("/update", authMiddleware(updateHandler))
	http.HandleFunc("/batch-add", authMiddleware(batchAddHandler))
	http.HandleFunc("/batch-update", authMiddleware(bulkUpdateHandler))

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

// 🔐 THE GATEKEEPER: Middleware to protect your endpoints
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Check for the Authorization header
		token := r.Header.Get("Authorization")

		// 2. Validate the token (In a real app, verify the JWT here)
		// For the sandbox, we enforce a strict secret key.
		expectedToken := "Bearer VOIBIZ_SUPER_SECRET"

		if token != expectedToken {
			log.Printf("⚠️ Unauthorized access attempt from %s", r.RemoteAddr)
			sendJSON(w, http.StatusUnauthorized, APIResponse{
				Status: "error",
				Error:  "Unauthorized. Invalid or missing token.",
			})
			return
		}

		// 3. If the token is valid, pass the request to the actual handler
		next.ServeHTTP(w, r)
	}
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

	// 🚀 THE FIX: Wrap the array and the count in a map
	sendJSON(w, 200, APIResponse{
		Status: "success",
		Data: map[string]interface{}{
			"count": len(users),
			"users": users,
		},
	})
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

// 📦 THE BATCH ADD HANDLER: Uses modern BulkWriter API
func batchAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, http.StatusMethodNotAllowed, APIResponse{Status: "error", Error: "Only POST allowed"})
		return
	}

	var users []User
	if err := json.NewDecoder(r.Body).Decode(&users); err != nil {
		sendJSON(w, http.StatusBadRequest, APIResponse{Status: "error", Error: "Invalid JSON payload. Expected an array of users."})
		return
	}

	if len(users) == 0 {
		sendJSON(w, http.StatusBadRequest, APIResponse{Status: "error", Error: "Empty payload"})
		return
	}

	ctx := r.Context()

	// Initialize the BulkWriter.
	// It automatically handles the 500-limit chunking, retries, and parallel execution.
	bw := client.BulkWriter(ctx)

	// Queue the operations
	for _, u := range users {
		var ref *firestore.DocumentRef
		if u.ID == "" {
			ref = client.Collection("users").NewDoc()
		} else {
			ref = client.Collection("users").Doc(u.ID)
		}

		// bw.Set queues the write. It does not block.
		_, err := bw.Set(ref, u)
		if err != nil {
			log.Printf("BulkWriter queue error for user %s: %v", u.Name, err)
			// We log the error but allow the rest of the queue to process.
		}
	}

	// Flush blocks until all queued operations have been sent to Firestore and completed.
	bw.Flush()

	sendJSON(w, http.StatusCreated, APIResponse{
		Status: "success",
		Data:   fmt.Sprintf("Successfully processed %d records via BulkWriter", len(users)),
	})
}

// ✏️ THE BATCH UPDATE HANDLER: Safely merges partial data into existing documents
func bulkUpdateHandler(w http.ResponseWriter, r *http.Request) {
	// Updates should technically be PUT or PATCH, but POST is acceptable for custom bulk endpoints
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		sendJSON(w, http.StatusMethodNotAllowed, APIResponse{Status: "error", Error: "Only POST or PUT allowed"})
		return
	}

	var updates []UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		sendJSON(w, http.StatusBadRequest, APIResponse{Status: "error", Error: "Invalid JSON payload. Expected an array of update objects."})
		return
	}

	if len(updates) == 0 {
		sendJSON(w, http.StatusBadRequest, APIResponse{Status: "error", Error: "Empty payload"})
		return
	}

	ctx := r.Context()
	bw := client.BulkWriter(ctx)

	validUpdates := 0

	for _, u := range updates {
		// Defensive programming: Do not process if the client forgot the ID
		if u.ID == "" {
			log.Printf("⚠️ Bulk Update Warning: Skipped record because ID was missing")
			continue
		}
		if len(u.Data) == 0 {
			log.Printf("⚠️ Bulk Update Warning: Skipped record %s because data was empty", u.ID)
			continue
		}

		ref := client.Collection("users").Doc(u.ID)

		// MergeAll is the safety net. It only overwrites the fields provided in u.Data
		_, err := bw.Set(ref, u.Data, firestore.MergeAll)
		if err != nil {
			log.Printf("BulkWriter update error for user %s: %v", u.ID, err)
		} else {
			validUpdates++
		}
	}

	bw.Flush()

	sendJSON(w, http.StatusOK, APIResponse{
		Status: "success",
		Data:   fmt.Sprintf("Successfully processed %d updates via BulkWriter", validUpdates),
	})
}

func sendJSON(w http.ResponseWriter, status int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
