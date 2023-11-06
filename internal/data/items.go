package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	filestorage "github.com/jesusangelm/api_galeria/internal/file_storage"
	"github.com/jesusangelm/api_galeria/internal/validator"
)

type Item struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	CreatedAt      time.Time      `json:"created_at"`
	CategoryID     int64          `json:"category_id"`
	Version        int32          `json:"version"`
	CategoryName   string         `json:"category_name,omitempty"` // extracted from join with categories table
	ImageURL       string         `json:"image_url,omitempty"`     // extracted from join with item_attachments table
	ItemAttachment ItemAttachment `json:"item_attachment,omitempty"`
}

type ItemModel struct {
	DB        *pgxpool.Pool
	S3Manager filestorage.S3
}

// Insert in DB a new Item based on the item struct given
func (m *ItemModel) Insert(item *Item) error {
	query := `
		INSERT INTO items (name, description, category_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, version
	`
	args := []interface{}{
		item.Name,
		item.Description,
		item.CategoryID,
	}

	return m.DB.QueryRow(context.Background(), query, args...).Scan(
		&item.ID,
		&item.CreatedAt,
		&item.Version,
	)
}

// Return a single item based on the ID given
func (m *ItemModel) Get(id int64) (*Item, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT
			items.id, items.name, items.description, items.created_at, items.version,
			items.category_id, categories.name AS category_name,
			COALESCE(item_attachments.filename, '') as filename,
			COALESCE(item_attachments.key, '') as key
		FROM items
		INNER JOIN categories ON categories.id = items.category_id
		LEFT JOIN item_attachments ON items.id = item_attachments.item_id
		WHERE items.id = $1
	`

	var item Item

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, id).Scan(
		&item.ID,
		&item.Name,
		&item.Description,
		&item.CreatedAt,
		&item.Version,
		&item.CategoryID,
		&item.CategoryName,
		&item.ItemAttachment.Filename,
		&item.ItemAttachment.Key,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	url := m.S3Manager.GetFileUrl(item.ItemAttachment.Key)
	item.ImageURL = url

	return &item, nil
}

func (m *ItemModel) Update(item *Item) error {
	query := `
		UPDATE items
		SET name = $1, description = $2, category_id = $3, version = version + 1
		WHERE id = $4 AND version = $5
		RETURNING version
	`

	args := []any{
		item.Name,
		item.Description,
		item.CategoryID,
		item.ID,
		item.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, args...).Scan(&item.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m *ItemModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
		DELETE FROM items
		WHERE id = $1
	`

	// SQL query to find the key of the ItemAttachment attached to the Item
	queryAttachment := `
		SELECT item_attachments.key
		FROM item_attachments
		WHERE item_id = $1
		LIMIT 1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	attachment := ItemAttachment{}

	// query queryAttachment execution
	err := m.DB.QueryRow(ctx, queryAttachment, id).Scan(
		&attachment.Key,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrRecordNotFound
		default:
			return err
		}
	}

	// Delete from S3 the file attached to the Item
	err = m.S3Manager.DeleteFile(attachment.Key)
	if err != nil {
		return err
	}

	result, err := m.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *ItemModel) List(name string, categoryID int, filters Filters) ([]*Item, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT
			count(*) OVER(), items.id, items.name, items.description, items.created_at,
			items.category_id, items.version, categories.name AS category_name,
			COALESCE(item_attachments.filename, '') AS filename,
			COALESCE(item_attachments.key, '') AS key
		FROM items
		INNER JOIN categories ON categories.id = items.category_id
		LEFT JOIN item_attachments on items.id = item_attachments.item_id
		WHERE (to_tsvector('simple', items.name) @@ plainto_tsquery('simple', $1) OR $1 = '')
		AND (items.category_id = $2 OR $2 = 0)
		ORDER by %s %s, id ASC
		LIMIT $3
		OFFSET $4
	`, filters.sortColumn(), filters.sortDirection())

	// 3 seconds timeout for quering the DB
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{name, categoryID, filters.limit(), filters.offset()}

	rows, err := m.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	var items []*Item

	for rows.Next() {
		var item Item

		err := rows.Scan(
			&totalRecords,
			&item.ID,
			&item.Name,
			&item.Description,
			&item.CreatedAt,
			&item.CategoryID,
			&item.Version,
			&item.CategoryName,
			&item.ItemAttachment.Filename,
			&item.ItemAttachment.Key,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		url := m.S3Manager.GetFileUrl(item.ItemAttachment.Key)
		item.ImageURL = url

		items = append(items, &item)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return items, metadata, nil
}

func ValidateItem(v *validator.Validator, item *Item) {
	v.Check(item.Name != "", "name", "must be provided")
	v.Check(len(item.Name) <= 100, "name", "must not be more than 100 bytes long")

	v.Check(item.Description != "", "description", "must be provided")
	v.Check(len(item.Description) <= 500, "description", "must not be more than 500 bytes long")
}

func ValidateItemCategoryID(v *validator.Validator, item *Item) {
	v.Check(item.CategoryID != 0, "category_id", "must be provided")
}
