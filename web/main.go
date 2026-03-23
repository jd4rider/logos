package main

import (
"embed"
"fmt"
"io/fs"
"log"
"net/http"
"os"

"github.com/jd4rider/logos/internal/api"
"github.com/jd4rider/logos/internal/tts"

"github.com/joho/godotenv"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
_ = godotenv.Load()

apiKey := os.Getenv("API_BIBLE_KEY")
if apiKey == "" {
log.Fatal("API_BIBLE_KEY environment variable not set")
}

port := os.Getenv("PORT")
if port == "" {
port = "8484"
}

client := api.NewClient(apiKey)
piperModel := os.Getenv("PIPER_MODEL")
ttsEng := tts.New(piperModel)

mux := http.NewServeMux()
registerHandlers(mux, client, ttsEng)

// Serve frontend static files
dist, err := fs.Sub(frontendFS, "frontend/dist")
if err != nil {
log.Fatal(err)
}
fileServer := http.FileServer(http.FS(dist))

mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
// SPA fallback: serve index.html for non-API, non-asset routes
path := r.URL.Path
if len(path) > 0 && path[0] == '/' {
path = path[1:]
}
if _, err := dist.Open(path); err != nil {
r.URL.Path = "/"
}
fileServer.ServeHTTP(w, r)
})

addr := ":" + port
fmt.Printf("✝ Bible Web running at http://localhost%s\n", addr)
fmt.Printf("  TTS engine: %s\n", ttsEng.EngineName())
log.Fatal(http.ListenAndServe(addr, mux))
}
