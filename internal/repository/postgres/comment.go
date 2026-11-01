package postgres

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"context"
	"database/sql"
	"fmt"
)

type CommentRepository struct{ db *sql.DB }

func NewCommentRepository(db *sql.DB) *CommentRepository { return &CommentRepository{db: db} }

const commentColumns = `c.id, c.post_id, c.author_id, c.content, c.created_at, c.updated_at,
 u.id, u.username`

func scanComment(row rowScanner) (model.Comment, error) {
	var c model.Comment
	err := row.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.ID, &c.Author.Username)
	return c, err
}

func (r *CommentRepository) Create(ctx context.Context, comment model.Comment) (model.Comment, error) {
	const query = `WITH created AS (
  INSERT INTO comments (post_id, author_id, content) VALUES ($1, $2, $3) RETURNING *
 ) SELECT ` + commentColumns + ` FROM created c JOIN users u ON u.id = c.author_id`
	created, err := scanComment(r.db.QueryRowContext(ctx, query, comment.PostID, comment.AuthorID, comment.Content))
	if err != nil {
		return model.Comment{}, translateError(err, "create comment")
	}
	return created, nil
}

func (r *CommentRepository) GetByPostID(ctx context.Context, postID int64, limit, offset int) ([]model.Comment, int64, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, fmt.Errorf("begin list comments: %w", err)
	}
	defer tx.Rollback()
	var total int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE post_id = $1`, postID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count comments: %w", err)
	}
	const query = `SELECT ` + commentColumns + ` FROM comments c JOIN users u ON u.id = c.author_id
  WHERE c.post_id = $1 ORDER BY c.created_at ASC, c.id ASC LIMIT $2 OFFSET $3`
	rows, err := tx.QueryContext(ctx, query, postID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()
	comments := make([]model.Comment, 0)
	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate comments: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("finish list comments: %w", err)
	}
	return comments, total, nil
}

var _ repository.CommentRepository = (*CommentRepository)(nil)
