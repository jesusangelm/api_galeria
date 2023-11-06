package data

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	filestorage "github.com/jesusangelm/api_galeria/internal/file_storage"
)

// Define a custom ErrRecordNotFound error. We'll return this from our Get() method when
// looking up a movie that doesn't exist in our database.
var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

// Create a Models struct which wraps the CategoryModel. We'll add other models to this,
// later
type Models struct {
	Categories     CategoryModel
	Items          ItemModel
	ItemAttachment ItemAttachmentModel
}

// For ease of use, we also add a New() method which returns a Models struct containing
func NewModels(db *pgxpool.Pool, s3Manager filestorage.S3) Models {
	return Models{
		Categories:     CategoryModel{DB: db, S3Manager: s3Manager},
		Items:          ItemModel{DB: db, S3Manager: s3Manager},
		ItemAttachment: ItemAttachmentModel{DB: db},
	}
}
