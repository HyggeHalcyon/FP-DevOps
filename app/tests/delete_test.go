package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"FP-DevOps/config"
	"FP-DevOps/constants"
	"FP-DevOps/controller"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"FP-DevOps/repository"
	"FP-DevOps/service"
)

func CleanUpTestUserss() {
	db := config.SetUpDatabaseConnection()
	if err := db.Exec("DELETE FROM users").Error; err != nil {
		panic(fmt.Errorf("failed to cleanup test users: %w", err))
	}
}

func InsertTestUserr() string {
	db := config.SetUpDatabaseConnection()
	testUsers := []entity.User{
		{ID: uuid.New(), Username: "testuser1", Password: "password"},
		{ID: uuid.New(), Username: "testuser2", Password: "password"},
	}
	for _, user := range testUsers {
		if err := db.Create(&user).Error; err != nil {
			panic(fmt.Errorf("failed to insert test user: %w", err))
		}
	}
	return testUsers[0].ID.String()
}

func cleanUploadsDirr(t *testing.T) {
	err := os.RemoveAll("uploads")
	if err != nil {
		t.Logf("Failed to cleanup uploads directory: %v", err)
	}
	err = os.MkdirAll("uploads", os.ModePerm)
	assert.NoError(t, err)
}

func addUserIDToContext(req *http.Request, userID string) *http.Request {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	c.Set(constants.CTX_KEY_USER_ID, userID)
	return req
}

func uploadTestFile(t *testing.T, router *gin.Engine, userID string) string {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileWriter, err := writer.CreateFormFile("file", "dummy.txt")
	assert.NoError(t, err)

	_, err = fileWriter.Write([]byte("test file content"))
	assert.NoError(t, err)

	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = addUserIDToContext(req, userID)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusCreated, resp.Code, "Upload test file seharusnya sukses (201 Created)")

	type UploadSuccessResponse struct {
		Data dto.FileResponse `json:"data"`
	}
	var uploadRes UploadSuccessResponse
	err = json.Unmarshal(resp.Body.Bytes(), &uploadRes)
	assert.NoError(t, err, "Gagal mem-parse respons JSON dari uploadTestFile")
	assert.NotEmpty(t, uploadRes.Data.ID, "ID file seharusnya tidak kosong setelah uploadTestFile")

	return uploadRes.Data.ID
}

func setupDeleteTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	db := config.SetUpDatabaseConnection()
	r := gin.Default()
	fileController := controller.NewFileController(
		service.NewFileService(repository.NewFileRepository(db)),
		config.NewJWTService(),
	)
	r.POST("/api/upload", func(ctx *gin.Context) {
		fileController.Create(ctx)
	})
	r.DELETE("/api/files/:id", func(ctx *gin.Context) {
		fileController.DeleteByID(ctx)
	})
	r.GET("/api/file/:id", func(ctx *gin.Context) {
		fileController.GetFileByID(ctx)
	})
	return r
}

func Test_DeleteFile_OK(t *testing.T) {
	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	CleanUpTestUsers()
	userID := InsertTestUserr()

	router := setupDeleteTestRouter()
	fileID := uploadTestFile(t, router, userID)

	req, _ := http.NewRequest("DELETE", "/api/files/"+fileID, nil)
	req = addUserIDToContext(req, userID)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "success delete file")

	savedFileName := fileID + filepath.Ext("dummy.txt")
	savedFilePath := filepath.Join("uploads", userID, savedFileName)
	_, err := os.Stat(savedFilePath)
	assert.True(t, os.IsNotExist(err), "File fisik seharusnya sudah tidak ada di disk")

	fileRepo := repository.NewFileRepository(config.SetUpDatabaseConnection())
	_, err = fileRepo.Get(fileID)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func Test_DeleteFile_NotFound(t *testing.T) {
	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	CleanUpTestUsers()
	userID := InsertTestUserr()
	router := setupDeleteTestRouter()

	randomFileID := uuid.New().String()
	req, _ := http.NewRequest("DELETE", "/api/files/"+randomFileID, nil)
	req = addUserIDToContext(req, userID)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "file not found")
}

func Test_DeleteFile_Unauthorized(t *testing.T) {
	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	CleanUpTestUsers()
	userA_ID := InsertTestUserr()
	userB_ID := InsertTestUserr()
	router := setupDeleteTestRouter()

	fileID := uploadTestFile(t, router, userA_ID)

	req, _ := http.NewRequest("DELETE", "/api/files/"+fileID, nil)
	req = addUserIDToContext(req, userB_ID)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "unauthorized file access")

	savedFileName := fileID + filepath.Ext("dummy.txt")
	savedFilePath := filepath.Join("uploads", userA_ID, savedFileName)
	_, err := os.Stat(savedFilePath)
	assert.NoError(t, err, "File fisik seharusnya masih ada di disk")

	fileRepo := repository.NewFileRepository(config.SetUpDatabaseConnection())
	var retrievedFile entity.File
	_, err = fileRepo.Get(fileID)
	assert.NoError(t, err, "Metadata file seharusnya masih ada di DB")
	assert.Equal(t, userA_ID, retrievedFile.UserID.String(), "File seharusnya masih dimiliki oleh userA")
}

func Test_DeleteFile_NoAuth(t *testing.T) {
	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	CleanUpTestUsers()
	userID := InsertTestUserr()

	router := setupDeleteTestRouter()
	fileID := uploadTestFile(t, router, userID)

	req, _ := http.NewRequest("DELETE", "/api/files/"+fileID, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}