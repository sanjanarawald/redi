package firebase

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

var (
	FirestoreClient *firestore.Client
	StorageBucket   *storage.BucketHandle
	BucketName      string
)

// Init initializes Firestore and Storage using explicit config.
func Init(projectID, storageBucket, credentialsPath string) error {
	ctx := context.Background()
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID:     projectID,
		StorageBucket: storageBucket,
	}, opt)
	if err != nil {
		return err
	}
	FirestoreClient, err = app.Firestore(ctx)
	if err != nil {
		return err
	}
	// Use GCP Storage client with same credentials for bucket access
	storageClient, err := storage.NewClient(ctx, opt)
	if err != nil {
		return err
	}
	StorageBucket = storageClient.Bucket(storageBucket)
	BucketName = storageBucket
	log.Printf("Firebase initialized: project=%s bucket=%s (images at posts/... in this bucket)", projectID, storageBucket)
	return nil
}

// Close releases Firebase clients.
func Close() {
	if FirestoreClient != nil {
		_ = FirestoreClient.Close()
	}
}
