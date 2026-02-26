package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cloud.google.com/go/firestore"
	"redi/firebase"
	"redi/middleware"
	"redi/models"
)

func CreateComment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Content  string  `json:"content"`
		ParentID *string `json:"parent_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if body.Content == "" {
		http.Error(w, `{"error":"content required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	postRef := firebase.FirestoreClient.Collection(firebase.ColPosts).Doc(postID)
	postDoc, err := postRef.Get(ctx)
	if err != nil || !postDoc.Exists() {
		http.Error(w, `{"error":"post not found"}`, http.StatusNotFound)
		return
	}
	if body.ParentID != nil && *body.ParentID != "" {
		parentDoc, _ := firebase.FirestoreClient.Collection(firebase.ColComments).Doc(*body.ParentID).Get(ctx)
		if parentDoc == nil || !parentDoc.Exists() {
			http.Error(w, `{"error":"invalid parent comment"}`, http.StatusBadRequest)
			return
		}
		if p, _ := parentDoc.Data()["post_id"].(string); p != postID {
			http.Error(w, `{"error":"invalid parent comment"}`, http.StatusBadRequest)
			return
		}
	}
	id := uuid.New().String()
	payload := map[string]interface{}{
		"post_id":    postID,
		"user_id":    userID,
		"content":    body.Content,
		"created_at": time.Now(),
	}
	if body.ParentID != nil && *body.ParentID != "" {
		payload["parent_id"] = *body.ParentID
	}
	_, err = firebase.FirestoreClient.Collection(firebase.ColComments).Doc(id).Set(ctx, payload)
	if err != nil {
		http.Error(w, `{"error":"failed to create comment"}`, http.StatusInternalServerError)
		return
	}
	comment, _ := getCommentByID(ctx, id, userID)
	respondJSON(w, http.StatusCreated, comment)
}

func GetComments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, `{"error":"post id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	iter := firebase.FirestoreClient.Collection(firebase.ColComments).Where("post_id", "==", postID).Documents(ctx)
	var all []*models.Comment
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			http.Error(w, `{"error":"failed to get comments"}`, http.StatusInternalServerError)
			return
		}
		c := docToComment(doc)
		if c != nil {
			all = append(all, c)
		}
	}
	// Sort by created_at (no composite index needed)
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	enrichComments(ctx, all, userID)
	top := make([]*models.Comment, 0)
	byID := make(map[string]*models.Comment)
	for _, c := range all {
		byID[c.ID] = c
	}
	for _, c := range all {
		if c.ParentID == nil || *c.ParentID == "" {
			top = append(top, c)
		} else {
			parent := byID[*c.ParentID]
			if parent != nil {
				parent.Replies = append(parent.Replies, c)
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"comments": top})
}

func ReplyToComment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	commentID := chi.URLParam(r, "id")
	if commentID == "" {
		http.Error(w, `{"error":"comment id required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	parentDoc, err := firebase.FirestoreClient.Collection(firebase.ColComments).Doc(commentID).Get(ctx)
	if err != nil || !parentDoc.Exists() {
		http.Error(w, `{"error":"comment not found"}`, http.StatusNotFound)
		return
	}
	postID, _ := parentDoc.Data()["post_id"].(string)
	if postID == "" {
		http.Error(w, `{"error":"comment not found"}`, http.StatusNotFound)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if body.Content == "" {
		http.Error(w, `{"error":"content required"}`, http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	_, err = firebase.FirestoreClient.Collection(firebase.ColComments).Doc(id).Set(ctx, map[string]interface{}{
		"post_id":   postID,
		"user_id":   userID,
		"parent_id": commentID,
		"content":   body.Content,
		"created_at": time.Now(),
	})
	if err != nil {
		http.Error(w, `{"error":"failed to create reply"}`, http.StatusInternalServerError)
		return
	}
	comment, _ := getCommentByID(ctx, id, userID)
	respondJSON(w, http.StatusCreated, comment)
}

func getCommentByID(ctx context.Context, id, currentUserID string) (*models.Comment, error) {
	doc, err := firebase.FirestoreClient.Collection(firebase.ColComments).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	if !doc.Exists() {
		return nil, status.Error(codes.NotFound, "not found")
	}
	c := docToComment(doc)
	if c != nil {
		c.ID = id
		enrichComments(ctx, []*models.Comment{c}, currentUserID)
	}
	return c, nil
}

func docToComment(doc *firestore.DocumentSnapshot) *models.Comment {
	if doc == nil || !doc.Exists() {
		return nil
	}
	data := doc.Data()
	c := &models.Comment{ID: doc.Ref.ID}
	if v, ok := data["post_id"].(string); ok {
		c.PostID = v
	}
	if v, ok := data["user_id"].(string); ok {
		c.UserID = v
	}
	if v, ok := data["parent_id"].(string); ok && v != "" {
		c.ParentID = &v
	}
	if v, ok := data["content"].(string); ok {
		c.Content = v
	}
	if v, ok := data["created_at"].(time.Time); ok {
		c.CreatedAt = v
	}
	userDoc, _ := firebase.FirestoreClient.Collection(firebase.ColUsers).Doc(c.UserID).Get(context.Background())
	if userDoc != nil && userDoc.Exists() {
		if u, ok := userDoc.Data()["username"].(string); ok {
			c.AuthorUsername = u
		}
	}
	return c
}

func enrichComments(ctx context.Context, comments []*models.Comment, currentUserID string) {
	if len(comments) == 0 {
		return
	}
	likeMap := make(map[string]int)
	likedSet := make(map[string]bool)
	for _, c := range comments {
		iter := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("target_type", "==", "comment").Where("target_id", "==", c.ID).Documents(ctx)
		n := 0
		for {
			_, err := iter.Next()
			if err == iterator.Done {
				break
			}
			n++
		}
		likeMap[c.ID] = n
		if currentUserID != "" {
			iter2 := firebase.FirestoreClient.Collection(firebase.ColLikes).Where("user_id", "==", currentUserID).Where("target_type", "==", "comment").Where("target_id", "==", c.ID).Limit(1).Documents(ctx)
			doc, _ := iter2.Next()
			likedSet[c.ID] = (doc != nil)
		}
	}
	for _, c := range comments {
		c.LikeCount = likeMap[c.ID]
		c.LikedByMe = likedSet[c.ID]
	}
}
