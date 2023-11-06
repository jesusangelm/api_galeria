package data

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemAttachment struct {
	ID          int64     `json:"id,omitempty"`
	Key         string    `json:"key,omitempty"`
	Filename    string    `json:"filename,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	ByteSize    int64     `json:"byte_size,omitempty"`
	CreatedAt   time.Time `json:"-"`
	ItemID      int64     `json:"item_id,omitempty"`
}

type ItemAttachmentModel struct {
	DB *pgxpool.Pool
}

func (m *ItemAttachmentModel) Insert(itemAttachment *ItemAttachment) error {
	query := `
		INSERT INTO item_attachments (key, filename, content_type, byte_size, item_id)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	args := []interface{}{
		itemAttachment.Key,
		itemAttachment.Filename,
		itemAttachment.ContentType,
		itemAttachment.ByteSize,
		itemAttachment.ItemID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRow(ctx, query, args...).Scan(
		&itemAttachment.ID,
		&itemAttachment.CreatedAt,
	)
}
