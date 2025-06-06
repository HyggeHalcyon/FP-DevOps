package tests

import (
	"bytes"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"FP-DevOps/config"
	"FP-DevOps/constants" // Tambahkan import ini untuk CTX_KEY_USER_ID
	"FP-DevOps/controller"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid" // Tambahkan import ini untuk uuid.New()
	"github.com/stretchr/testify/assert"
)

func SetupFileController() controller.FileController {
	db := config.SetUpDatabaseConnection()
	fileRepo := repository.NewFileRepository(db)
	jwtService := config.NewJWTService()
	fileService := service.NewFileService(fileRepo)
	return controller.NewFileController(fileService, jwtService)
}

func CreateMultipartRequest(paramName, fileName string, fileContent []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(paramName, fileName)
	if err != nil {
		return nil, err
	}

	_, err = part.Write(fileContent)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "/api/upload", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func cleanUploadsDir(t *testing.T) {
	err := os.RemoveAll("uploads")
	if err != nil {
		t.Logf("Failed to cleanup uploads directory: %v", err)
	}
	err = os.MkdirAll("uploads", os.ModePerm)
	assert.NoError(t, err)
}

func Test_UploadFile_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	CleanUpTestUsers()
	users, err := InsertTestUser()
	assert.NoError(t, err)

	r := gin.Default()
	fileController := SetupFileController()

	// Simulasikan user ID di konteks Gin
	r.POST("/api/upload", func(ctx *gin.Context) {
		ctx.Set(constants.CTX_KEY_USER_ID, users[0].ID.String()) // Set user ID dummy
		fileController.Create(ctx)
	})

	fileContent := []byte("This is a test file.")
	req, err := CreateMultipartRequest("file", "test.txt", fileContent)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	savedFileContent, err := ioutil.ReadFile("uploads/test.txt")
	assert.NoError(t, err)
	assert.Equal(t, fileContent, savedFileContent)
}

func Test_UploadFile_TooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	r := gin.Default()
	r.MaxMultipartMemory = 20 << 20 // 20MB

	fileController := SetupFileController()

	// Simulasikan user ID di konteks Gin
	r.POST("/api/upload", func(ctx *gin.Context) {
		ctx.Set(constants.CTX_KEY_USER_ID, uuid.New().String()) // Set user ID dummy
		fileController.Create(ctx)
	})

	largeData := make([]byte, 21<<20) // 21MB
	req, err := CreateMultipartRequest("file", "large.txt", largeData)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Harapkan 500 Internal Server Error dari controller/service
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	_, err = os.Stat("uploads/large.txt")
	assert.True(t, os.IsNotExist(err), "File besar seharusnya tidak tersimpan")
}

func Test_UploadFile_NoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	r := gin.Default()
	fileController := SetupFileController()

	// Simulasikan user ID di konteks Gin
	r.POST("/api/upload", func(ctx *gin.Context) {
		ctx.Set(constants.CTX_KEY_USER_ID, uuid.New().String()) // Set user ID dummy
		fileController.Create(ctx)
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := writer.Close()
	assert.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/upload", body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
