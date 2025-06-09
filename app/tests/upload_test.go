package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"FP-DevOps/config"
	"FP-DevOps/constants"
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

// Insert a single test user and return it
func InsertUploadUser() (entity.User, error) {
	db := config.SetUpDatabaseConnection()
	user := entity.User{
		ID:       uuid.New(),
		Username: "Rani",
		Password: "password123", // adjust if you hash passwords in your model
	}
	if err := db.Create(&user).Error; err != nil {
		return entity.User{}, err
	}
	return user, nil
}

func createDummyFile(size int64) (io.Reader, string, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "test_file.bin")
	if err != nil {
		return nil, "", err
	}

	_, err = io.CopyN(part, bytes.NewReader(bytes.Repeat([]byte("a"), int(size))), size)
	if err != nil {
		return nil, "", err
	}

	writer.Close()

	return body, writer.FormDataContentType(), nil
}

func CleanUpUploadData(userID uuid.UUID) {
	db := config.SetUpDatabaseConnection()
	db.Exec("DELETE FROM files")
	db.Exec("DELETE FROM users WHERE id = ?", userID)
}

func TestUploadFile_Success(t *testing.T) {
	CleanUpTestData(uuid.Nil) // Bersihkan data sebelum tes untuk isolasi

	user, err := InsertFileUser()
	assert.NoError(t, err)

	jwtSvc := config.NewJWTService()
	fileSvc := service.NewFileService(repository.NewFileRepository(config.SetUpDatabaseConnection()))
	fc := controller.NewFileController(fileSvc, jwtSvc)

	r := gin.Default()
	r.MaxMultipartMemory = 25 * constants.MB // Max 25MB for Gin's parsing, so our 20MB limit works

	fileGroup := r.Group("/files")
	fileGroup.Use(middleware.Authenticate(jwtSvc)) // Terapkan middleware autentikasi
	fileGroup.POST("/upload", fc.Create)

	token := jwtSvc.GenerateToken(user.ID.String(), user.Username)

	const validFileSize = 10 * constants.MB // 10 MB
	fileReader, contentType, err := createDummyFile(validFileSize)
	assert.NoError(t, err)

	req, _ := http.NewRequest("POST", "/files/upload", fileReader)
	req.Header.Set("Content-Type", contentType)      // Set header Content-Type untuk multipart form
	req.Header.Set("Authorization", "Bearer "+token) // Sertakan token autentikasi

	w := httptest.NewRecorder() // Perekam respons HTTP
	r.ServeHTTP(w, req)         // Layani permintaan ke router

	if w.Code != http.StatusCreated { // Harapkan 201 Created jika upload sukses
		t.Logf("TestUploadFile_Success failed. Got status %d, body: %s", w.Code, w.Body.String())
	}

	assert.Equal(t, http.StatusCreated, w.Code, "Expected 201 Created for successful upload")

	var responseBody dto.Response // Asumsi Anda menggunakan struktur Response dari utils.BuildResponseSuccess
	err = json.Unmarshal(w.Body.Bytes(), &responseBody)
	assert.NoError(t, err)
	assert.True(t, responseBody.Status)
	assert.Equal(t, dto.MESSAGE_SUCCESS_CREATE_FILE, responseBody.Message)
	CleanUpTestData(user.ID)
}

func TestUploadFile_TooLarge(t *testing.T) {
	CleanUpTestData(uuid.Nil) // Bersihkan data sebelum tes

	user, err := InsertFileUser()
	assert.NoError(t, err)

	jwtSvc := config.NewJWTService()
	fileSvc := service.NewFileService(repository.NewFileRepository(config.SetUpDatabaseConnection()))
	fc := controller.NewFileController(fileSvc, jwtSvc)

	r := gin.Default()
	r.MaxMultipartMemory = 25 * constants.MB

	fileGroup := r.Group("/files")
	fileGroup.Use(middleware.Authenticate(jwtSvc))
	fileGroup.POST("/upload", fc.Create)

	token := jwtSvc.GenerateToken(user.ID.String(), user.Username)

	const invalidFileSize = 21 * constants.MB // 21 MB
	fileReader, contentType, err := createDummyFile(invalidFileSize)
	assert.NoError(t, err)

	req, _ := http.NewRequest("POST", "/files/upload", fileReader)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge { // Harapkan 413 jika terlalu besar
		t.Logf("TestUploadFile_TooLarge failed. Expected %d, but got status %d, body: %s", http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	}

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, "Expected 413 Request Entity Too Large for file too big")

	var responseBody dto.Response
	err = json.Unmarshal(w.Body.Bytes(), &responseBody)
	assert.NoError(t, err)
	assert.False(t, responseBody.Status) // Harapkan status false untuk kegagalan
	assert.Equal(t, dto.MESSAGE_FAILED_CREATE_FILE, responseBody.Message) // Asumsi pesan default untuk gagal create
	assert.Equal(t, dto.ErrFileSizeExceeded.Error(), responseBody.Errors.(string)) // Cek error spesifik

	CleanUpTestData(user.ID)
}
