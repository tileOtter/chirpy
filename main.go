package main

import (
	"MODULE_PATH/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	returnString := fmt.Sprintf(
		`<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
	</html>`, cfg.fileserverHits.Load())
	w.Write([]byte(returnString))
}

func (cfg *apiConfig) handlerValidate(w http.ResponseWriter, r *http.Request) {
	type validate struct {
		Body string `json:"body"`
	}

	type errorResponse struct {
		Error string `json:"error"`
	}

	type cleanedResponse struct {
		Clean string `json:"cleaned_body"`
	}

	decoder := json.NewDecoder(r.Body)
	valid := validate{}
	err := decoder.Decode(&valid)
	if err != nil {
		log.Printf("Error Decoding: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(valid.Body) > 140 {
		errR := errorResponse{Error: "Chirp is too long"}
		data, _ := json.Marshal(errR)
		w.WriteHeader(400)
		w.Write(data)
	} else {
		cleanedR := cleanedResponse{Clean: profanityFilter(valid.Body)}
		data, _ := json.Marshal(cleanedR)
		w.WriteHeader(200)
		w.Write(data)
	}

}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cfg.fileserverHits.Store(0)
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		fmt.Println("Hits:", cfg.fileserverHits.Load())
		next.ServeHTTP(w, r)
	})
}

func profanityFilter(unclean string) string {
	words := strings.Split(unclean, " ")
	for index := range words {
		if strings.ToLower(words[index]) == "kerfuffle" || strings.ToLower(words[index]) == "sharbert" || strings.ToLower(words[index]) == "fornax" {
			words[index] = "****"
		}
	}
	return strings.Join(words, " ")
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	const port = "8080"

	mux := http.NewServeMux()
	var apiCfg apiConfig
	apiCfg.dbQueries = dbQueries
	var srv http.Server
	srv.Addr = ":" + port
	srv.Handler = mux

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	mux.HandleFunc("POST /api/validate_chirp", apiCfg.handlerValidate)

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(srv.ListenAndServe())
}
