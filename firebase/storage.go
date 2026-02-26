package firebase

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"cloud.google.com/go/storage"
)

// UploadPostImage uploads image to Firebase Storage at "posts/{objectName}" and returns the public URL.
// objectName should be like "uuid.jpg". Object is set to public read so the URL works in browsers.
func UploadPostImage(ctx context.Context, objectName string, content io.Reader, contentType string) (string, error) {
	path := "posts/" + objectName
	obj := StorageBucket.Object(path)
	w := obj.NewWriter(ctx)
	if contentType != "" {
		w.ContentType = contentType
	}
	if _, err := io.Copy(w, content); err != nil {
		_ = w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	// Make object publicly readable so the URL works in browsers
	_ = obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader)
	encoded := url.PathEscape(path)
	return fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media", BucketName, encoded), nil
}
