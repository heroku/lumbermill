package main

import (
	"log"
	"net/http"
	"os"

	"github.com/heroku/slog"
)

func LogWithContext(ctx slog.Context) {
	ctx.Add("app", "lumbermill")
	log.Println(ctx)
}

func main() {
	port := os.Getenv("PORT")
	http.HandleFunc("/drain", serveDrain)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
