package main

import (
	"fmt"
	"encoding/json"
	"net/http"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}

	for _, user := range users {
		fmt.Printf("User: %+v\n", user)
	}

	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)

// File is truncated here - missing closing brace and potentially more code