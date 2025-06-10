package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing" // Diperlukan untuk TestMain

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"FP-DevOps/middleware"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert" // Diperlukan oleh helper functions
	"gorm.io/gorm"
)

// =============================================================================
// Variabel Global untuk Lingkungan Tes
// =============================================================================
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

// =============================================================================
// Setup dan Teardown Lingkungan Tes
// =============================================================================

// TestMain adalah fungsi khusus yang dijalankan sebelum dan sesudah semua tes dalam paket 'tests'.
// Ini penting untuk setup dan teardown global.
func TestMain(m *testing.M) {
	// Setup Lingkungan Tes
	SetupTestEnvironment()

	// Jalankan semua tes
	exitCode := m.Run()

	// Teardown Lingkungan Tes
	TearDownTestEnvironment()

	// Keluar dengan kode status yang benar
	os.Exit(exitCode)
}

// SetupTestEnvironment menginisialisasi semua komponen yang diperlukan untuk tes.
func SetupTestEnvironment() {
	gin.SetMode(gin.TestMode)

	// Set variabel lingkungan untuk database test secara eksplisit.
	// PASTIKAN ini adalah kredensial database yang BENAR-BENAR untuk TESTING dan BUKAN database produksi Anda.
	os.Setenv("ENV", "test")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASS", "123") // GANTI DENGAN PASSWORD ASLI USER POSTGRES ANDA
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_NAME", "fp_devops_test") // NAMA DATABASE TEST YANG BERBEDA!
	os.Setenv("DB_PORT", "5432")

	dbTest = config.SetUpDatabaseConnection()

	// Pastikan database test bersih sebelum migrasi (opsional, tapi bagus untuk konsistensi)
	if err := dbTest.Migrator().DropTable(&entity.User{}, &entity.File{}); err != nil {
		log.Printf("Warning: Failed to drop tables before migration: %v", err)
	}

	err := dbTest.AutoMigrate(&entity.User{}, &entity.File{})
	if err != nil {
		panic(fmt.Sprintf("Failed to auto migrate database for tests: %v", err))
	}

	jwtServiceTest = config.NewJWTService()
	userRepoTest = repository.NewUserRepository(dbTest)
	fileRepoTest = repository.NewFileRepository(dbTest)
	userServiceTest = service.NewUserService(userRepoTest)
	fileServiceTest = service.NewFileService(fileRepoTest)
	fileControllerTest = controller.NewFileController(fileServiceTest, jwtServiceTest)

	routerTest = gin.Default()
	routerTest.Use(middleware.CORSMiddleware())

	userRoutes := routerTest.Group("/api/user")
	{
		userRoutes.POST("/register", controller.NewUserController(userServiceTest, jwtServiceTest).Register)
		userRoutes.POST("/login", controller.NewUserController(userServiceTest, jwtServiceTest).Login)
	}

	fileRoutes := routerTest.Group("/api/file")
	{
		fileRoutes.POST("", middleware.Authenticate(jwtServiceTest), fileControllerTest.Create)
		fileRoutes.GET("/:id", fileControllerTest.GetFileByID)
		fileRoutes.PATCH("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)
		fileRoutes.DELETE("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.DeleteByID)
	}
}

// TearDownTestEnvironment membersihkan sumber daya setelah semua tes selesai.
func TearDownTestEnvironment() {
	sqlDB, err := dbTest.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
	}
	if sqlDB != nil {
		sqlDB.Close()
	}

	// Hapus semua tabel di database test untuk memastikan database bersih.
	if err := dbTest.Migrator().DropTable(&entity.File{}, &entity.User{}); err != nil {
		log.Printf("Warning: Failed to drop tables during teardown: %v", err)
	}

	// Bersihkan direktori upload
	uploadDir := filepath.Join("..", "uploads")
	if _, err := os.Stat(uploadDir); !os.IsNotExist(err) {
		if err := os.RemoveAll(uploadDir); err != nil {
			fmt.Printf("Warning: Could not clean up upload directory %s: %v\n", uploadDir, err)
		}
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fmt.Printf("Warning: Could not recreate upload directory %s: %v\n", uploadDir, err)
	}
}

// =============================================================================
// Helper Functions untuk Tes
// =============================================================================

// InsertTestUser menyisipkan pengguna tunggal untuk tujuan tes dan mengembalikannya.
// Fungsi ini mirip dengan InsertFileUser Anda, namun diganti namanya agar lebih umum.
// func InsertTestUser() (entity.User, error) {
// 	// Pastikan kita menggunakan dbTest yang diinisialisasi di SetupTestEnvironment
// 	user := entity.User{
// 		ID:       uuid.New(),
// 		Username: "testuser",
// 		Password: "password123",
// 	}
// 	// Perlu hash password di sini jika aplikasi utama Anda menghash password saat registrasi
// 	// Contoh: user.Password = service.HashPassword("password123")
// 	if err := dbTest.Create(&user).Error; err != nil {
// 		return entity.User{}, err
// 	}
// 	return user, nil
// }

// InsertTestFiles menyisipkan file-file untuk userID yang diberikan.
func InsertTestFiles(userID uuid.UUID) ([]entity.File, error) {
	files := []entity.File{
		{
			ID:       uuid.New(),
			Filename: "a.txt",
			UserID:   userID,
			Size:     100, // Tambahkan detail file lainnya yang mungkin dibutuhkan
			MimeType: "text/plain",
		},
		{
			ID:       uuid.New(),
			Filename: "KEGANTICOY.txt",
			UserID:   userID,
			Size:     200,
			MimeType: "text/plain",
		},
	}
	for i := range files { // Gunakan index untuk memodifikasi slice
		if err := dbTest.Create(&files[i]).Error; err != nil {
			return nil, err
		}
	}
	return files, nil
}

// CleanUpDatabaseData membersihkan data pengguna dan file dari database test.
// Ini berbeda dari TearDownTestEnvironment karena ini bisa dipanggil per test.
// Jika TestMain sudah membersihkan semua tabel, fungsi ini mungkin hanya diperlukan
// jika Anda ingin membersihkan data di tengah-tengah suite tes.
func CleanUpDatabaseData(userID uuid.UUID) {
	// Perlu hati-hati di sini: jika ada transaksi aktif, ini mungkin tidak berfungsi.
	// dbTest.Exec("DELETE FROM files") // Hapus semua file
	// dbTest.Exec("DELETE FROM users WHERE id = ?", userID) // Hapus user tertentu

	// Alternatif yang lebih aman jika ingin membersihkan per-test (misalnya dengan Gorm):
	// dbTest.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&entity.File{})
	// dbTest.Unscoped().Delete(&entity.User{}, "id = ?", userID) // Unscoped untuk menghapus soft-deleted
	log.Println("WARNING: CleanUpDatabaseData is not fully implemented or might be redundant if TestMain handles global teardown.")
	log.Println("Consider using database transactions per-test for better isolation if fine-grained cleanup is needed.")
	// Karena TestMain sudah DropTable, fungsi ini mungkin tidak terlalu dibutuhkan
	// atau harus diimplementasikan dengan transaction jika dipanggil per-test.
}

// createUserAndGetToken membuat pengguna baru dan mengembalikan token JWT mereka.
func createUserAndGetToken(username, password string) (entity.User, string, error) {
	// Gunakan service.NewUserService untuk mendaftarkan user
	userCreated, err := userServiceTest.RegisterUser(context.Background(), dto.UserRequest{
		Username: username,
		Password: password,
		// Email:    username + "@example.com", // Tambahkan jika ada field Email di UserRegisterRequest
	})
	if err != nil {
		return entity.User{}, "", err
	}
	// Pastikan user ID diubah ke string yang benar jika UUID
	token := jwtServiceTest.GenerateToken(userCreated.ID, "user") // asumsikan role default "user"
	return entity.User{}, token, nil
}

// uploadTestFile mengunggah file dummy ke aplikasi tes dan mengembalikan detail file yang diunggah.
func uploadTestFile(t *testing.T, token string, userID uuid.UUID, filename, content string) (entity.File, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
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

	assert.Equal(t, http.StatusCreated, w.Code)
	var response struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Status)
	assert.Equal(t, userID, response.Data.UserID)
	assert.Equal(t, filename, response.Data.Filename)
	assert.True(t, response.Data.ID != uuid.Nil)

	return response.Data, nil
}
