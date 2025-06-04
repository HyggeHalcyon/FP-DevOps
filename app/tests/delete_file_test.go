package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/entity"
	"FP-DevOps/middleware"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var (
	dbTest             *gorm.DB
	jwtServiceTest     config.JWTService
	userRepoTest       repository.UserRepository
	fileRepoTest       repository.FileRepository
	userServiceTest    service.UserService
	fileServiceTest    service.FileService
	fileControllerTest controller.FileController
	routerTest         *gin.Engine
)

// SetupTestEnvironment initializes the test environment
func SetupTestEnvironment() {
	gin.SetMode(gin.TestMode) // Set Gin to test mode

	// Initialize a test database (in-memory SQLite for speed and isolation)
	dbTest = config.SetUpTestDatabaseConnection() // Assume you have this function for SQLite

	// Run migrations
	err := dbTest.AutoMigrate(&entity.User{}, &entity.File{})
	if err != nil {
		panic(fmt.Sprintf("Failed to auto migrate: %v", err))
	}

	jwtServiceTest = config.NewJWTService() // Using the same JWT service as main app
	userRepoTest = repository.NewUserRepository(dbTest)
	fileRepoTest = repository.NewFileRepository(dbTest)
	userServiceTest = service.NewUserService(userRepoTest)
	fileServiceTest = service.NewFileService(fileRepoTest)
	fileControllerTest = controller.NewFileController(fileServiceTest, jwtServiceTest)

	routerTest = gin.Default()
	routerTest.Use(middleware.CORSMiddleware()) // Apply CORS for completeness, though not strictly needed for direct tests

	// Define file routes specifically for testing
	fileRoutes := routerTest.Group("/api/file")
	{
		fileRoutes.GET("", middleware.Authenticate(jwtServiceTest), fileControllerTest.GetPaginated)
		fileRoutes.POST("", middleware.Authenticate(jwtServiceTest), fileControllerTest.Create)
		fileRoutes.GET("/:id", fileControllerTest.GetFileByID) // Public/private handled in controller
		fileRoutes.PATCH("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)
		fileRoutes.DELETE("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.DeleteByID)
	}
}

// TearDownTestEnvironment cleans up the test database
func TearDownTestEnvironment() {
	sqlDB, err := dbTest.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
	}
	if sqlDB != nil {
		sqlDB.Close()
	}
	// For SQLite, simply closing the connection usually cleans up
	// For other DBs, you might need to drop tables or truncate.
}

// Helper to create a test user and get their token
func createUserAndGetToken(username, password string) (entity.User, string, error) {
	user := entity.User{
		Username: username,
		// Email:    username + "@example.com",
		Password: password,
	}
	createdUser, err := userRepoTest.Create(user)
	if err != nil {
		return entity.User{}, "", err
	}
	token := jwtServiceTest.GenerateToken(createdUser.ID.String(), "user")
	return createdUser, token, nil
}

// Helper to upload a file (for setup)
func uploadTestFile(t *testing.T, token string, userID uuid.UUID, filename, content string) (entity.File, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Create a form file field
	part, err := writer.CreateFormFile("file", filename)
	assert.NoError(t, err)

	_, err = io.Copy(part, bytes.NewBufferString(content))
	assert.NoError(t, err)

	writer.Close()

	req, _ := http.NewRequest("POST", "/api/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	routerTest.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code) // Assuming 201 Created for successful upload

	var response struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Status)
	assert.Equal(t, userID, response.Data.UserID)
	assert.Equal(t, filename, response.Data.Filename)
	assert.True(t, response.Data.ID != uuid.Nil) // Ensure file ID is generated

	// Manually set path for testing purposes, as actual storage isn't mocked here
	// In a real app, this would be handled by the file service
	// For testing, we might need to simulate this if the file service depends on it directly.
	// For simplicity in this test, we assume the file record in DB is enough.

	return response.Data, nil
}

// Test_FileDelete_OK: Sukses menghapus file milik sendiri
func Test_FileDelete_OK(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat user dan dapatkan token
	user, token, err := createUserAndGetToken("owner_delete", "password123")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	// 2. Upload file oleh user tersebut
	uploadedFile, err := uploadTestFile(t, token, user.ID, "my_document.txt", "This is my secret document content.")
	assert.NoError(t, err)
	assert.NotNil(t, uploadedFile)
	assert.True(t, uploadedFile.ID != uuid.Nil)

	// Verify file exists in DB before deletion
	var fileBeforeDelete entity.File
	res := dbTest.First(&fileBeforeDelete, "id = ?", uploadedFile.ID)
	assert.Nil(t, res.Error) // Should not be an error, file should exist

	// 3. Lakukan permintaan DELETE ke endpoint /api/file/:id
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/file/%s", uploadedFile.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	routerTest.ServeHTTP(recorder, req)

	// 4. Verifikasi respons
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Status)
	assert.Equal(t, "File deleted successfully", response.Message)

	// 5. Verifikasi bahwa file sudah tidak ada di database
	var deletedFile entity.File
	res = dbTest.First(&deletedFile, "id = ?", uploadedFile.ID)
	assert.NotNil(t, res.Error) // Should return an error (record not found)
	assert.Equal(t, gorm.ErrRecordNotFound, res.Error)

	// In a real scenario, you would also verify the file is removed from storage
	// (e.g., check if os.Stat(path) returns an error)
}

// Test_FileDelete_NotOwner: Mencoba menghapus file milik pengguna lain
func Test_FileDelete_NotOwner(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat user A (pemilik file)
	userA, tokenA, err := createUserAndGetToken("user_a", "passA")
	assert.NoError(t, err)

	// 2. Upload file oleh user A
	fileOwnedByA, err := uploadTestFile(t, tokenA, userA.ID, "user_a_file.jpg", "Content of A's file.")
	assert.NoError(t, err)

	// 3. Buat user B (bukan pemilik)
	_, tokenB, err := createUserAndGetToken("user_b", "passB")
	assert.NoError(t, err)

	// 4. User B mencoba menghapus file milik user A
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/file/%s", fileOwnedByA.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tokenB) // Use token of user B
	recorder := httptest.NewRecorder()
	routerTest.ServeHTTP(recorder, req)

	// 5. Verifikasi respons: Unauthorized (atau Forbidden jika ada logic spesifik untuk itu)
	assert.Equal(t, http.StatusForbidden, recorder.Code) // Assuming 403 Forbidden for not being the owner
	var response struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Status)
	assert.Contains(t, response.Message, "not authorized to perform this action") // Or similar specific message

	// 6. Verifikasi bahwa file masih ada di database
	var fileStillExists entity.File
	res := dbTest.First(&fileStillExists, "id = ?", fileOwnedByA.ID)
	assert.Nil(t, res.Error) // Should not be an error, file should still exist
	assert.Equal(t, fileOwnedByA.ID, fileStillExists.ID)
}

// Test_FileDelete_NotFound: Mencoba menghapus file yang tidak ada (ID file salah/tidak ditemukan)
func Test_FileDelete_NotFound(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat user dan dapatkan token
	_, token, err := createUserAndGetToken("user_notfound", "password123")
	assert.NoError(t, err)

	// 2. Gunakan ID file yang tidak ada
	nonExistentFileID := uuid.New() // Generate a random, non-existent UUID

	// 3. Lakukan permintaan DELETE ke endpoint /api/file/:id
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/file/%s", nonExistentFileID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	routerTest.ServeHTTP(recorder, req)

	// 4. Verifikasi respons: Not Found
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	var response struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Status)
	assert.Contains(t, response.Message, "File not found") // Or similar specific message
}

// Test_FileDelete_NoToken: Hapus file tanpa token/tidak login
func Test_FileDelete_NoToken(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat user dan upload file (untuk memastikan ada file yang bisa dicoba dihapus)
	user, token, err := createUserAndGetToken("user_notoken", "password123")
	assert.NoError(t, err)
	uploadedFile, err := uploadTestFile(t, token, user.ID, "no_token_test.txt", "Some content.")
	assert.NoError(t, err)

	// 2. Lakukan permintaan DELETE tanpa token (Authorization header)
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/file/%s", uploadedFile.ID), nil)
	recorder := httptest.NewRecorder()
	routerTest.ServeHTTP(recorder, req)

	// 3. Verifikasi respons: Unauthorized
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Status)
	assert.Contains(t, response.Message, "Unauthorized") // Or similar specific message from your JWT middleware

	// 4. Verifikasi bahwa file masih ada di database
	var fileStillExists entity.File
	res := dbTest.First(&fileStillExists, "id = ?", uploadedFile.ID)
	assert.Nil(t, res.Error) // Should not be an error, file should still exist
	assert.Equal(t, uploadedFile.ID, fileStillExists.ID)
}
