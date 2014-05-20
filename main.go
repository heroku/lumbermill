package main

import (
	"log"
	"net/http"
	"os"

	"github.com/heroku/slog"
)

var (
	source string
)

func LogWithContext(ctx slog.Context) {
	ctx.Add("app", "lumbermill")
	ctx.Add("source", source)
	log.Println(ctx)
}

func main() {
	port := os.Getenv("PORT")
	source = os.Getenv("SOURCE")
	http.HandleFunc("/drain", serveDrain)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
