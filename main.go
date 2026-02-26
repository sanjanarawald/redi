package main

import (
	"log"
	"net/http"

	"redi/config"
	"redi/firebase"
	"redi/routes"
)

func main() {
	cfg := config.Load()

	if err := firebase.Init(cfg.Firebase.ProjectID, cfg.Firebase.StorageBucket, cfg.Firebase.CredentialsPath); err != nil {
		log.Fatal("firebase init:", err)
	}
	defer firebase.Close()

	router := routes.Setup(cfg)
	log.Printf("Server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal("server:", err)
	}
}
