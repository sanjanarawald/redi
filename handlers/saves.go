package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"redi/firebase"
	"redi/middleware"
	"redi/models"
)

func SavePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	_, err := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(postID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			http.Error(w, `{"error":"post not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to save"}`, http.StatusInternalServerError)
		return
	}
	iter := firebase.FirestoreClient.Collection(firebase.ColSaves).Where("user_id", "==", userID).Where("post_id", "==", postID).Limit(1).Documents(ctx)
	existing, _ := iter.Next()
	if existing != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"saved": true, "message": "already saved"})
		return
	}
	id := uuid.New().String()
	_, err = firebase.FirestoreClient.Collection(firebase.ColSaves).Doc(id).Set(ctx, map[string]interface{}{
		"user_id":   userID,
		"post_id":   postID,
		"created_at": time.Now(),
	})
	if err != nil {
		http.Error(w, `{"error":"failed to save"}`, http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"saved": true})
}

func UnsavePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	iter := firebase.FirestoreClient.Collection(firebase.ColSaves).Where("user_id", "==", userID).Where("post_id", "==", postID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			http.Error(w, `{"error":"failed to unsave"}`, http.StatusInternalServerError)
			return
		}
		_, _ = doc.Ref.Delete(ctx)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"saved": false})
}

func GetSavedPosts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()
	iter := firebase.FirestoreClient.Collection(firebase.ColSaves).Where("user_id", "==", userID).Documents(ctx)
	var posts []*models.Post
	type saveDoc struct {
		postID   string
		createdAt time.Time
	}
	var saves []saveDoc
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			http.Error(w, `{"error":"failed to get saved posts"}`, http.StatusInternalServerError)
			return
		}
		postID, _ := doc.Data()["post_id"].(string)
		if postID == "" {
			continue
		}
		var createdAt time.Time
		if t, ok := doc.Data()["created_at"].(time.Time); ok {
			createdAt = t
		}
		saves = append(saves, saveDoc{postID: postID, createdAt: createdAt})
	}
	sort.Slice(saves, func(i, j int) bool { return saves[j].createdAt.Before(saves[i].createdAt) })
	for _, s := range saves {
		postDoc, err := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(s.postID).Get(ctx)
		if err != nil || !postDoc.Exists() {
			continue
		}
		p := docToPost(ctx, postDoc)
		if p != nil {
			posts = append(posts, p)
		}
	}
	enrichPosts(ctx, posts, userID)
	respondJSON(w, http.StatusOK, map[string]interface{}{"posts": posts})
}
