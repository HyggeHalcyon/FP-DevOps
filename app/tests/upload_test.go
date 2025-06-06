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
	"FP-DevOps/controller"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// SetupFileController: gunakan FileController, bukan UserController
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
	// Buat ulang folder uploads
	err = os.MkdirAll("uploads", os.ModePerm)
	assert.NoError(t, err)
}

func Test_UploadFile_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Bersihkan dan buat ulang folder uploads
	cleanUploadsDir(t)
	// Pastikan cleanup lagi setelah test selesai
	t.Cleanup(func() { cleanUploadsDir(t) })

	r := gin.Default()
	fileController := SetupFileController()
	r.POST("/api/upload", fileController.Create)

	fileContent := []byte("This is a test file.")
	req, err := CreateMultipartRequest("file", "test.txt", fileContent)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verifikasi file tersimpan dengan isi yang sama
	savedFileContent, err := ioutil.ReadFile("uploads/test.txt")
	assert.NoError(t, err)
	assert.Equal(t, fileContent, savedFileContent)
}

func Test_UploadFile_TooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Bersihkan folder uploads sebelum dan sesudah test
	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	r := gin.Default()
	r.MaxMultipartMemory = 20 << 20 // 20MB

	fileController := SetupFileController()
	r.POST("/api/upload", fileController.Create)

	// Buat data 21MB > max memory 20MB
	largeData := make([]byte, 21<<20)
	req, err := CreateMultipartRequest("file", "large.txt", largeData)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Pastikan file besar tidak tersimpan
	_, err = os.Stat("uploads/large.txt")
	assert.True(t, os.IsNotExist(err), "File besar seharusnya tidak tersimpan")
}

func Test_UploadFile_NoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	r := gin.Default()
	fileController := SetupFileController()
	r.POST("/api/upload", fileController.Create)

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
