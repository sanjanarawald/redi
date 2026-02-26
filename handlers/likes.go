package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"redi/firebase"
	"redi/middleware"
)

func LikePost(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, `{"error":"failed to like"}`, http.StatusInternalServerError)
		return
	}
	// Check if already liked
	iter := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("user_id", "==", userID).Where("target_type", "==", "post").Where("target_id", "==", postID).Limit(1).Documents(ctx)
	existing, _ := iter.Next()
	if existing != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"liked": true, "message": "already liked"})
		return
	}
	id := uuid.New().String()
	_, err = firebase.FirestoreClient.Collection(firebase.ColLikes).Doc(id).Set(ctx, map[string]interface{}{
		"user_id":     userID,
		"target_type": "post",
		"target_id":   postID,
		"created_at":  time.Now(),
	})
	if err != nil {
		http.Error(w, `{"error":"failed to like"}`, http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"liked": true})
}

func UnlikePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	iter := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("user_id", "==", userID).Where("target_type", "==", "post").Where("target_id", "==", postID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			http.Error(w, `{"error":"failed to unlike"}`, http.StatusInternalServerError)
			return
		}
		_, _ = doc.Ref.Delete(ctx)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"liked": false})
}

func LikeComment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	commentID := chi.URLParam(r, "id")
	if commentID == "" {
		http.Error(w, `{"error":"comment id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	_, err := firebase.FirestoreClient.Collection(firebase.ColComments).Doc(commentID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			http.Error(w, `{"error":"comment not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to like"}`, http.StatusInternalServerError)
		return
	}
	iter := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("user_id", "==", userID).Where("target_type", "==", "comment").Where("target_id", "==", commentID).Limit(1).Documents(ctx)
	existing, _ := iter.Next()
	if existing != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"liked": true, "message": "already liked"})
		return
	}
	id := uuid.New().String()
	_, err = firebase.FirestoreClient.Collection(firebase.ColLikes).Doc(id).Set(ctx, map[string]interface{}{
		"user_id":     userID,
		"target_type": "comment",
		"target_id":   commentID,
		"created_at":  time.Now(),
	})
	if err != nil {
		http.Error(w, `{"error":"failed to like"}`, http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"liked": true})
}

func UnlikeComment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	commentID := chi.URLParam(r, "id")
	if commentID == "" {
		http.Error(w, `{"error":"comment id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	iter := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("user_id", "==", userID).Where("target_type", "==", "comment").Where("target_id", "==", commentID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			http.Error(w, `{"error":"failed to unlike"}`, http.StatusInternalServerError)
			return
		}
		_, _ = doc.Ref.Delete(ctx)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"liked": false})
}
