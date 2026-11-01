package postgres

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"context"
	"database/sql"
	"fmt"
)

type PostRepository struct{ db *sql.DB }

func NewPostRepository(db *sql.DB) *PostRepository { return &PostRepository{db: db} }

const postColumns = `p.id, p.title, p.content, p.author_id, p.created_at, p.updated_at,
 u.id, u.username`

type rowScanner interface{ Scan(...any) error }

func scanPost(row rowScanner) (model.Post, error) {
	var p model.Post
	err := row.Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.CreatedAt, &p.UpdatedAt,
		&p.Author.ID, &p.Author.Username)
	return p, err
}

func (r *PostRepository) Create(ctx context.Context, post model.Post) (model.Post, error) {
	const query = `WITH created AS (
  INSERT INTO posts (title, content, author_id) VALUES ($1, $2, $3) RETURNING *
 ) SELECT ` + postColumns + ` FROM created p JOIN users u ON u.id = p.author_id`
	created, err := scanPost(r.db.QueryRowContext(ctx, query, post.Title, post.Content, post.AuthorID))
	if err != nil {
		return model.Post{}, translateError(err, "create post")
	}
	return created, nil
}

func (r *PostRepository) GetByID(ctx context.Context, id int64) (model.Post, error) {
	const query = `SELECT ` + postColumns + ` FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = $1`
	post, err := scanPost(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return model.Post{}, translateError(err, "get post")
	}
	return post, nil
}

func (r *PostRepository) GetAll(ctx context.Context, limit, offset int) ([]model.Post, int64, error) {
	// Count and page use the same snapshot, even when offset is past the last row.
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, fmt.Errorf("begin list posts: %w", err)
	}
	defer tx.Rollback()
	var total int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}
	const query = `SELECT ` + postColumns + ` FROM posts p JOIN users u ON u.id = p.author_id
  ORDER BY p.created_at DESC, p.id DESC LIMIT $1 OFFSET $2`
	rows, err := tx.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()
	posts := make([]model.Post, 0)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate posts: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("finish list posts: %w", err)
	}
	return posts, total, nil
}

func (r *PostRepository) Update(ctx context.Context, post model.Post) (model.Post, error) {
	const query = `WITH updated AS (
  UPDATE posts SET title = $1, content = $2, updated_at = CURRENT_TIMESTAMP
  WHERE id = $3 AND author_id = $4 RETURNING *
 ) SELECT ` + postColumns + ` FROM updated p JOIN users u ON u.id = p.author_id`
	updated, err := scanPost(r.db.QueryRowContext(ctx, query, post.Title, post.Content, post.ID, post.AuthorID))
	if err != nil {
		return model.Post{}, translateError(err, "update post")
	}
	return updated, nil
}

func (r *PostRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM posts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted post count: %w", err)
	}
	if rowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

var _ repository.PostRepository = (*PostRepository)(nil)
