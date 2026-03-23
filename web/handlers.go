package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/tts"
)

func registerHandlers(mux *http.ServeMux, client *api.Client, ttsEng *tts.Engine) {
	mux.HandleFunc("/api/bibles", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("language")
		bibles, err := client.GetBibles(lang)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, bibles)
	}))

	mux.HandleFunc("/api/bibles/{bibleID}/books", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		bibleID := r.PathValue("bibleID")
		books, err := client.GetBooks(bibleID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, books)
	}))

	mux.HandleFunc("/api/bibles/{bibleID}/books/{bookID}/chapters", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		bibleID := r.PathValue("bibleID")
		bookID := r.PathValue("bookID")
		chapters, err := client.GetChapters(bibleID, bookID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, chapters)
	}))

	mux.HandleFunc("/api/bibles/{bibleID}/chapters/{chapterID}", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		bibleID := r.PathValue("bibleID")
		chapterID := r.PathValue("chapterID")
		chapter, err := client.GetChapter(bibleID, chapterID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, chapter)
	}))

	mux.HandleFunc("/api/bibles/{bibleID}/verses/{verseID}", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		bibleID := r.PathValue("bibleID")
		verseID := r.PathValue("verseID")
		verse, err := client.GetVerse(bibleID, verseID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, verse)
	}))

	mux.HandleFunc("/api/bibles/{bibleID}/search", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		bibleID := r.PathValue("bibleID")
		query := r.URL.Query().Get("query")
		limitStr := r.URL.Query().Get("limit")
		limit, _ := strconv.Atoi(limitStr)
		data, err := client.Search(bibleID, query, limit)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, data)
	}))

	mux.HandleFunc("/api/tts/engine", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]string{"engine": ttsEng.EngineName(), "available": strconv.FormatBool(ttsEng.Available())})
	}))

	mux.HandleFunc("/api/tts/speak", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid body", http.StatusBadRequest)
			return
		}
		if _, err := ttsEng.Speak(body.Text); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/tts/stop", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ttsEng.Stop()
		jsonOK(w, map[string]bool{"ok": true})
	}))
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}
