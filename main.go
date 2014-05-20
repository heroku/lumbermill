package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	http.HandleFunc("/drain", serveDrain)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
