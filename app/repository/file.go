package repository

import (
	"FP-DevOps/constants"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"fmt"
	"math"
	"os"

	"gorm.io/gorm"
)

type (
	FileRepository interface {
		Get(string) (entity.File, error)
		GetPagination(string, string, int, int) ([]entity.File, int64, int64, error)
		Create(entity.File) (entity.File, error)
		Update(entity.File) (entity.File, error)
		Delete(string) error
		DeleteFile(entity.File) error
		WriteFile(string, string, []byte) (string, error)
		ReadFile(entity.File) ([]byte, error)
	}

	fileRepository struct {
		db *gorm.DB
	}
)

func NewFileRepository(db *gorm.DB) FileRepository {
	return &fileRepository{
		db: db,
	}
}

func (r *fileRepository) Get(fileID string) (entity.File, error) {
	var file entity.File
	if err := r.db.Where("id = ?", fileID).First(&file).Error; err != nil {
		return entity.File{}, err
	}

	return file, nil
}

func (r *fileRepository) GetPagination(userID, search string, limit, page int) ([]entity.File, int64, int64, error) {
	var files []entity.File
	var count int64

	if search != "" {
		err := r.db.Model(&entity.File{}).Where("user_id = ?", userID).Where("filename LIKE ?", "%"+search+"%").Count(&count).Error
		if err != nil {
			return nil, 0, 0, err
		}
	} else {
		err := r.db.Where("user_id = ?", userID).Model(&entity.File{}).Count(&count).Error
		if err != nil {
			return nil, 0, 0, err
		}
	}

	maxPage := int64(math.Ceil(float64(count) / float64(limit)))
	offset := (page - 1) * limit

	err := r.db.Where("user_id = ?", userID).Where("filename LIKE ?", "%"+search+"%").Offset(offset).Limit(limit).Find(&files).Error
	if err != nil {
		return nil, 0, 0, err
	}

	return files, maxPage, count, nil
}

func (r *fileRepository) Create(file entity.File) (entity.File, error) {
	if err := r.db.Create(&file).Error; err != nil {
		return entity.File{}, err
	}

	return file, nil
}

func (r *fileRepository) Update(file entity.File) (entity.File, error) {
	if err := r.db.Updates(&file).Error; err != nil {
		return entity.File{}, err
	}
	return file, nil
}

func (r *fileRepository) Delete(fileID string) error {
	if err := r.db.Where("id = ?", fileID).Delete(&entity.File{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *fileRepository) WriteFile(userID, fileName string, content []byte) (string, error) {
	directory := fmt.Sprintf("%s/%s/", constants.FILE_STORAGE_DIRECTORY, userID)
	if err := os.MkdirAll(directory, os.ModePerm); err != nil {
		return "", err
	}

	filePath := fmt.Sprintf("%s%s", directory, fileName)
	newFile, err := os.Create(filePath)
	defer newFile.Close()
	if err != nil {
		return "", err
	}

	_, err = newFile.Write(content)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

func (r *fileRepository) DeleteFile(file entity.File) error {
	if _, err := os.Lstat(file.Path); err != nil {
		if os.IsNotExist(err) {
			return r.db.Where("id = ?", file.ID.String()).Delete(&entity.File{}).Error
		}
	}

	if err := os.Remove(file.Path); err != nil {
		return err
	}

	return r.db.Where("id = ?", file.ID.String()).Delete(&entity.File{}).Error
}

func (r *fileRepository) ReadFile(file entity.File) ([]byte, error) {
	content, err := os.ReadFile(file.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, dto.ErrFileNotFound
		}
		return nil, err
	}

	return content, nil
}
