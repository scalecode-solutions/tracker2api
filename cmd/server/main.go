package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/scalecode-solutions/tracker2api/internal/api"
	"github.com/scalecode-solutions/tracker2api/internal/auth"
	"github.com/scalecode-solutions/tracker2api/internal/db"
)

func main() {
	// Load configuration from environment
	port := getEnv("PORT", "8080")
	databaseURL := getEnv("DATABASE_URL", "postgres://mvchat:@localhost:5432/mvchat?sslmode=disable")
	authTokenKey := getEnv("AUTH_TOKEN_KEY", "")
	uploadPath := getEnv("UPLOAD_PATH", "/srv/docker/mvchat/uploads/tracker2")
	dataPath := getEnv("DATA_PATH", "./data")
	corsOrigins := getEnv("CORS_ORIGINS", "*")

	if authTokenKey == "" {
		log.Fatal("AUTH_TOKEN_KEY environment variable is required")
	}

	// Decode base64 auth token key (same format as mvchat2's TOKEN_KEY)
	authKeyBytes, err := base64.StdEncoding.DecodeString(authTokenKey)
	if err != nil {
		log.Fatalf("Failed to decode AUTH_TOKEN_KEY: %v", err)
	}

	// Initialize database connection
	database, err := db.New(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Initialize authenticator (validates mvchat2 JWT tokens)
	authenticator := auth.New(authKeyBytes)

	// Create API handler
	apiHandler := api.New(database, authenticator, uploadPath, dataPath)

	// Set up router
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Static data endpoints (no auth required)
	r.HandleFunc("/api/data/baby-sizes", apiHandler.GetBabySizes).Methods("GET")
	r.HandleFunc("/api/data/weekly-facts", apiHandler.GetWeeklyFacts).Methods("GET")

	// API routes (all require authentication)
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(apiHandler.AuthMiddleware)

	// Pregnancy endpoints (legacy - single pregnancy)
	apiRouter.HandleFunc("/pregnancy", apiHandler.GetPregnancy).Methods("GET")
	apiRouter.HandleFunc("/pregnancy", apiHandler.CreatePregnancy).Methods("POST")
	apiRouter.HandleFunc("/pregnancy", apiHandler.UpdatePregnancy).Methods("PUT")

	// Multi-pregnancy endpoints
	apiRouter.HandleFunc("/pregnancies", apiHandler.ListPregnancies).Methods("GET")
	apiRouter.HandleFunc("/pregnancies/{id}", apiHandler.GetPregnancyByID).Methods("GET")
	apiRouter.HandleFunc("/pregnancies/{id}", apiHandler.UpdatePregnancyByID).Methods("PUT")
	apiRouter.HandleFunc("/pregnancies/{id}/entries", apiHandler.GetPregnancyEntries).Methods("GET")
	apiRouter.HandleFunc("/pregnancies/{id}/outcome", apiHandler.SetPregnancyOutcome).Methods("PUT")
	apiRouter.HandleFunc("/pregnancies/{id}/archive", apiHandler.SetPregnancyArchive).Methods("PUT")

	// Entry endpoints
	apiRouter.HandleFunc("/entries", apiHandler.GetEntries).Methods("GET")
	apiRouter.HandleFunc("/entries", apiHandler.CreateEntry).Methods("POST")
	apiRouter.HandleFunc("/entries/batch", apiHandler.BatchCreateEntries).Methods("POST")
	apiRouter.HandleFunc("/entries/{clientId}", apiHandler.DeleteEntry).Methods("DELETE")

	// Settings endpoints
	apiRouter.HandleFunc("/settings", apiHandler.GetSettings).Methods("GET")
	apiRouter.HandleFunc("/settings/{type}", apiHandler.UpdateSetting).Methods("PUT")

	// Sync endpoints
	apiRouter.HandleFunc("/sync", apiHandler.GetSync).Methods("GET")
	apiRouter.HandleFunc("/sync", apiHandler.PostSync).Methods("POST")

	// Pairing endpoints
	apiRouter.HandleFunc("/pairing/request", apiHandler.CreatePairingRequest).Methods("POST")
	apiRouter.HandleFunc("/pairing/pending", apiHandler.GetPendingPairingRequests).Methods("GET")
	apiRouter.HandleFunc("/pairing/approve/{requestId}", apiHandler.ApprovePairingRequest).Methods("POST")
	apiRouter.HandleFunc("/pairing/deny/{requestId}", apiHandler.DenyPairingRequest).Methods("POST")
	apiRouter.HandleFunc("/pairing/permission", apiHandler.UpdatePartnerPermission).Methods("PUT")
	apiRouter.HandleFunc("/pairing", apiHandler.RemovePairing).Methods("DELETE")
	apiRouter.HandleFunc("/pairing/status", apiHandler.GetPairingStatus).Methods("GET")

	// Sharing / Invite code endpoints
	apiRouter.HandleFunc("/sharing/status", apiHandler.GetSharingStatus).Methods("GET")
	apiRouter.HandleFunc("/sharing/generate", apiHandler.GenerateInviteCode).Methods("POST")
	apiRouter.HandleFunc("/sharing/redeem", apiHandler.RedeemInviteCode).Methods("POST")
	apiRouter.HandleFunc("/sharing/codes/{codeId}/revoke", apiHandler.RevokeInviteCode).Methods("POST")
	apiRouter.HandleFunc("/sharing/supporters/{supporterId}", apiHandler.RemoveSupporter).Methods("DELETE")
	apiRouter.HandleFunc("/me/role", apiHandler.GetMyRole).Methods("GET")

	// File endpoints
	apiRouter.HandleFunc("/files/upload", apiHandler.UploadFile).Methods("POST")
	apiRouter.HandleFunc("/files/{fileId}", apiHandler.GetFile).Methods("GET")
	apiRouter.HandleFunc("/files/{fileId}", apiHandler.DeleteFile).Methods("DELETE")

	// Set up CORS
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{corsOrigins}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Authorization", "Content-Type"}),
	)

	// Create server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      corsHandler(r),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Tracker2API server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		_, err := fmt.Sscanf(value, "%d", &result)
		if err == nil {
			return result
		}
	}
	return defaultValue
}
