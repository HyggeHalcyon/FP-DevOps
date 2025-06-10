package tests

import (
	"bytes"
	"context" // Import context
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/dto" // Import dto
	"FP-DevOps/entity"
	"FP-DevOps/middleware"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
	os.Setenv("ENV", "test")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASS", "123") // <<< PENTING: GANTI DENGAN PASSWORD ASLI USER POSTGRES ANDA
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_NAME", "fp_devops_test") // NAMA DATABASE TEST YANG BERBEDA!
	os.Setenv("DB_PORT", "5432")

	// Inisialisasi koneksi database
	log.Println("DEBUG: Menginisialisasi koneksi database test...")
	dbTest = config.SetUpDatabaseConnection()
	if dbTest == nil {
		panic("Failed to connect to test database: dbTest is nil after SetUpDatabaseConnection()")
	}
	log.Println("DEBUG: dbTest berhasil diinisialisasi.")

	// Pastikan database test bersih sebelum migrasi
	log.Println("DEBUG: Membersihkan tabel database test (jika ada)...")
	if err := dbTest.Migrator().DropTable(&entity.User{}, &entity.File{}); err != nil {
		log.Printf("WARNING: Gagal menghapus tabel sebelum migrasi: %v", err)
	}
	log.Println("DEBUG: Tabel dibersihkan.")

	// Auto-migrate skema database
	log.Println("DEBUG: Melakukan AutoMigrate...")
	err := dbTest.AutoMigrate(&entity.User{}, &entity.File{})
	if err != nil {
		panic(fmt.Sprintf("Gagal auto-migrate database untuk tes: %v", err))
	}
	log.Println("DEBUG: AutoMigrate selesai.")

	// Inisialisasi service dan repository, dengan pengecekan nil
	log.Println("DEBUG: Menginisialisasi layanan dan repository...")
	jwtServiceTest = config.NewJWTService()
	if jwtServiceTest == nil {
		panic("jwtServiceTest adalah nil setelah NewJWTService()")
	}
	log.Println("DEBUG: jwtServiceTest diinisialisasi.")

	userRepoTest = repository.NewUserRepository(dbTest)
	if userRepoTest == nil {
		panic("userRepoTest adalah nil setelah NewUserRepository()")
	}
	log.Println("DEBUG: userRepoTest diinisialisasi.")

	fileRepoTest = repository.NewFileRepository(dbTest)
	if fileRepoTest == nil {
		panic("fileRepoTest adalah nil setelah NewFileRepository()")
	}
	log.Println("DEBUG: fileRepoTest diinisialisasi.")

	userServiceTest = service.NewUserService(userRepoTest)
	if userServiceTest == nil {
		panic("userServiceTest adalah nil setelah NewUserService()")
	}
	log.Println("DEBUG: userServiceTest diinisialisasi.")

	fileServiceTest = service.NewFileService(fileRepoTest)
	if fileServiceTest == nil {
		panic("fileServiceTest adalah nil setelah NewFileService()")
	}
	log.Println("DEBUG: fileServiceTest diinisialisasi.")

	fileControllerTest = controller.NewFileController(fileServiceTest, jwtServiceTest)
	if fileControllerTest == nil {
		panic("fileControllerTest adalah nil setelah NewFileController()")
	}
	log.Println("DEBUG: fileControllerTest diinisialisasi.")

	routerTest = gin.Default()
	routerTest.Use(middleware.CORSMiddleware())
	log.Println("DEBUG: routerTest diinisialisasi.")

	// Definisikan semua rute API yang mungkin diuji di sini.
	userRoutes := routerTest.Group("/api/user")
	{
		// Dapatkan instance UserController yang sudah diinisialisasi
		userController := controller.NewUserController(userServiceTest, jwtServiceTest)
		userRoutes.POST("/register", userController.Register)
		userRoutes.POST("/login", userController.Login)
	}

	fileRoutes := routerTest.Group("/api/file")
	{
		fileRoutes.POST("", middleware.Authenticate(jwtServiceTest), fileControllerTest.Create)
		fileRoutes.GET("/:id", fileControllerTest.GetFileByID)
		fileRoutes.PATCH("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)
		fileRoutes.DELETE("/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.DeleteByID)
	}
	log.Println("DEBUG: Semua rute terdaftar.")
}

// TearDownTestEnvironment membersihkan sumber daya setelah semua tes selesai.
func TearDownTestEnvironment() {
	log.Println("DEBUG: Memulai TearDownTestEnvironment...")
	sqlDB, err := dbTest.DB()
	if err != nil {
		log.Printf("Gagal mendapatkan SQL DB: %v", err)
	}
	if sqlDB != nil {
		sqlDB.Close()
		log.Println("DEBUG: Koneksi database ditutup.")
	}

	// Hapus semua tabel di database test untuk memastikan database bersih.
	if err := dbTest.Migrator().DropTable(&entity.File{}, &entity.User{}); err != nil {
		log.Printf("WARNING: Gagal menghapus tabel selama teardown: %v", err)
	}
	log.Println("DEBUG: Tabel database test dihapus.")

	// Bersihkan direktori upload
	uploadDir := filepath.Join("..", "uploads")
	if _, err := os.Stat(uploadDir); !os.IsNotExist(err) {
		if err := os.RemoveAll(uploadDir); err != nil {
			fmt.Printf("WARNING: Tidak dapat membersihkan direktori upload %s: %v\n", uploadDir, err)
		} else {
			log.Printf("DEBUG: Direktori upload %s dibersihkan.", uploadDir)
		}
	}
	// Buat ulang direktori uploads agar tidak mengganggu test berikutnya atau aplikasi utama
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fmt.Printf("WARNING: Tidak dapat membuat ulang direktori upload %s: %v\n", uploadDir, err)
	} else {
		log.Printf("DEBUG: Direktori upload %s dibuat ulang.", uploadDir)
	}
	log.Println("DEBUG: TearDownTestEnvironment selesai.")
}

// =============================================================================
// Helper Functions untuk Tes
// =============================================================================

// InsertTestFiles menyisipkan file-file untuk userID yang diberikan.
func InsertTestFiles(userID uuid.UUID) ([]entity.File, error) {
	files := []entity.File{
		{
			ID:       uuid.New(),
			Filename: "a.txt",
			UserID:   userID,
			Size:     100,
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
	for i := range files {
		if err := dbTest.Create(&files[i]).Error; err != nil {
			return nil, err
		}
	}
	return files, nil
}

// CleanUpDatabaseData membersihkan data pengguna dan file dari database test.
// Fungsi ini mungkin tidak selalu diperlukan jika TearDownTestEnvironment sudah membersihkan semua tabel.
func CleanUpDatabaseData(userID uuid.UUID) {
	log.Println("WARNING: CleanUpDatabaseData mungkin redundan jika TestMain sudah menangani global teardown (menghapus tabel).")
	log.Println("Pertimbangkan untuk menggunakan transaksi database per-tes untuk isolasi yang lebih baik jika pembersihan data yang lebih halus dibutuhkan.")
}

// createUserAndGetToken membuat pengguna baru dan mengembalikan token JWT mereka.
// Memperbaiki argumen RegisterUser, type assertion ID, dan tipe return.
func createUserAndGetToken(username, password string) (entity.User, string, error) {
	userReq := dto.UserRequest{ // Menggunakan dto.UserRegisterRequest
		Username: username,
		Password: password,
		// Email:    username + "@example.com",
	}
	log.Printf("DEBUG: Memanggil userServiceTest.RegisterUser untuk %s", username)

	// Memanggil RegisterUser dengan context dan DTO yang benar
	userCreatedDTO, err := userServiceTest.RegisterUser(context.Background(), userReq)
	if err != nil {
		log.Printf("ERROR: userServiceTest.RegisterUser gagal untuk %s: %v", username, err)
		return entity.User{}, "", err
	}
	log.Printf("DEBUG: Pengguna %s berhasil diregistrasi. DTO: %+v", username, userCreatedDTO)

	// Konversi dto.UserResponse ke entity.User
	// Asumsi userCreatedDTO.ID adalah string yang valid UUID
	userEntity := entity.User{
		ID:       uuid.MustParse(userCreatedDTO.ID), // Konversi string ID dari DTO ke uuid.UUID
		Username: userCreatedDTO.Username,
		// Password: userCreatedDTO.Password, // Hati-hati jika ini tidak ada di DTO atau tidak di-hash
	}
	// Jika password perlu diisi untuk entitas ini dan tidak ada di DTO,
	// Anda mungkin perlu mengambilnya dari DB atau menghiraukannya.
	// userEntity.Password = password // Jika Anda ingin menyimpan password plain untuk tes ini

	// Generate token
	token := jwtServiceTest.GenerateToken(userCreatedDTO.ID, "user")
	log.Printf("DEBUG: Token dibuat untuk %s", username)

	return userEntity, token, nil // Mengembalikan entity.User yang valid
}

// uploadTestFile mengunggah file dummy ke aplikasi tes dan mengembalikan detail file yang diunggah.
func uploadTestFile(t *testing.T, token string, userID uuid.UUID, filename, content string) (entity.File, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body) // Perbaikan: multipart.NewNewWriter() menjadi multipart.NewWriter()
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