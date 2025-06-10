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
	"os"
	"path/filepath" // Import path/filepath
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

// TestMain adalah fungsi khusus yang dijalankan sebelum dan sesudah semua tes dalam paket.
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
	// Atur Gin ke mode tes untuk menonaktifkan log normal
	gin.SetMode(gin.TestMode)

	// --- PERBAIKAN PENTING UNTUK MENGATASI "insufficient arguments" ---
	// Set variabel lingkungan untuk database test secara eksplisit.
	// Ini memastikan konfigurasi database yang benar saat menjalankan tes,
	// terlepas dari keberadaan file .env atau pengaturan environment di luar.
	os.Setenv("ENV", "test") // Set ENV ke "test" agar godotenv tidak mencari .env di jalur yang salah
	os.Setenv("DB_USER", "postgres") // Ganti dengan username database test Anda
	os.Setenv("DB_PASS", "123")      // Ganti dengan password database test Anda
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_NAME", "fp_devops_test") // << PENTING: Gunakan NAMA DATABASE YANG BERBEDA UNTUK TEST!
	os.Setenv("DB_PORT", "5432")

	// Setup koneksi database
	dbTest = config.SetUpDatabaseConnection()

	// Migrasi otomatis skema database untuk entitas yang relevan
	err := dbTest.AutoMigrate(&entity.User{}, &entity.File{})
	if err != nil {
		panic(fmt.Sprintf("Failed to auto migrate database for tests: %v", err))
	}

	// Inisialisasi service dan repository
	jwtServiceTest = config.NewJWTService()
	userRepoTest = repository.NewUserRepository(dbTest)
	fileRepoTest = repository.NewFileRepository(dbTest)
	userServiceTest = service.NewUserService(userRepoTest)
	fileServiceTest = service.NewFileService(fileRepoTest)
	fileControllerTest = controller.NewFileController(fileServiceTest, jwtServiceTest)

	// Inisialisasi router Gin dan terapkan middleware umum
	routerTest = gin.Default()
	routerTest.Use(middleware.CORSMiddleware())

	// Definisikan semua rute API yang mungkin diuji di sini, SEKALI SAJA.
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
	// Tutup koneksi database
	sqlDB, err := dbTest.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
	}
	if sqlDB != nil {
		sqlDB.Close()
	}

	// Hapus semua tabel di database test untuk memastikan database bersih untuk eksekusi berikutnya.
	// PENTING: Ini hanya aman dilakukan pada database TEST!
	if err := dbTest.Migrator().DropTable(&entity.File{}, &entity.User{}); err != nil {
		log.Printf("Warning: Failed to drop tables during teardown: %v", err)
	}

	// Bersihkan direktori upload
	// Asumsi direktori 'uploads' berada satu level di atas direktori 'tests'
	uploadDir := filepath.Join("..", "uploads")
	if _, err := os.Stat(uploadDir); !os.IsNotExist(err) {
		if err := os.RemoveAll(uploadDir); err != nil {
			fmt.Printf("Warning: Could not clean up upload directory %s: %v\n", uploadDir, err)
		}
	}
	// Buat ulang direktori uploads agar tidak mengganggu test berikutnya atau aplikasi utama
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fmt.Printf("Warning: Could not recreate upload directory %s: %v\n", uploadDir, err)
	}
}

// =============================================================================
// Helper Functions untuk Tes
// =============================================================================

// createUserAndGetToken membuat pengguna baru di database tes dan mengembalikan token JWT mereka.
func createUserAndGetToken(username, password string) (entity.User, string, error) {
	user := entity.User{
		Username: username,
		// Email:    username + "@example.com", // Hapus baris ini jika entity.User Anda tidak punya field Email
		Password: password,
	}
	createdUser, err := userRepoTest.Create(user)
	if err != nil {
		return entity.User{}, "", err
	}
	token := jwtServiceTest.GenerateToken(createdUser.ID.String(), "user")
	return createdUser, token, nil
}

// uploadTestFile mengunggah file dummy ke aplikasi tes dan mengembalikan detail file yang diunggah.
func uploadTestFile(t *testing.T, token string, userID uuid.UUID, filename, content string) (entity.File, error) {
	// Karena rute POST /api/file sudah didefinisikan secara global di SetupTestEnvironment(),
	// baris di bawah ini TIDAK DIPERLUKAN lagi:
	// routerTest.POST("/api/file", middleware.Authenticate(jwtServiceTest), fileControllerTest.Create)

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
	req.Header.Set("Authorization", "Bearer "+token) // Token diperlukan untuk autentikasi upload
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

// =============================================================================
// Tes Fungsional untuk Fitur Berbagi File
// =============================================================================

// Test_FileSharing_TogglePublic_OK: Menguji skenario sukses mengubah status shareable file
func Test_FileSharing_TogglePublic_OK(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat pengguna dan dapatkan token mereka
	user, token, err := createUserAndGetToken("sharer", "sharer123")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	// 2. Unggah file sebagai pengguna tersebut
	uploadedFile, err := uploadTestFile(t, token, user.ID, "private_doc.txt", "Dokumen rahasia.")
	assert.NoError(t, err)
	assert.NotNil(t, uploadedFile)

	// Rute PATCH untuk file sudah terdaftar di SetupTestEnvironment(), jadi tidak perlu didaftarkan ulang.
	// Baris berikut DIHAPUS:
	// routerTest.PATCH("/api/file/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)

	// 3. Ubah status file menjadi PUBLIC
	payloadPublic := struct {
		Shareable bool `json:"shareable"`
	}{Shareable: true}
	bodyPublic, _ := json.Marshal(payloadPublic)
	reqPublic, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPublic))
	assert.NoError(t, err)
	reqPublic.Header.Set("Content-Type", "application/json")
	reqPublic.Header.Set("Authorization", "Bearer "+token) // Token diperlukan untuk otorisasi
	wPublic := httptest.NewRecorder()
	routerTest.ServeHTTP(wPublic, reqPublic)

	// Verifikasi respons setelah mengubah ke PUBLIC
	assert.Equal(t, http.StatusOK, wPublic.Code)
	var respPublic struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(wPublic.Body.Bytes(), &respPublic)
	assert.NoError(t, err)
	assert.True(t, respPublic.Status)
	assert.True(t, *respPublic.Data.Shareable) // Pastikan status shareable menjadi true
	assert.Equal(t, uploadedFile.ID, respPublic.Data.ID)

	// 4. Ubah status file kembali menjadi PRIVATE
	payloadPrivate := struct {
		Shareable bool `json:"shareable"`
	}{Shareable: false}
	bodyPrivate, _ := json.Marshal(payloadPrivate)
	reqPrivate, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPrivate))
	assert.NoError(t, err)
	reqPrivate.Header.Set("Content-Type", "application/json")
	reqPrivate.Header.Set("Authorization", "Bearer "+token) // Token diperlukan untuk otorisasi
	wPrivate := httptest.NewRecorder()
	routerTest.ServeHTTP(wPrivate, reqPrivate)

	// Verifikasi respons setelah mengubah kembali ke PRIVATE
	assert.Equal(t, http.StatusOK, wPrivate.Code)
	var respPrivate struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(wPrivate.Body.Bytes(), &respPrivate)
	assert.NoError(t, err)
	assert.True(t, respPrivate.Status)
	assert.False(t, *respPrivate.Data.Shareable) // Pastikan status shareable menjadi false
	assert.Equal(t, uploadedFile.ID, respPrivate.Data.ID)
}

// Test_FileSharing_AccessPublicFile_OK: Menguji akses file publik tanpa autentikasi
func Test_FileSharing_AccessPublicFile_OK(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat pengguna dan upload file
	user, token, err := createUserAndGetToken("public_owner", "public_owner123")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	uploadedFile, err := uploadTestFile(t, token, user.ID, "public_access.txt", "Ini file publik.")
	assert.NoError(t, err)
	assert.NotNil(t, uploadedFile)

	// 2. Ubah file menjadi PUBLIC
	// Rute PATCH sudah terdaftar di SetupTestEnvironment().
	payloadPublic := struct {
		Shareable bool `json:"shareable"`
	}{Shareable: true}
	bodyPublic, _ := json.Marshal(payloadPublic)
	reqPublic, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPublic))
	assert.NoError(t, err)
	reqPublic.Header.Set("Content-Type", "application/json")
	reqPublic.Header.Set("Authorization", "Bearer "+token)
	wPublic := httptest.NewRecorder()
	routerTest.ServeHTTP(wPublic, reqPublic)
	assert.Equal(t, http.StatusOK, wPublic.Code) // Verifikasi perubahan status sukses

	// 3. Coba akses file publik tanpa autentikasi (menggunakan /api/file/:id?view=true)
	// Rute GET sudah terdaftar di SetupTestEnvironment().
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s?view=true", uploadedFile.ID), nil) // Tanpa header Auth
	assert.NoError(t, err)
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	// 4. Verifikasi respons
	assert.Equal(t, http.StatusOK, wAccess.Code)
	assert.Contains(t, wAccess.Body.String(), "Ini file publik.") // Asumsi konten file dikembalikan dalam respons
}

// Test_FileSharing_AccessPrivateFile_Unauthorized: Menguji akses file private tanpa autentikasi (anonim)
func Test_FileSharing_AccessPrivateFile_Unauthorized(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat pengguna dan unggah file, pastikan statusnya privat (default upload biasanya privat)
	user, token, err := createUserAndGetToken("private_owner", "private_owner123")
	assert.NoError(t, err)
	uploadedFile, err := uploadTestFile(t, token, user.ID, "private_doc_unauth.txt", "Ini dokumen privat.")
	assert.NoError(t, err)

	// 2. Verifikasi file ini privat (shareable = false) secara eksplisit jika diperlukan
	// Rute PATCH sudah terdaftar di SetupTestEnvironment().
	payloadPrivateExplicit := struct {
		Shareable bool `json:"shareable"`
	}{Shareable: false}
	bodyPrivateExplicit, _ := json.Marshal(payloadPrivateExplicit)
	reqPrivateExplicit, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPrivateExplicit))
	assert.NoError(t, err)
	reqPrivateExplicit.Header.Set("Content-Type", "application/json")
	reqPrivateExplicit.Header.Set("Authorization", "Bearer "+token)
	wPrivateExplicit := httptest.NewRecorder()
	routerTest.ServeHTTP(wPrivateExplicit, reqPrivateExplicit)
	assert.Equal(t, http.StatusOK, wPrivateExplicit.Code)
	var respPrivateExplicit struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(wPrivateExplicit.Body.Bytes(), &respPrivateExplicit)
	assert.NoError(t, err)
	assert.False(t, *respPrivateExplicit.Data.Shareable) // Pastikan sudah privat

	// 3. Coba akses file private tanpa autentikasi (anonim)
	// Rute GET sudah terdaftar di SetupTestEnvironment().
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s?view=true", uploadedFile.ID), nil) // Tanpa header Auth
	assert.NoError(t, err)
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	// 4. Verifikasi respons
	assert.Equal(t, http.StatusUnauthorized, wAccess.Code) // Harapan: Status 401 Unauthorized
	assert.Contains(t, wAccess.Body.String(), "Unauthorized") // Asumsi pesan Unauthorized
}

// Test_FileSharing_AccessPrivateFile_NotOwner: Menguji akses file private oleh pengguna lain (bukan pemilik)
func Test_FileSharing_AccessPrivateFile_NotOwner(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat User A (pemilik file)
	userA, tokenA, err := createUserAndGetToken("ownerA_private", "passA123")
	assert.NoError(t, err)
	uploadedFile, err := uploadTestFile(t, tokenA, userA.ID, "ownerA_private_file.txt", "File pribadi A.")
	assert.NoError(t, err)

	// 2. Pastikan file ini privat
	// Rute PATCH sudah terdaftar di SetupTestEnvironment().
	payloadPrivateExplicit := struct {
		Shareable bool `json:"shareable"`
	}{Shareable: false}
	bodyPrivateExplicit, _ := json.Marshal(payloadPrivateExplicit)
	reqPrivateExplicit, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPrivateExplicit))
	assert.NoError(t, err)
	reqPrivateExplicit.Header.Set("Content-Type", "application/json")
	reqPrivateExplicit.Header.Set("Authorization", "Bearer "+tokenA)
	wPrivateExplicit := httptest.NewRecorder()
	routerTest.ServeHTTP(wPrivateExplicit, reqPrivateExplicit)
	assert.Equal(t, http.StatusOK, wPrivateExplicit.Code)
	var respPrivateExplicit struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(wPrivateExplicit.Body.Bytes(), &respPrivateExplicit)
	assert.NoError(t, err)
	assert.False(t, *respPrivateExplicit.Data.Shareable) // Pastikan sudah privat

	// 3. Buat User B (bukan pemilik) dan dapatkan tokennya
	_, tokenB, err := createUserAndGetToken("userB_private", "passB123")
	assert.NoError(t, err)

	// 4. User B mencoba mengakses file milik User A (yang privat)
	// Rute GET sudah terdaftar di SetupTestEnvironment().
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s", uploadedFile.ID), nil) // Tanpa ?view=true, karena ada token
	assert.NoError(t, err)
	reqAccess.Header.Set("Authorization", "Bearer "+tokenB) // Menggunakan token User B
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	// 5. Verifikasi respons
	assert.Equal(t, http.StatusForbidden, wAccess.Code) // Harapan: Status 403 Forbidden
	assert.Contains(t, wAccess.Body.String(), "not authorized to perform this action") // Asumsi pesan otorisasi dari controller
}