package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/secnex/sethorize-kit/database"
	"github.com/secnex/sethorize-kit/handler/auth"
	"github.com/secnex/sethorize-kit/helper"
	"github.com/secnex/sethorize-kit/initializer"
	"github.com/secnex/sethorize-kit/middleware"
	"github.com/secnex/sethorize-kit/server"
)

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	dbHost := os.Getenv("DB_HOST")
	dbPort, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		log.Fatal(err)
	}
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")
	apiHost := os.Getenv("API_HOST")
	apiPort, err := strconv.Atoi(os.Getenv("API_PORT"))
	if err != nil {
		log.Fatal(err)
	}
	db := database.NewServer(database.ServerConnection{
		Host:     dbHost,
		Port:     dbPort,
		User:     dbUser,
		Password: dbPassword,
		Database: dbName,
	})
	db.Connect()

	// Initialize basic data (Tenant, Clients, Admin-User)
	init := initializer.NewInitializer(db.DB)
	init.Initialize()

	// Key Manager for RSA Private Keys
	keyManager := helper.NewKeyManager()
	err = keyManager.LoadOrGenerateKey()
	if err != nil {
		log.Fatal("Error loading or generating key:", err)
	}

	// Handler and Middleware
	authHandler := auth.NewAuthHandler(db.DB, keyManager)
	server := server.NewServer(apiHost, apiPort)
	logger := middleware.NewHTTPLogger(log.New(os.Stdout, "", log.LstdFlags))
	authMiddleware := middleware.NewAuthMiddleware(db.DB)

	// Global Logging Middleware for all Requests
	server.Router.Use(logger.LoggingMiddleware)

	// === UNGESCHÜTZTE ENDPUNKTE ===
	server.Router.HandleFunc("/healthz", healthz).Methods("GET")
	server.Router.HandleFunc("/auth/token", authHandler.Token).Methods("POST")

	// === LOGIN WITH CLIENT-MIDDLEWARE ===
	server.Router.Handle("/auth/login", authMiddleware.ClientMiddleware(http.HandlerFunc(authHandler.Login))).Methods("POST")

	// === PROTECTED AUTH-ENDPOINTS ===
	authProtectedRouter := server.Router.PathPrefix("/auth").Subrouter()
	authProtectedRouter.Use(authMiddleware.AuthMiddleware)
	authProtectedRouter.HandleFunc("/authorize", authHandler.Authorize).Methods("POST")
	authProtectedRouter.HandleFunc("/logout", authHandler.Logout).Methods("GET")
	authProtectedRouter.HandleFunc("/session", authHandler.Session).Methods("GET")
	authProtectedRouter.HandleFunc("/client", authHandler.Client).Methods("POST")

	// === PROTECTED API-ENDPOINTS (for future use) ===
	apiProtectedRouter := server.Router.PathPrefix("/api").Subrouter()
	apiProtectedRouter.Use(authMiddleware.AuthMiddleware)
	// Here you can add more API endpoints

	server.Start()
}
