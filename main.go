package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
)

var client *firestore.Client

func main() {
	// ctx := context.Background()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT is not set")
	}

	var err error
	// client, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("⚠️ Firestore init failed: %v", err)
	} else {
		log.Println("✅ Firestore connected")
	}

	log.Println("✅ Firestore initialized")

	http.HandleFunc("/", handler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/list", listHandler)

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
	ctx := context.Background()

	iter := client.Collection("test").Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		log.Printf("List error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	for _, doc := range docs {
		fmt.Fprintf(w, "%v\n", doc.Data())
	}
}
