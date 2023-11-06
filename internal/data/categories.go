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

// struct to represent the Category model
type Category struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Version     int32     `json:"version"`
	Items       []*Item   `json:"items,omitempty"`
	ItemsCount  int64     `json:"items_count"`
}

type CategoryModel struct {
	DB        *pgxpool.Pool
	S3Manager filestorage.S3
}

// Insert in DB a new Category based on the category given
func (m *CategoryModel) Insert(category *Category) error {
	query := `
		INSERT INTO categories (name, description)
		VALUES ($1, $2)
		RETURNING id, created_at, version
	`
	args := []interface{}{category.Name, category.Description}

	// This will mutate the category struct to add the category.ID
	// and the category.CreatedAt taken from the recent created record in DB
	return m.DB.QueryRow(context.Background(), query, args...).Scan(
		&category.ID,
		&category.CreatedAt,
		&category.Version,
	)
}

// Return a single category based on the ID given
func (m *CategoryModel) Get(id int64) (*Category, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT
			categories.id, categories.name, categories.description,
			categories.created_at, categories.version, COUNT(items.id) AS items_count
		FROM categories
		LEFT JOIN items ON categories.id = items.category_id
		WHERE categories.id = $1
		GROUP BY categories.id
	`
	var category Category

	err := m.DB.QueryRow(ctx, query, id).Scan(
		&category.ID,
		&category.Name,
		&category.Description,
		&category.CreatedAt,
		&category.Version,
		&category.ItemsCount,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	// query to get the items in a given category
	query = `
		SELECT items.id, items.name, items.description, items.created_at,
				items.version, COALESCE(item_attachments.filename, '') as filename
		FROM items
		LEFT JOIN item_attachments ON items.id = item_attachments.item_id
		WHERE items.category_id = $1
		ORDER BY items.created_at DESC
	`
	rows, err := m.DB.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		var item Item
		err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.CreatedAt,
			&item.Version,
			&item.ItemAttachment.Filename,
		)
		if err != nil {
			return nil, err
		}

		url := m.S3Manager.GetFileUrl(item.ItemAttachment.Filename)
		item.ImageURL = url

		items = append(items, &item)
	}
	category.Items = items

	return &category, nil
}

func (m *CategoryModel) Update(category *Category) error {
	query := `
		UPDATE categories
		SET name = $1, description = $2, version = version + 1
		WHERE id = $3 AND version = $4
		RETURNING version
	`

	args := []any{
		category.Name,
		category.Description,
		category.ID,
		category.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, args...).Scan(&category.Version)
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

func (m *CategoryModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
		DELETE FROM categories
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

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

// Return a slice of categories.
func (m *CategoryModel) List(name string, filters Filters) ([]*Category, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT
			count(*) OVER(), categories.id, categories.name, categories.description,
			categories.created_at, categories.version, COUNT(items.id) AS items_count
		FROM categories
		LEFT JOIN items ON categories.id = items.category_id
		WHERE (to_tsvector('simple', categories.name) @@ plainto_tsquery('simple', $1) OR $1 = '')
		GROUP BY categories.id
		ORDER BY %s %s, id ASC
		LIMIT $2
		OFFSET $3
	`, filters.sortColumn(), filters.sortDirection())

	// 3 seconds timeout for quering the DB
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{name, filters.limit(), filters.offset()}

	rows, err := m.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	var categories []*Category

	for rows.Next() {
		var category Category
		err := rows.Scan(
			&totalRecords,
			&category.ID,
			&category.Name,
			&category.Description,
			&category.CreatedAt,
			&category.Version,
			&category.ItemsCount,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		categories = append(categories, &category)
	}
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return categories, metadata, nil
}

func ValidateCategory(v *validator.Validator, category *Category) {
	v.Check(category.Name != "", "name", "must be provided")
	v.Check(len(category.Name) <= 100, "name", "must not be more than 100 bytes long")

	v.Check(category.Description != "", "description", "must be provided")
	v.Check(len(category.Description) <= 500, "description", "must not be more than 500 bytes long")
}
