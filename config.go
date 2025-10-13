package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// getConfigPath returns the absolute path for a configuration file, ensuring it's
// stored in the appropriate user config directory. /
// getConfigPath mengembalikan path absolut untuk file konfigurasi, memastikan file tersebut disimpan di direktori config pengguna yang sesuai.
func getConfigPath(filename string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not get user config dir: %w", err)
	}

	appConfigDir := filepath.Join(configDir, "panggil")
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		return "", fmt.Errorf("could not create app config dir: %w", err)
	}

	return filepath.Join(appConfigDir, filename), nil
}

// initLogger sets up the application's logger to write to a file.
// initLogger mengatur logger aplikasi untuk menulis log ke sebuah file.
func initLogger() {
	path, err := getConfigPath("panggil.log")
	if err != nil {
		log.Fatalf("FATAL: Failed to get log file path: %v", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("FATAL: error opening log file: %v", err)
	}
	log.SetOutput(f)
	log.Println("INFO: Logger initialized. Application starting.")
}

// saveCollections serializes the collections data to a JSON file.
// saveCollections melakukan serialisasi data Collections ke file JSON.
func (a *App) saveCollections() {
	path, err := getConfigPath("collections.json")
	if err != nil {
		log.Printf("ERROR: Could not get config path for collections: %v", err)
		return
	}
	data, err := json.MarshalIndent(a.collectionsRoot, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal collections: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("ERROR: Failed to write collections file: %v", err)
	}
}

// saveGrpcCache serializes the gRPC request body cache to a JSON file.
// saveGrpcCache melakukan serialisasi cache body request gRPC ke file JSON.
func (a *App) saveGrpcCache() {
	path, err := getConfigPath("grpc_cache.json")
	if err != nil {
		log.Printf("ERROR: Could not get config path for gRPC cache: %v", err)
		return
	}
	data, err := json.MarshalIndent(a.grpcBodyCache, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal gRPC cache: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("ERROR: Failed to write gRPC cache file: %v", err)
	}
}
