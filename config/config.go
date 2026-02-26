package config

import "os"

// Config holds server and Firebase settings. Sensitive values come from environment variables.
type Config struct {
	Port       string
	JWTSecret  string
	Firebase   FirebaseConfig
}

type FirebaseConfig struct {
	ProjectID       string // Firebase project ID
	StorageBucket   string // e.g. "my-project.firebasestorage.app"
	CredentialsPath string // path to service account JSON key file
}

// Env keys for configuration (set these in production; see .env.example).
const (
	EnvPort               = "REDI_PORT"
	EnvJWTSecret          = "REDI_JWT_SECRET"
	EnvFirebaseProjectID  = "REDI_FIREBASE_PROJECT_ID"
	EnvFirebaseBucket     = "REDI_FIREBASE_STORAGE_BUCKET"
	EnvFirebaseCredsPath  = "REDI_FIREBASE_CREDENTIALS_PATH"
)

// Load reads config from environment variables. Defaults support local dev only; set all REDI_* vars in production.
func Load() *Config {
	port := os.Getenv(EnvPort)
	if port == "" {
		port = "8080"
	}
	jwtSecret := os.Getenv(EnvJWTSecret)
	if jwtSecret == "" {
		jwtSecret = "test-jwt-secret-change-in-production"
	}
	projectID := os.Getenv(EnvFirebaseProjectID)
	storageBucket := os.Getenv(EnvFirebaseBucket)
	credsPath := os.Getenv(EnvFirebaseCredsPath)
	if credsPath == "" {
		credsPath = "serviceAccountKey.json"
	}
	return &Config{
		Port:      port,
		JWTSecret: jwtSecret,
		Firebase: FirebaseConfig{
			ProjectID:       projectID,
			StorageBucket:   storageBucket,
			CredentialsPath: credsPath,
		},
	}
}
