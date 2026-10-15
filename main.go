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
	"time"

	"github.com/google/uuid"

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
	plat := os.Getenv("PLATFORM")
	if plat != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cfg.dbQueries.DeleteUsers(r.Context())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cfg.fileserverHits.Store(0)
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		fmt.Println("Hits:", cfg.fileserverHits.Load())
		next.ServeHTTP(w, r)
	})
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error Decoding: %s", err)
		w.WriteHeader(500)
		return
	}
	dbUser, err := cfg.dbQueries.CreateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Error Creating User: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}
	data, err := json.Marshal(user)
	if err != nil {
		log.Printf("Error Marshaling: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	type errorResponse struct {
		Error string `json:"error"`
	}

	type successResponse struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error Decoding: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Body) > 140 {
		errR := errorResponse{Error: "Chirp is too long"}
		data, _ := json.Marshal(errR)
		w.WriteHeader(400)
		w.Write(data)
	} else {
		cleanedBody := profanityFilter(params.Body)
		chirp, err := cfg.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   cleanedBody,
			UserID: params.UserID,
		})
		if err != nil {
			log.Printf("Error Creating Chirp: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := successResponse{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error Marshaling: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(data)
	}
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
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerCreateChirp)
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(srv.ListenAndServe())
}
