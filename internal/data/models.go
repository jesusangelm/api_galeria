package data

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	filestorage "github.com/jesusangelm/api_galeria/internal/file_storage"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Categories     CategoryModel
	Items          ItemModel
	ItemAttachment ItemAttachmentModel
	AdminUser      AdminUserModel
}

func NewModels(db *pgxpool.Pool, s3Manager filestorage.S3) Models {
	return Models{
		Categories:     CategoryModel{DB: db, S3Manager: s3Manager},
		Items:          ItemModel{DB: db, S3Manager: s3Manager},
		ItemAttachment: ItemAttachmentModel{DB: db},
		AdminUser:      AdminUserModel{DB: db},
	}
}
