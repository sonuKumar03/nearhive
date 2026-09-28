package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sonukumar/nearhive/internal/api"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/config"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

var rootCmd = &cobra.Command{
	Use:   "nearhive",
	Short: "NearHive API",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(userCmd())
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the NearHive REST API",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("failed to load config: %v", err)
			}

			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("failed to connect to database: %v", err)
			}
			defer dbStore.Close()

			// Auth Manager
			authMgr := auth.NewManager(cfg.JWTSecret, 24*time.Hour)

			// HTTP Server
			router := api.NewRouter(dbStore, authMgr)
			srv := &http.Server{
				Addr:         ":" + cfg.Port,
				Handler:      router,
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
			}

			go func() {
				log.Printf("🐝 NearHive API server listening on :%s", cfg.Port)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("listen failed: %v", err)
				}
			}()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			<-stop

			log.Println("Shutting down NearHive...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		},
	}
}

func userCmd() *cobra.Command {
	userRoot := &cobra.Command{Use: "user", Short: "User management commands"}
	var email, password string

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user account",
		Run: func(cmd *cobra.Command, args []string) {
			if email == "" || password == "" {
				log.Fatal("both --email and --password are required")
			}
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("config error: %v", err)
			}
			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("database error: %v", err)
			}
			defer dbStore.Close()

			hashed, err := auth.HashPassword(password)
			if err != nil {
				log.Fatalf("hash error: %v", err)
			}

			user := &model.User{
				ID:       uuid.New(),
				Email:    email,
				Password: hashed,
			}
			if err := dbStore.CreateUser(context.Background(), user); err != nil {
				log.Fatalf("create user failed: %v", err)
			}
			fmt.Printf("User created successfully: %s (ID: %s)\n", user.Email, user.ID)
		},
	}
	createCmd.Flags().StringVarP(&email, "email", "e", "", "User email")
	createCmd.Flags().StringVarP(&password, "password", "p", "", "User password")
	userRoot.AddCommand(createCmd)
	return userRoot
}
