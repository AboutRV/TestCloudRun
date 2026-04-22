package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
)

var client *firestore.Client

func main() {
	ctx := context.Background()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	var err error
	client, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", handler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/list", listHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.ListenAndServe(":"+port, nil)
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
		http.Error(w, err.Error(), 500)
		return
	}

	fmt.Fprintf(w, "Data added")
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	iter := client.Collection("test").Documents(ctx)
	docs, _ := iter.GetAll()

	for _, doc := range docs {
		fmt.Fprintf(w, "%v\n", doc.Data())
	}
}
