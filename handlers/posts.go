package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"redi/firebase"
	"redi/middleware"
	"redi/models"
)

func CreatePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	cfg := getConfig(r)
	if cfg == nil {
		http.Error(w, `{"error":"config not available"}`, http.StatusInternalServerError)
		return
	}

	var content string
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/") {
		_ = r.ParseMultipartForm(10 << 20)
		content = r.FormValue("content")
	} else {
		var body struct {
			Content string `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		content = body.Content
	}

	var imageURL string
	if r.MultipartForm != nil {
		if fhs := r.MultipartForm.File["image"]; len(fhs) > 0 {
			fh := fhs[0]
			ext := filepath.Ext(fh.Filename)
			if ext == "" {
				ext = ".jpg"
			}
			objectName := uuid.New().String() + ext
			src, err := fh.Open()
			if err != nil {
				http.Error(w, `{"error":"failed to read image file"}`, http.StatusBadRequest)
				return
			}
			url, err := firebase.UploadPostImage(r.Context(), objectName, src, "image/jpeg")
			_ = src.Close()
			if err != nil {
				http.Error(w, `{"error":"failed to upload image"}`, http.StatusInternalServerError)
				return
			}
			imageURL = url
		}
	}

	if content == "" && imageURL == "" {
		http.Error(w, `{"error":"content or image required"}`, http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	now := time.Now()
	_, err := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(id).Set(r.Context(), map[string]interface{}{
		"user_id":    userID,
		"content":    content,
		"image_url":  imageURL,
		"created_at": now,
		"updated_at": now,
	})
	if err != nil {
		http.Error(w, `{"error":"failed to create post"}`, http.StatusInternalServerError)
		return
	}
	post, _ := getPostByID(r.Context(), id, userID)
	respondJSON(w, http.StatusCreated, post)
}

func GetFeed(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)
	if limit > 50 {
		limit = 50
	}

	// Firestore: newest first (Desc), then skip offset and take limit in code
	iter := firebase.FirestoreClient.Collection(firebase.ColPosts).OrderBy("created_at", firestore.Desc).Limit(limit + offset).Documents(r.Context())
	var posts []*models.Post
	idx := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			http.Error(w, `{"error":"failed to get feed"}`, http.StatusInternalServerError)
			return
		}
		if idx >= offset {
	p := docToPost(r.Context(), doc)
	if p != nil {
		posts = append(posts, p)
	}
		}
		idx++
	}
	enrichPosts(r.Context(), posts, userID)
	respondJSON(w, http.StatusOK, map[string]interface{}{"posts": posts})
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	post, err := getPostByID(r.Context(), id, userID)
	if status.Code(err) == codes.NotFound || post == nil {
		http.Error(w, `{"error":"post not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"failed to get post"}`, http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, post)
}

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	ref := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(id)
	doc, err := ref.Get(r.Context())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			http.Error(w, `{"error":"post not found or not owned by you"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to update post"}`, http.StatusInternalServerError)
		return
	}
	owner, _ := doc.Data()["user_id"].(string)
	if owner != userID {
		http.Error(w, `{"error":"post not found or not owned by you"}`, http.StatusNotFound)
		return
	}
	_, err = ref.Update(r.Context(), []firestore.Update{
		{Path: "content", Value: body.Content},
		{Path: "updated_at", Value: time.Now()},
	})
	if err != nil {
		http.Error(w, `{"error":"failed to update post"}`, http.StatusInternalServerError)
		return
	}
	post, _ := getPostByID(r.Context(), id, userID)
	respondJSON(w, http.StatusOK, post)
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	ref := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(id)
	doc, err := ref.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			http.Error(w, `{"error":"post not found or not owned by you"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to delete post"}`, http.StatusInternalServerError)
		return
	}
	owner, _ := doc.Data()["user_id"].(string)
	if owner != userID {
		http.Error(w, `{"error":"post not found or not owned by you"}`, http.StatusNotFound)
		return
	}
	// Delete image from Storage first so we don't leave orphan files
	if rawURL := stringFromData(doc.Data(), "image_url"); rawURL != "" {
		if objectPath := parseStorageObjectPath(rawURL); objectPath != "" {
			_ = firebase.StorageBucket.Object(objectPath).Delete(ctx)
		}
	}
	_, err = ref.Delete(ctx)
	if err != nil {
		http.Error(w, `{"error":"failed to delete post"}`, http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func getPostByID(ctx context.Context, id, currentUserID string) (*models.Post, error) {
	doc, err := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	p := docToPost(ctx, doc)
	if p == nil {
		return nil, status.Error(codes.NotFound, "not found")
	}
	p.ID = id
	enrichPosts(ctx, []*models.Post{p}, currentUserID)
	return p, nil
}

func docToPost(ctx context.Context, doc *firestore.DocumentSnapshot) *models.Post {
	if doc == nil || !doc.Exists() {
		return nil
	}
	ref := doc.Ref
	data := doc.Data()
	p := &models.Post{ID: ref.ID}
	if v, ok := data["user_id"].(string); ok {
		p.UserID = v
	}
	if v, ok := data["content"].(string); ok {
		p.Content = v
	}
	// Use stringFromData so we never miss image_url due to Firestore type quirks
	if raw := stringFromData(data, "image_url"); raw != "" {
		// Serve via proxy (distinct path so no route conflict with GET /posts/{id})
		p.ImageURL = "/api/img/post/" + ref.ID
		p.ImagePath = p.ImageURL
	}
	if v, ok := data["created_at"].(time.Time); ok {
		p.CreatedAt = v
	}
	if v, ok := data["updated_at"].(time.Time); ok {
		p.UpdatedAt = v
	}
	userDoc, _ := firebase.FirestoreClient.Collection(firebase.ColUsers).Doc(p.UserID).Get(ctx)
	if userDoc != nil && userDoc.Exists() {
		if u, ok := userDoc.Data()["username"].(string); ok {
			p.AuthorUsername = u
		}
	}
	return p
}

func enrichPosts(ctx context.Context, posts []*models.Post, currentUserID string) {
	if len(posts) == 0 {
		return
	}
	ids := make([]string, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	// Like counts for posts
	likeMap := make(map[string]int)
	for _, id := range ids {
		iter := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("target_type", "==", "post").Where("target_id", "==", id).Documents(ctx)
		c := 0
		for {
			_, err := iter.Next()
			if err == iterator.Done {
				break
			}
			c++
		}
		likeMap[id] = c
	}
	commentMap := make(map[string]int)
	for _, id := range ids {
		iter := firebase.FirestoreClient.Collection(firebase.ColComments).Where("post_id", "==", id).Documents(ctx)
		c := 0
		for {
			_, err := iter.Next()
			if err == iterator.Done {
				break
			}
			c++
		}
		commentMap[id] = c
	}
	likedSet := make(map[string]bool)
	if currentUserID != "" {
		for _, id := range ids {
			iter := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("user_id", "==", currentUserID).Where("target_type", "==", "post").Where("target_id", "==", id).Limit(1).Documents(ctx)
			doc, _ := iter.Next()
			likedSet[id] = (doc != nil)
		}
	}
	savedSet := make(map[string]bool)
	if currentUserID != "" {
		for _, id := range ids {
			iter := firebase.FirestoreClient.Collection(firebase.ColSaves).Where("user_id", "==", currentUserID).Where("post_id", "==", id).Limit(1).Documents(ctx)
			doc, _ := iter.Next()
			savedSet[id] = (doc != nil)
		}
	}
	for _, p := range posts {
		p.LikeCount = likeMap[p.ID]
		p.CommentCount = commentMap[p.ID]
		p.LikedByMe = likedSet[p.ID]
		p.SavedByMe = savedSet[p.ID]
	}
}

// GetPostImage streams the post image from Firebase Storage (no auth required so img src works).
// Route: GET /api/img/post/{id}
func GetPostImage(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ctx := r.Context()
	doc, err := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(postID).Get(ctx)
	if err != nil || !doc.Exists() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	rawURL := stringFromData(doc.Data(), "image_url")
	if rawURL == "" {
		http.Error(w, "no image", http.StatusNotFound)
		return
	}
	objectPath := parseStorageObjectPath(rawURL)
	if objectPath == "" {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	reader, err := firebase.StorageBucket.Object(objectPath).NewReader(ctx)
	if err != nil {
		http.Error(w, "failed to read image", http.StatusNotFound)
		return
	}
	defer reader.Close()
	contentType := reader.ContentType()
	if contentType == "" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.Copy(w, reader)
}

// stringFromData gets a string from Firestore doc data (type assertion or fmt.Sprint).
func stringFromData(data map[string]interface{}, key string) string {
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return fmt.Sprint(v)
}

// parseStorageObjectPath extracts object path from Firebase Storage URL or returns path as-is.
func parseStorageObjectPath(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if !strings.HasPrefix(rawURL, "http") {
		path := strings.TrimPrefix(rawURL, "/")
		if strings.HasPrefix(path, "posts/") {
			return path
		}
		return "posts/" + path
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := u.Path
	// Firebase URL: /v0/b/BUCKET/o/ENCODED_PATH  -> we want ENCODED_PATH decoded
	if i := strings.Index(path, "/o/"); i >= 0 {
		path = path[i+3:]
	}
	if path == "" {
		return ""
	}
	path, err = url.PathUnescape(path)
	if err != nil {
		return ""
	}
	path = strings.Trim(path, "/")
	if path == "" || !strings.HasPrefix(path, "posts/") {
		return ""
	}
	return path
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	q := r.URL.Query().Get(key)
	if q == "" {
		return defaultVal
	}
	var n int
	_, _ = fmt.Sscanf(q, "%d", &n)
	if n <= 0 {
		return defaultVal
	}
	return n
}
