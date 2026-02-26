package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Post struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	ImagePath string    `json:"image_path,omitempty"`   // legacy
	ImageURL  string    `json:"image_url,omitempty"`    // Firebase Storage link
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Populated for API responses
	AuthorUsername string `json:"author_username,omitempty"`
	LikeCount      int    `json:"like_count,omitempty"`
	CommentCount   int    `json:"comment_count,omitempty"`
	LikedByMe      bool   `json:"liked_by_me,omitempty"`
	SavedByMe      bool   `json:"saved_by_me,omitempty"`
}

type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	// Populated for API
	AuthorUsername string     `json:"author_username,omitempty"`
	Replies        []*Comment `json:"replies,omitempty"`
	LikeCount      int        `json:"like_count,omitempty"`
	LikedByMe      bool       `json:"liked_by_me,omitempty"`
}

type Like struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TargetType string    `json:"target_type"` // "post" or "comment"
	TargetID   string    `json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Save struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	PostID    string    `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}
