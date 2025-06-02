package service

import (
	"FP-DevOps/constants"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"FP-DevOps/repository"
	"FP-DevOps/utils"
	"context"
	"io"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type (
	FileService interface {
		Create(context.Context, string, dto.CreateFileRequest) (dto.FileResponse, error)
		Update(context.Context, string, string, dto.FileUpdate) (dto.FileResponse, error)
		Delete(context.Context, string, string) error
		Get(context.Context, string, string) (dto.GetFileResponse, error)
	}

	fileService struct {
		fileRepo repository.FileRepository
	}
)

func NewFileService(ur repository.FileRepository) FileService {
	return &fileService{
		fileRepo: ur,
	}
}

func (s *fileService) Create(ctx context.Context, userID string, req dto.CreateFileRequest) (dto.FileResponse, error) {

	if req.File.Size > 20*constants.MB {
		return dto.FileResponse{}, dto.ErrFileSizeExceeded
	}

	file, err := req.File.Open()
	defer file.Close()
	if err != nil {
		return dto.FileResponse{}, err
	}

	// buffer := make([]byte, 512)
	buffer, err := io.ReadAll(file)
	if err != nil {
		return dto.FileResponse{}, err
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return dto.FileResponse{}, err
	}

	fileType := http.DetectContentType(buffer)
	fileExt := filepath.Ext(req.File.Filename)
	fileID := uuid.New()
	fileName := fileID.String() + fileExt

	filePath, err := s.fileRepo.WriteFile(userID, fileName, buffer)
	if err != nil {
		return dto.FileResponse{}, err
	}

	fileEntity := entity.File{
		ID:       fileID,
		Filename: utils.SanitizeFilename(req.File.Filename),
		Size:     req.File.Size,
		MimeType: fileType,
		UserID:   uuid.MustParse(userID),
		Path:     filePath,
	}
	if _, err := s.fileRepo.Create(fileEntity); err != nil {
		return dto.FileResponse{}, err
	}

	return dto.FileResponse{
		ID:       fileEntity.ID.String(),
		Filename: fileEntity.Filename,
		Size:     fileEntity.Size,
		MimeType: fileEntity.MimeType,
	}, err
}

func (s *fileService) Update(ctx context.Context, userID, fileID string, req dto.FileUpdate) (dto.FileResponse, error) {
	file, err := s.fileRepo.Get(fileID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return dto.FileResponse{}, dto.ErrFileNotFound
		}
		return dto.FileResponse{}, err
	}

	if file.UserID.String() != userID {
		return dto.FileResponse{}, dto.ErrUnauthorizedFileAccess
	}

	if _, err := s.fileRepo.Update(entity.File{
		ID:       uuid.MustParse(fileID),
		Filename: req.Filename,
	}); err != nil {
		return dto.FileResponse{}, err
	}

	return dto.FileResponse{
		ID:       file.ID.String(),
		Filename: req.Filename,
		Size:     file.Size,
		MimeType: file.MimeType,
	}, nil
}

func (s *fileService) Delete(ctx context.Context, userID, fileID string) error {
	file, err := s.fileRepo.Get(fileID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return dto.ErrFileNotFound
		}
		return err
	}

	if file.UserID.String() != userID {
		return dto.ErrUnauthorizedFileAccess
	}

	if err := s.fileRepo.DeleteFile(file); err != nil {
		return err
	}

	if err := s.fileRepo.Delete(fileID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return dto.ErrFileNotFound
		}
		return err
	}

	return nil
}

func (s *fileService) Get(ctx context.Context, userID, fileID string) (dto.GetFileResponse, error) {
	file, err := s.fileRepo.Get(fileID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return dto.GetFileResponse{}, dto.ErrFileNotFound
		}
		return dto.GetFileResponse{}, err
	}

	if file.UserID.String() != userID {
		return dto.GetFileResponse{}, dto.ErrUnauthorizedFileAccess
	}

	data, err := s.fileRepo.ReadFile(file)
	if err != nil {
		return dto.GetFileResponse{}, err
	}

	return dto.GetFileResponse{
		Content:  data,
		Filename: file.Filename,
		MimeType: file.MimeType,
	}, nil
}
