package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"FP-DevOps/middleware"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func InsertDeleteUser() (entity.User, error) {
	db := config.SetUpDatabaseConnection()
	user := entity.User{
		ID:       uuid.New(),
		Username: "Rani2",
		Password: "password123",
	}
	if err := db.Create(&user).Error; err != nil {
		return entity.User{}, err
	}
	return user, nil
}

func InsertDeleteFiles(userID uuid.UUID) ([]entity.File, error) {
	db := config.SetUpDatabaseConnection()
	files := []entity.File{
		{
			ID:       uuid.New(),
			Filename: "daily.csv",
			UserID:   userID,
		},
	}
	for _, f := range files {
		if err := db.Create(&f).Error; err != nil {
			return nil, err
		}
	}
	return files, nil
}

func CleanDeletedata(userID uuid.UUID) {
	db := config.SetUpDatabaseConnection()
	db.Exec("DELETE FROM files")
	db.Exec("DELETE FROM users WHERE id = ?", userID)
}

func TestDeleteFile_Success(t *testing.T) {
	CleanDeletedata(uuid.Nil)

	user, err := InsertDeleteUser()
	assert.NoError(t, err)

	files, err := InsertDeleteFiles(user.ID)
	assert.NoError(t, err)
	assert.Len(t, files, 1)

	jwtSvc := config.NewJWTService()
	fileSvc := service.NewFileService(repository.NewFileRepository(config.SetUpDatabaseConnection()))
	fc := controller.NewFileController(fileSvc, jwtSvc)

	r := gin.Default()
	fileGroup := r.Group("/files")
	fileGroup.Use(middleware.Authenticate(jwtSvc))
	fileGroup.DELETE("/:id", fc.DeleteByID)

	token := jwtSvc.GenerateToken(user.ID.String(), user.Username)

	targetFileID := files[0].ID.String()
	req, _ := http.NewRequest("DELETE", "/files/"+targetFileID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Logf("TestDeleteFile_Success failed. Got status %d, body: %s", w.Code, w.Body.String())
	}

	assert.Equal(t, http.StatusOK, w.Code)

	var responseBody dto.Response
	err = json.Unmarshal(w.Body.Bytes(), &responseBody)
	assert.NoError(t, err)
	assert.True(t, responseBody.Status)
	assert.Equal(t, dto.MESSAGE_SUCCESS_DELETE_FILE, responseBody.Message)

	db := config.SetUpDatabaseConnection()
	var deletedFile entity.File
	err = db.First(&deletedFile, "id = ?", targetFileID).Error
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	CleanDeletedata(user.ID)
}

func TestDeleteFile_Unauthorized(t *testing.T) {
	CleanDeletedata(uuid.Nil)

	ownerUser, err := InsertDeleteUser()
	assert.NoError(t, err)
	ownerFiles, err := InsertDeleteFiles(ownerUser.ID)
	assert.NoError(t, err)
	assert.Len(t, ownerFiles, 1)

	otherUser := entity.User{
		ID:       uuid.New(),
		Username: "BukanRani",
		Password: "password123",
	}
	db := config.SetUpDatabaseConnection()
	err = db.Create(&otherUser).Error
	assert.NoError(t, err)

	jwtSvc := config.NewJWTService()
	fileSvc := service.NewFileService(repository.NewFileRepository(config.SetUpDatabaseConnection()))
	fc := controller.NewFileController(fileSvc, jwtSvc)

	r := gin.Default()
	fileGroup := r.Group("/files")
	fileGroup.Use(middleware.Authenticate(jwtSvc))
	fileGroup.DELETE("/:id", fc.DeleteByID)

	otherUserToken := jwtSvc.GenerateToken(otherUser.ID.String(), otherUser.Username)

	req, _ := http.NewRequest("DELETE", "/files/"+ownerFiles[0].ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+otherUserToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	t.Logf("TestDeleteFile_Unauthorized - Actual Status Code: %d", w.Code)
	t.Logf("TestDeleteFile_Unauthorized - Actual Response Body: %s", w.Body.String())

	if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Logf("TestDeleteFile_Unauthorized failed. Got status %d, body: %s", w.Code, w.Body.String())
	}

	assert.True(t, w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized)

	var responseBody dto.Response
	err = json.Unmarshal(w.Body.Bytes(), &responseBody)
	assert.NoError(t, err)
	assert.False(t, responseBody.Status)
	assert.Equal(t, dto.MESSAGE_FAILED_DELETE_FILE, responseBody.Message)
	if assertedError, ok := responseBody.Errors.(string); ok {
		assert.Equal(t, dto.ErrUnauthorizedFileAccess.Error(), assertedError)
	} else {
		t.Errorf("Expected responseBody.Errors to be a string, but got type %T with value %v", responseBody.Errors, responseBody.Errors)
	}

	var originalFile entity.File
	err = db.First(&originalFile, "id = ?", ownerFiles[0].ID).Error
	assert.NoError(t, err)
	assert.Equal(t, ownerFiles[0].Filename, originalFile.Filename)

	CleanDeletedata(ownerUser.ID)
	CleanDeletedata(otherUser.ID)
}
