package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/jesusangelm/api_galeria/internal/validator"
)

var ErrDuplicateEmail = errors.New("duplicate email")

type AdminUser struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"`
	Activated bool      `json:"activated"`
	CreatedAt time.Time `json:"created_at"`
	Version   int       `json:"-"`
}

type password struct {
	plaintext *string
	hash      []byte
}

type AdminUserModel struct {
	DB *pgxpool.Pool
}

// DB related Utilities
func (m *AdminUserModel) Insert(adminUser *AdminUser) error {
	query := `
    INSERT INTO admin_users (first_name, last_name, email, password_hash, activated)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, created_at, version
  `
	args := []any{
		adminUser.FirstName,
		adminUser.LastName,
		adminUser.Email,
		adminUser.Password.hash,
		adminUser.Activated,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, args...).Scan(
		&adminUser.ID,
		&adminUser.CreatedAt,
		&adminUser.Version,
	)
	if err != nil {
		switch {
		case err.Error() == `ERROR: duplicate key value violates unique constraint "admin_users_email_key" (SQLSTATE 23505)`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m *AdminUserModel) GetByEmail(email string) (*AdminUser, error) {
	query := `
    SELECT id, first_name, last_name, email, password_hash, activated, created_at, version
    FROM admin_users
    WHERE email = $1
  `
	var adminUser AdminUser

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, email).Scan(
		&adminUser.ID,
		&adminUser.FirstName,
		&adminUser.LastName,
		&adminUser.Email,
		&adminUser.Password.hash,
		&adminUser.Activated,
		&adminUser.CreatedAt,
		&adminUser.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &adminUser, nil
}

func (m *AdminUserModel) GetById(id int64) (*AdminUser, error) {
	query := `
    SELECT id, first_name, last_name, email, password_hash, activated, created_at, version
    FROM admin_users
    WHERE id = $1
  `
	var adminUser AdminUser

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, id).Scan(
		&adminUser.ID,
		&adminUser.FirstName,
		&adminUser.LastName,
		&adminUser.Email,
		&adminUser.Password.hash,
		&adminUser.Activated,
		&adminUser.CreatedAt,
		&adminUser.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &adminUser, nil
}

// Password hashing
func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

// Validations
func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateUser(v *validator.Validator, adminUser *AdminUser) {
	v.Check(adminUser.FirstName != "", "first name", "must be provided")
	v.Check(len(adminUser.FirstName) <= 500, "first name", "must not be more than 500 bytes long")

	v.Check(adminUser.LastName != "", "last name", "must be provided")
	v.Check(len(adminUser.LastName) <= 500, "last name", "must not be more than 500 bytes long")

	ValidateEmail(v, adminUser.Email)

	if adminUser.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *adminUser.Password.plaintext)
	}

	if adminUser.Password.hash == nil {
		panic("missing password hash for admin user")
	}
}
