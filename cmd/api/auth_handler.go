package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/golang-jwt/jwt/v4"

	"github.com/jesusangelm/api_galeria/internal/data"
	"github.com/jesusangelm/api_galeria/internal/validator"
)

func (app *application) authenticate(w http.ResponseWriter, r *http.Request) {
	// read json payload
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// validate user input payload
	v := validator.New()
	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// validate user against database
	adminUser, err := app.models.AdminUser.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// check password
	match, err := adminUser.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	// create a jwt adminUser
	au := jwtAdmin{
		ID:        adminUser.ID,
		FirstName: adminUser.FirstName,
		LastName:  adminUser.LastName,
	}

	// generate tokens
	tokens, err := app.config.auth.GenerateTokenPair(&au)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	refreshCookie := app.config.auth.GetRefreshCookie(tokens.RefreshToken)
	http.SetCookie(w, refreshCookie)

	err = app.writeJSON(w, http.StatusOK, envelope{"tokens": tokens}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) refreshToken(w http.ResponseWriter, r *http.Request) {
	for _, cookie := range r.Cookies() {
		if cookie.Name == app.config.auth.CookieName {
			claims := &Claims{}
			refreshToken := cookie.Value

			// parse the token to get the claims
			_, err := jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(app.config.JWTSecret), nil
			})
			if err != nil {
				app.errorResponse(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
				return
			}

			// get the user id from the token claims
			adminUserId, err := strconv.Atoi(claims.Subject)
			if err != nil {
				app.errorResponse(w, r, http.StatusUnauthorized, errors.New("unknow admin user"))
				return
			}

			adminUser, err := app.models.AdminUser.GetById(int64(adminUserId))
			if err != nil {
				app.errorResponse(w, r, http.StatusUnauthorized, errors.New("unknow admin user"))
				return
			}

			au := jwtAdmin{
				ID:        adminUser.ID,
				FirstName: adminUser.FirstName,
				LastName:  adminUser.LastName,
			}

			tokenPairs, err := app.config.auth.GenerateTokenPair(&au)
			if err != nil {
				app.errorResponse(w, r, http.StatusUnauthorized, errors.New("error generating token"))
				return
			}

			http.SetCookie(w, app.config.auth.GetRefreshCookie(tokenPairs.RefreshToken))
			err = app.writeJSON(w, http.StatusOK, envelope{"tokens": tokenPairs}, nil)
			if err != nil {
				app.serverErrorResponse(w, r, err)
			}
		}
	}
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, app.config.auth.GetExpiredRefreshCookie())
	w.WriteHeader(http.StatusAccepted)
}
