package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"redi/config"
	"redi/firebase"
	"redi/middleware"
	"redi/models"
)

func Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"email, username and password required"}`, http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"failed to hash password"}`, http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	// Check unique email
	emailIter := firebase.FirestoreClient.Collection(firebase.ColUsers).Where("email", "==", req.Email).Limit(1).Documents(ctx)
	emailDoc, emailErr := emailIter.Next()
	if emailErr != nil && emailErr != iterator.Done {
		http.Error(w, `{"error":"registration failed"}`, http.StatusInternalServerError)
		return
	}
	if emailDoc != nil {
		http.Error(w, `{"error":"email or username already taken"}`, http.StatusConflict)
		return
	}
	// Check unique username
	userIter := firebase.FirestoreClient.Collection(firebase.ColUsers).Where("username", "==", req.Username).Limit(1).Documents(ctx)
	userDoc, userErr := userIter.Next()
	if userErr != nil && userErr != iterator.Done {
		http.Error(w, `{"error":"registration failed"}`, http.StatusInternalServerError)
		return
	}
	if userDoc != nil {
		http.Error(w, `{"error":"email or username already taken"}`, http.StatusConflict)
		return
	}
	id := uuid.New().String()
	_, err = firebase.FirestoreClient.Collection(firebase.ColUsers).Doc(id).Set(ctx, map[string]interface{}{
		"email":         req.Email,
		"username":      req.Username,
		"password_hash": string(hash),
		"created_at":    time.Now(),
	})
	if err != nil {
		http.Error(w, `{"error":"registration failed"}`, http.StatusInternalServerError)
		return
	}
	user, _ := userByID(ctx, id)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"user":  user,
		"token": mustToken(id, r),
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	iter := firebase.FirestoreClient.Collection(firebase.ColUsers).Where("email", "==", req.Email).Limit(1).Documents(ctx)
	doc, err := iter.Next()
	if err == iterator.Done || doc == nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"login failed"}`, http.StatusInternalServerError)
		return
	}
	data := doc.Data()
	hash, _ := data["password_hash"].(string)
	if hash == "" {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}
	id := doc.Ref.ID
	user, _ := userByID(ctx, id)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": mustToken(id, r),
	})
}

func Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	user, err := userByID(r.Context(), userID)
	if err != nil || user == nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

func userByID(ctx context.Context, id string) (*models.User, error) {
	doc, err := firebase.FirestoreClient.Collection(firebase.ColUsers).Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	data := doc.Data()
	u := &models.User{ID: id}
	if v, ok := data["email"].(string); ok {
		u.Email = v
	}
	if v, ok := data["username"].(string); ok {
		u.Username = v
	}
	if v, ok := data["created_at"].(time.Time); ok {
		u.CreatedAt = v
	}
	return u, nil
}

func mustToken(userID string, r *http.Request) string {
	cfg := getConfig(r)
	if cfg == nil {
		return ""
	}
	claims := &middleware.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(cfg.JWTSecret))
	return signed
}

func isUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "AlreadyExists") || strings.Contains(err.Error(), "already exists")
}

func getConfig(r *http.Request) *config.Config {
	cfg, _ := r.Context().Value(configKey).(*config.Config)
	return cfg
}
