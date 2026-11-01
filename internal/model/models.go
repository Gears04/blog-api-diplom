package model

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserResponse never includes a password or its hash.
type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func (u User) ToResponse() UserResponse {
	return UserResponse{ID: u.ID, Username: u.Username, Email: u.Email, CreatedAt: u.CreatedAt}
}

// AuthorResponse contains only public information about a post or comment author.
type AuthorResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Post struct {
	ID        int64          `json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	AuthorID  int64          `json:"author_id"`
	Author    AuthorResponse `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Comment struct {
	ID        int64          `json:"id"`
	PostID    int64          `json:"post_id"`
	AuthorID  int64          `json:"author_id"`
	Author    AuthorResponse `json:"author"`
	Content   string         `json:"text"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type PostRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type CommentRequest struct {
	Content string `json:"text"`
}

type TokenResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

type PostListResponse struct {
	Posts  []Post `json:"posts"`
	Total  int64  `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type CommentListResponse struct {
	Comments []Comment `json:"comments"`
	Total    int64     `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
}
