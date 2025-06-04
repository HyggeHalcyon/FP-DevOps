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
	"os"            // Untuk membuat file dummy
	"path/filepath" // Untuk path file dummy
	"testing"

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"FP-DevOps/middleware" // Digunakan di SetupTestEnvironment untuk routerTest.Use
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4" // Pastikan ini v4, sesuai dengan yang dipakai di config
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/driver/sqlite" // Untuk SetUpTestDatabaseConnection
	"gorm.io/gorm/logger"   // Untuk SetUpTestDatabaseConnection
)

// --- Variabel Global untuk Lingkungan Tes ---
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

// --- Setup dan TearDown Lingkungan Tes ---

// SetupTestEnvironment menginisialisasi seluruh lingkungan tes
func SetupTestEnvironment() {
	gin.SetMode(gin.TestMode) // Set Gin ke mode tes

	// 1. Inisialisasi database tes (SQLite in-memory)
	dbTest = config.SetUpTestDatabaseConnection()

	// 2. AutoMigrate model-model yang dibutuhkan
	err := dbTest.AutoMigrate(&entity.User{}, &entity.File{})
	if err != nil {
		panic(fmt.Sprintf("Failed to auto migrate: %v", err))
	}

	// 3. Inisialisasi service dan controller
	jwtServiceTest = config.NewJWTService()
	userRepoTest = repository.NewUserRepository(dbTest)
	fileRepoTest = repository.NewFileRepository(dbTest)
	userServiceTest = service.NewUserService(userRepoTest)
	fileServiceTest = service.NewFileService(fileRepoTest)
	fileControllerTest = controller.NewFileController(fileServiceTest, jwtServiceTest) // Pastikan NewFileController menerima jwtService

	// 4. Inisialisasi router Gin untuk tes
	routerTest = gin.Default()
	routerTest.Use(middleware.CORSMiddleware()) // Gunakan CORS middleware jika diperlukan oleh aplikasi Anda

	// 5. Definisikan rute-rute API yang akan diuji
	userRoutes := routerTest.Group("/api/user")
	{
		userRoutes.POST("/register", controller.NewUserController(userServiceTest, jwtServiceTest).Register)
		userRoutes.POST("/login", controller.NewUserController(userServiceTest, jwtServiceTest).Login)
	}

	fileRoutes := routerTest.Group("/api/file")
	{
		// Sesuaikan nama method di sini agar sama dengan yang di app/controller/file.go
		fileRoutes.POST("", middleware.Authenticate(jwtServiceTest), fileControllerTest.Create)
		fileRoutes.GET("/:id", fileControllerTest.GetFileByID) // Gunakan AuthenticateIfExists jika bisa diakses publik
		fileRoutes.PATCH("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)
		fileRoutes.DELETE("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.DeleteByID)
	}
}

// TearDownTestEnvironment membersihkan database tes
func TearDownTestEnvironment() {
	sqlDB, err := dbTest.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
	}
	if sqlDB != nil {
		sqlDB.Close() // Menutup koneksi database (untuk SQLite in-memory akan membersihkan data)
	}
	// Hapus file fisik jika disimpan di lokal (sesuaikan path storage Anda)
	uploadDir := filepath.Join("..", "uploads") // Asumsi folder 'uploads' di luar 'app'
	if _, err := os.Stat(uploadDir); !os.IsNotExist(err) {
		if err := os.RemoveAll(uploadDir); err != nil {
			fmt.Printf("Warning: Could not clean up upload directory %s: %v\n", uploadDir, err)
		}
	}
	// Buat ulang folder jika diperlukan oleh aplikasi
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fmt.Printf("Warning: Could not recreate upload directory %s: %v\n", uploadDir, err)
	}
}

// --- Helper Functions untuk Tes ---

// createUserAndGetToken membuat user tes dan mengembalikan token JWT-nya
func createUserAndGetToken(username, password string) (entity.User, string, error) {
	user := entity.User{
		Username: username,
		// Email:    username + "@example.com", // Hapus baris ini jika entity.User Anda tidak punya field Email
		Password: password,
	}
	createdUser, err := userRepoTest.Create(user) // Menggunakan method Create dari UserRepository
	if err != nil {
		return entity.User{}, "", err
	}
	// Pastikan GenerateToken menerima argumen yang benar (misal: ID user sebagai string, dan role sebagai string)
	token := jwtServiceTest.GenerateToken(createdUser.ID.String(), "user") // Asumsi role default 'user'
	return createdUser, token, nil
}

// uploadTestFile membantu mengunggah file dan mengembalikan objek File yang diunggah
func uploadTestFile(t *testing.T, token string, userID uuid.UUID, filename, content string) (entity.File, error) {
	// Router untuk endpoint upload file
	routerTest.POST("/api/file", func(c *gin.Context) {
		// Simulasikan user_id dari token
		tokenClaims, err := jwtServiceTest.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "error": "Invalid token"})
			return
		}
		claims, ok := tokenClaims.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "error": "Invalid token claims type"})
			return
		}
		c.Set("user_id", fmt.Sprintf("%v", claims["user_id"]))
		fileControllerTest.Create(c) // Memanggil method Create di controller
	})

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename) // "file" adalah nama field form untuk file
	assert.NoError(t, err)
	_, err = io.Copy(part, bytes.NewBufferString(content))
	assert.NoError(t, err)
	writer.Close()

	req, err := http.NewRequest("POST", "/api/file", body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	routerTest.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code) // Asumsi status 201 Created untuk upload sukses
	var response struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Status)
	assert.Equal(t, userID, response.Data.UserID)
	assert.Equal(t, filename, response.Data.Filename)
	assert.True(t, response.Data.ID != uuid.Nil) // Pastikan ID file tergenerate

	return response.Data, nil
}

// --- Akhir Helper Functions ---

// --- 3.8 Tests - File Upload ---

// Test_FileUpload_OK: Menguji skenario sukses mengunggah file
func Test_FileUpload_OK(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	user, token, err := createUserAndGetToken("uploader", "uploader123")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	// Router untuk endpoint upload file
	routerTest.POST("/api/file", func(c *gin.Context) {
		// Simulasikan user_id dari token
		tokenClaims, _ := jwtServiceTest.ValidateToken(token)
		claims, ok := tokenClaims.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "error": "Invalid token claims type"})
			return
		}
		c.Set("user_id", fmt.Sprintf("%v", claims["user_id"]))
		fileControllerTest.Create(c)
	})

	// Buat permintaan upload file
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "my_document.txt")
	assert.NoError(t, err)
	_, err = part.Write([]byte("Ini adalah isi dokumen penting saya."))
	assert.NoError(t, err)
	writer.Close()

	req, err := http.NewRequest("POST", "/api/file", body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	routerTest.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code) // Harapan: Status 201 Created
	var out struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &out)
	assert.NoError(t, err)
	assert.True(t, out.Status)
	assert.Equal(t, "my_document.txt", out.Data.Filename)
	assert.Equal(t, int64(30), out.Data.Size) // Ukuran "Ini adalah isi dokumen penting saya."
}

// Test_FileUpload_Unauthorized: Menguji skenario gagal upload karena tidak ada token
func Test_FileUpload_Unauthorized(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// Router untuk endpoint upload file tanpa autentikasi yang valid
	routerTest.POST("/api/file", fileControllerTest.Create) // Langsung ke controller tanpa middleware JWT

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "unauth_file.txt")
	assert.NoError(t, err)
	_, err = part.Write([]byte("Isi file."))
	assert.NoError(t, err)
	writer.Close()

	req, err := http.NewRequest("POST", "/api/file", body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// req.Header.Set("Authorization", "Bearer invalid_token") // Atau tanpa header Authorization sama sekali
	w := httptest.NewRecorder()
	routerTest.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code) // Harapan: Status 401 Unauthorized
}