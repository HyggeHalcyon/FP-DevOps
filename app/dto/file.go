package dto

import (
	"errors"
	"mime/multipart"
)

const (
	MESSAGE_FAILED_CREATE_FILE = "failed create file"
	MESSAGE_FAILED_UPDATE_FILE = "failed update file"
	MESSAGE_FAILED_DELETE_FILE = "failed delete file"
	MESSAGE_FAILED_GET_FILE    = "failed get file"

	MESSAGE_SUCCESS_CREATE_FILE = "success create file"
	MESSAGE_SUCCESS_UPDATE_FILE = "success update file"
	MESSAGE_SUCCESS_DELETE_FILE = "success delete file"
	MESSAGE_SUCCESS_GET_FILE    = "success get file"
)

var (
	ErrFileSizeExceeded       = errors.New("file size exceeds the limit of 20MB")
	ErrFileNotFound           = errors.New("file not found")
	ErrUnauthorizedFileAccess = errors.New("unauthorized file access, you can only access your own files")
)

type (

	Response struct {
		Status  bool        `json:"status"`
		Message string      `json:"message"`
		Errors  interface{} `json:"error,omitempty"` 
		Data    interface{} `json:"data,omitempty"`
		Meta    interface{} `json:"meta,omitempty"`
	}

	CreateFileRequest struct {
		File *multipart.FileHeader `json:"file" form:"file" binding:"required"`
	}

	FileUpdate struct {
		Filename  string `json:"filename" form:"filename"`
		Shareable *bool  `json:"shareable" form:"shareable"`
	}

	FileResponse struct {
		ID        string `json:"id" form:"id"`
		Filename  string `json:"filename" form:"filename"`
		Size      int64  `json:"size" form:"size"`
		MimeType  string `json:"mime_type" form:"mime_type"`
		Shareable *bool  `json:"shareable" form:"shareable"`

		Content []byte `json:"content,omitempty" form:"content,omitempty"`
	}

	FilePaginationResponse struct {
		Data []FileResponse `json:"data"`
		PaginationMetadata
	}
)
