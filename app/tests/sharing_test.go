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
// Definisi ini memungkinkan semua fungsi tes dan helper untuk mengaksesnya.
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
// Fungsi ini dijalankan sebelum dan sesudah semua tes dalam paket ini.
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
	gin.SetMode(gin.TestMode)

	dbTest = config.SetUpDatabaseConnection()

	err := dbTest.AutoMigrate(&entity.User{}, &entity.File{})
	if err != nil {
		panic(fmt.Sprintf("Failed to auto migrate: %v", err))
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
	// Jangan hapus database production
	// Hanya close koneksi jika perlu
	sqlDB, err := dbTest.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
	}
	if sqlDB != nil {
		sqlDB.Close()
	}

	// Kalau tidak yakin, HAPUS bagian yang menghapus folder uploads!
}


// =============================================================================
// Helper Functions untuk Tes
// Fungsi-fungsi pembantu ini mempermudah penulisan tes dengan mengotomatiskan
// tugas-tugas umum seperti membuat pengguna atau mengunggah file.
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
	// Generate token dengan user ID dan role "user"
	token := jwtServiceTest.GenerateToken(createdUser.ID.String(), "user")
	return createdUser, token, nil
}

// uploadTestFile mengunggah file dummy ke aplikasi tes dan mengembalikan detail file yang diunggah.
func uploadTestFile(t *testing.T, token string, userID uuid.UUID, filename, content string) (entity.File, error) {
	// Router untuk endpoint POST /api/file.
	// Di sini kita mendefinisikan ulang router.POST karena kita tidak ingin menggunakan middleware global
	// di fungsi helper ini agar lebih fleksibel. Namun, karena ini adalah helper untuk test,
	// kita perlu memastikan `c.Set("user_id", ...)` dilakukan agar controller berjalan.
	// Alternatif yang lebih baik adalah menggunakan middleware.Authenticate di rute ini juga,
	// seperti yang sudah dilakukan di SetupTestEnvironment.
	// Saya akan tetap menggunakan pendekatan ini agar sesuai dengan kode asli Anda,
	// tapi perlu diingat bahwa ini adalah cara "manual" yang bisa diganti.
	// Jika rute POST /api/file sudah didefinisikan di SetupTestEnvironment dengan middleware Authenticate,
	// Anda bisa menghapus blok routerTest.POST di helper ini.
	// Untuk saat ini, saya asumsikan rute ini harus diinisialisasi lagi di helper ini.
	// *** CATATAN PENTING: BARIS DI BAWAH INI BISA DIHAPUS JIKA rute `fileRoutes.POST` di `SetupTestEnvironment` sudah cukup.
	// *** Jika dihapus, Anda harus memastikan `routerTest` diinisialisasi sekali saja dengan semua rutenya.
	routerTest.POST("/api/file", middleware.Authenticate(jwtServiceTest), fileControllerTest.Create)
	// *** Akhir catatan penting.

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
// Bagian ini berisi kasus uji untuk fitur berbagi file (public/private access).
// =============================================================================

// Test_FileSharing_TogglePublic_OK: Menguji skenario sukses mengubah status shareable file
func Test_FileSharing_TogglePublic_OK(t *testing.T) {
	// Pastikan lingkungan tes siap sebelum menjalankan tes ini.
	// Tidak perlu memanggil SetupTestEnvironment di setiap fungsi tes individu
	// jika sudah ada TestMain yang mengelola Setup dan TearDown.
	// Namun, jika Anda menjalankan tes individu (go test -run <nama_test>),
	// Anda mungkin ingin tetap memanggilnya atau menggunakan suite.
	// Untuk saat ini, saya akan tetap memasukkannya agar fleksibel.
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

	// Pastikan rute PATCH untuk file sudah terdaftar dengan middleware autentikasi
	// Ini sudah dilakukan di SetupTestEnvironment(), jadi baris ini bisa dihilangkan
	// jika SetupTestEnvironment() selalu dipanggil sebelum tes.
	// Namun, untuk kejelasan bahwa rute ini diuji, bisa juga dibiarkan.
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
	// Pastikan rute PATCH untuk file sudah terdaftar dengan middleware autentikasi
	// (sudah dilakukan di SetupTestEnvironment(), jadi ini bisa dihilangkan)
	// routerTest.PATCH("/api/file/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)
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
	// Rute GET untuk file sudah terdaftar di SetupTestEnvironment().
	// Controller `GetFileByID` seharusnya menangani logika `?view=true` untuk file publik.
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
	// (sudah dilakukan di SetupTestEnvironment(), jadi ini bisa dihilangkan)
	// routerTest.PATCH("/api/file/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)
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
	// Rute GET untuk file sudah terdaftar di SetupTestEnvironment().
	// Karena ini file privat dan tidak ada token, akses seharusnya ditolak.
	routerTest.GET("/api/file/:id", fileControllerTest.GetFileByID) // Gunakan handler yang sama
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
	// (sudah dilakukan di SetupTestEnvironment(), jadi ini bisa dihilangkan)
	// routerTest.PATCH("/api/file/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.UpdateByID)
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
	// Rute GET untuk file sudah terdaftar dengan middleware autentikasi di SetupTestEnvironment().
	routerTest.GET("/api/file/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.GetFileByID) // Middleware akan memeriksa token User B
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s", uploadedFile.ID), nil)             // Tanpa ?view=true, karena ada token
	assert.NoError(t, err)
	reqAccess.Header.Set("Authorization", "Bearer "+tokenB) // Menggunakan token User B
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	// 5. Verifikasi respons
	assert.Equal(t, http.StatusForbidden, wAccess.Code) // Harapan: Status 403 Forbidden
	assert.Contains(t, wAccess.Body.String(), "not authorized to perform this action") // Asumsi pesan otorisasi dari controller
}