package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"rvpodview/internal/api"
	"rvpodview/internal/podman"
)

func main() {
	// Command line flags
	addr := flag.String("addr", ":80", "HTTP server address")
	socketPath := flag.String("socket", "", "Podman socket path (auto-detect if empty)")
	jwtSecret := flag.String("secret", "", "JWT secret key (auto-generate if empty)")
	noAuth := flag.Bool("no-auth", false, "Disable authentication (for development only!)")
	flag.Parse()

	// Create Podman client
	var client *podman.Client
	var err error

	if *socketPath != "" {
		client, err = podman.NewClientWithSocket(*socketPath)
	} else {
		client, err = podman.NewClient()
	}

	if err != nil {
		log.Fatalf("Failed to connect to Podman: %v", err)
	}

	// Test connection
	if err := client.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping Podman: %v", err)
	}

	// Get JWT secret from env or flag
	secret := *jwtSecret
	if secret == "" {
		secret = os.Getenv("RVPODVIEW_JWT_SECRET")
	}

	// Create API server
	server := api.NewServer(client, secret, *noAuth)

	// Start server
	fmt.Printf("RVPodView starting on %s\n", *addr)
	if *noAuth {
		fmt.Println("WARNING: Authentication is DISABLED!")
	}
	fmt.Println("Open http://localhost" + *addr + " in your browser")

	if err := http.ListenAndServe(*addr, server.Router()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
