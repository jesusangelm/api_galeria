package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jesusangelm/api_galeria/internal/data"
	"github.com/jesusangelm/api_galeria/internal/validator"
)

func (app *application) createItem(w http.ResponseWriter, r *http.Request) {
	// declare a struct to hold the information we expect to receive
	// this struct will be the target decode destination
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		CategoryID  int64  `json:"category_id"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	item := &data.Item{
		Name:        input.Name,
		Description: input.Description,
		CategoryID:  input.CategoryID,
	}

	v := validator.New()

	data.ValidateItem(v, item)
	data.ValidateItemCategoryID(v, item)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Items.Insert(item)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// utility header
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/items/%d", item.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"item": item}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) multipartCreateItem(w http.ResponseWriter, r *http.Request) {
	// reference for multiple files support https://freshman.tech/file-upload-golang/

	// Max 10MB files
	maxUploadSize := 10_485_760 // 10MB

	r.Body = http.MaxBytesReader(w, r.Body, int64(maxUploadSize))
	if err := r.ParseMultipartForm(int64(maxUploadSize)); err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("File must not be larger than %d bytes", maxUploadSize))
		return
	}

	file, handler, err := r.FormFile("item_file")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer file.Close()

	// only JPEG OR PNG allowed
	fileType := handler.Header.Get("Content-Type")
	if fileType != "image/jpeg" && fileType != "image/png" {
		app.badRequestResponse(w, r, fmt.Errorf("File format %s not allowed. Please upload a JPEG or PNG image", fileType))
		return
	}

	categoryID, err := strconv.Atoi(r.FormValue("category_id"))
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	item := &data.Item{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		CategoryID:  int64(categoryID),
	}

	v := validator.New()

	data.ValidateItem(v, item)
	data.ValidateItemCategoryID(v, item)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Items.Insert(item)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	attachment, err := app.s3Manager.UploadFile(file, *handler)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	itemAttachment := &data.ItemAttachment{
		Key:         attachment.Key,
		Filename:    attachment.Filename,
		ContentType: attachment.ContentType,
		ByteSize:    attachment.ByteSize,
		ItemID:      item.ID,
	}

	err = app.models.ItemAttachment.Insert(itemAttachment)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	item.ItemAttachment = *itemAttachment
	item.ImageURL = app.s3Manager.GetFileUrl(itemAttachment.Key)

	// utility header
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/items/%d", item.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"item": item}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showItem(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	item, err := app.models.Items.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"item": item}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateItem(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	item, err := app.models.Items.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// we use pointers here for support partial update
	var input struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		CategoryID  *int64  `json:"category_id"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Name != nil {
		item.Name = *input.Name
	}
	if input.Description != nil {
		item.Description = *input.Description
	}
	if input.CategoryID != nil {
		item.CategoryID = *input.CategoryID
	}

	v := validator.New()

	if data.ValidateItem(v, item); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Items.Update(item)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"item": item}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.Items.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "item successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) listItems(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name       string
		CategoryID int
		data.Filters
	}

	v := validator.New()
	qs := r.URL.Query() // To get filter parameters from the QueryString

	input.Name = app.readString(qs, "name", "")
	input.CategoryID = app.readInt(qs, "category_id", 0, v)
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "id")
	input.Filters.SortSafeList = []string{
		"id", "name", "created_at", "-id", "-name", "-created_at",
	}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	items, metadata, err := app.models.Items.List(input.Name, input.CategoryID, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"items": items, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
