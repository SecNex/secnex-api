package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/secnex/sethorize-kit/database"
	"github.com/secnex/sethorize-kit/handler/account"
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

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" && value != "*****" {
		return value
	}
	return defaultValue
}

func main() {
	fmt.Println("Starting application...")

	dbHost := os.Getenv("DB_HOST")
	dbPort, _ := strconv.Atoi(getEnvDefault("DB_PORT", "5432"))
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")
	apiHost := os.Getenv("API_HOST")
	apiPort, _ := strconv.Atoi(getEnvDefault("API_PORT", "8080"))
	fmt.Printf("Database connection: %s:%d/%s\n", dbHost, dbPort, dbName)
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
	if err := keyManager.LoadOrGenerateKey(); err != nil {
		fmt.Println("Error loading or generating key:", err)
		os.Exit(1)
	}

	// Handler and Middleware
	authHandler := auth.NewAuthHandler(db.DB, keyManager)
	accountHandler := account.NewAccountHandler(db.DB, keyManager)
	server := server.NewServer(apiHost, apiPort)
	logger := middleware.NewHTTPLogger()
	authMiddleware := middleware.NewAuthMiddleware(db.DB, keyManager)

	// Global Logging Middleware for all Requests
	server.Router.Use(logger.LoggingMiddleware)

	// === UNGESCHÜTZTE ENDPUNKTE ===
	server.Router.HandleFunc("/healthz", healthz).Methods("GET")
	server.Router.HandleFunc("/auth/token", authHandler.Token).Methods("POST")

	// === LOGIN WITH CLIENT-MIDDLEWARE ===
	server.Router.Handle("/auth/login", authMiddleware.ClientMiddleware(http.HandlerFunc(authHandler.Login))).Methods("POST")

	// === PROTECTED AUTH-ENDPOINTS ===
	server.Router.Handle("/auth/authorize", authMiddleware.AuthMiddleware(http.HandlerFunc(authHandler.Authorize))).Methods("POST")
	server.Router.Handle("/auth/logout", authMiddleware.AuthMiddleware(http.HandlerFunc(authHandler.Logout))).Methods("GET")
	server.Router.Handle("/auth/session", authMiddleware.AuthMiddleware(http.HandlerFunc(authHandler.Session))).Methods("GET")
	server.Router.Handle("/auth/client", authMiddleware.AuthMiddleware(http.HandlerFunc(authHandler.Client))).Methods("POST")

	// === PROTECTED API-ENDPOINTS (for future use) ===
	apiProtectedRouter := server.Router.PathPrefix("/api").Subrouter()
	apiProtectedRouter.Use(authMiddleware.AuthMiddleware)
	apiProtectedRouter.Handle("/account/password", http.HandlerFunc(accountHandler.PasswordChange)).Methods("PUT")
	// Here you can add more API endpoints

	server.Start()
}
