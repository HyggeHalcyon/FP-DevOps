package tests

import (
	"bytes"
	"encoding/json"
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

// ==== Helpers ====

func cleanUploads(t *testing.T) {
	_ = os.RemoveAll("uploads")
	_ = os.MkdirAll("uploads", os.ModePerm)
}

func resetUsers() string {
	db := config.SetUpDatabaseConnection()
	db.Exec("DELETE FROM users")

	user := entity.User{ID: uuid.New(), Username: "testuser", Password: "password"}
	db.Create(&user)
	return user.ID.String()
}

func addUserID(req *http.Request, userID string) *http.Request {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	c.Set(constants.CTX_KEY_USER_ID, userID)
	return req
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	db := config.SetUpDatabaseConnection()
	r := gin.Default()

	fileController := controller.NewFileController(
		service.NewFileService(repository.NewFileRepository(db)),
		config.NewJWTService(),
	)

	r.POST("/api/upload", fileController.Create)
	r.DELETE("/api/files/:id", fileController.DeleteByID)
	r.GET("/api/file/:id", fileController.GetFileByID)

	return r
}

func uploadFile(t *testing.T, router *gin.Engine, userID string) string {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", "dummy.txt")
	part.Write([]byte("test content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = addUserID(req, userID)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)

	var result struct {
		Data dto.FileResponse `json:"data"`
	}
	_ = json.Unmarshal(resp.Body.Bytes(), &result)
	return result.Data.ID
}

// ==== Tests ====

func Test_DeleteFile_OK(t *testing.T) {
	cleanUploads(t)
	t.Cleanup(func() { cleanUploads(t) })

	userID := resetUsers()
	router := setupRouter()

	fileID := uploadFile(t, router, userID)
	req, _ := http.NewRequest("DELETE", "/api/files/"+fileID, nil)
	req = addUserID(req, userID)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "success delete file")

	// Cek file fisik dihapus
	filePath := filepath.Join("uploads", userID, fileID+filepath.Ext("dummy.txt"))
	_, err := os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))

	// Cek metadata file dihapus
	_, err = repository.NewFileRepository(config.SetUpDatabaseConnection()).Get(fileID)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// func Test_DeleteFile_NotFound(t *testing.T) {
// 	cleanUploads(t)
// 	t.Cleanup(func() { cleanUploads(t) })

// 	userID := resetUsers()
// 	router := setupRouter()

// 	randomID := uuid.New().String()
// 	req, _ := http.NewRequest("DELETE", "/api/files/"+randomID, nil)
// 	req = addUserID(req, userID)

// 	resp := httptest.NewRecorder()
// 	router.ServeHTTP(resp, req)

// 	assert.Equal(t, http.StatusInternalServerError, resp.Code)
// 	assert.Contains(t, resp.Body.String(), "file not found")
// }

// func Test_DeleteFile_Unauthorized(t *testing.T) {
// 	cleanUploads(t)
// 	t.Cleanup(func() { cleanUploads(t) })

// 	userA := resetUsers()
// 	userB := resetUsers()
// 	router := setupRouter()

// 	fileID := uploadFile(t, router, userA)
// 	req, _ := http.NewRequest("DELETE", "/api/files/"+fileID, nil)
// 	req = addUserID(req, userB)

// 	resp := httptest.NewRecorder()
// 	router.ServeHTTP(resp, req)

// 	assert.Equal(t, http.StatusInternalServerError, resp.Code)
// 	assert.Contains(t, resp.Body.String(), "unauthorized file access")
// }

// func Test_DeleteFile_NoAuth(t *testing.T) {
// 	cleanUploads(t)
// 	t.Cleanup(func() { cleanUploads(t) })

// 	userID := resetUsers()
// 	router := setupRouter()

// 	fileID := uploadFile(t, router, userID)
// 	req, _ := http.NewRequest("DELETE", "/api/files/"+fileID, nil)

// 	resp := httptest.NewRecorder()
// 	router.ServeHTTP(resp, req)

// 	assert.Equal(t, http.StatusUnauthorized, resp.Code)
// }
