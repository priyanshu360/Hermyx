package main

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, world!")
	})

	http.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Current time: %s", time.Now().Format(time.RFC3339))
	})

	http.HandleFunc("/delay", func(w http.ResponseWriter, r *http.Request) {
		delay := 5 * time.Second
		time.Sleep(delay)
		fmt.Fprintf(w, "Response delayed by %v seconds", delay.Seconds())
	})

	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		msg := r.URL.Query().Get("msg")
		fmt.Fprintf(w, "Echo: %s", msg)
	})

	http.HandleFunc("/exceed", func(w http.ResponseWriter, r *http.Request) {
		size := 1_500_000
		payload := bytes.Repeat([]byte("a"), size)

		fmt.Fprintf(w, "Echo POST body length: %d\nFirst 100 bytes:\n%s", len(payload), string(payload[:100]))
	})

	fmt.Println("Test server running on :8081")
	http.ListenAndServe(":8081", nil)
}
