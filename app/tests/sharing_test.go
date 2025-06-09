package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"FP-DevOps/entity"
	"FP-DevOps/middleware" // Diperlukan untuk middleware.Authenticate
	"FP-DevOps/config" // Diperlukan untuk config.NewJWTService().ValidateToken

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

// --- 3.14 Tests - File Sharing (Toggle Public/Private dan Akses) ---

// Test_FileSharing_TogglePublic_OK: Menguji skenario sukses mengubah status shareable file
func Test_FileSharing_TogglePublic_OK(t *testing.T) {
	SetupTestEnvironment() // Panggil Setup dari test_helpers.go
	defer TearDownTestEnvironment() // Panggil TearDown dari test_helpers.go

	user, token, err := createUserAndGetToken("sharer", "sharer123") // Helper dari test_helpers.go
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	uploadedFile, err := uploadTestFile(t, token, user.ID, "private_doc.txt", "Dokumen rahasia.") // Helper dari test_helpers.go
	assert.NoError(t, err)
	assert.NotNil(t, uploadedFile)

	// Router untuk endpoint PATCH /api/file/:id
	routerTest.PATCH("/api/file/:id", func(c *gin.Context) { // RouterTest dari test_helpers.go
		tokenClaims, _ := config.NewJWTService().ValidateToken(token) // Harus pakai config.NewJWTService
		claims, ok := tokenClaims.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "error": "Invalid token claims type"})
			return
		}
		c.Set("user_id", fmt.Sprintf("%v", claims["user_id"]))
		fileControllerTest.UpdateByID(c) // fileControllerTest dari test_helpers.go
	})

	// 1. Ubah menjadi PUBLIC
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

	assert.Equal(t, http.StatusOK, wPublic.Code) // Harapan: Status 200 OK
	var respPublic struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(wPublic.Body.Bytes(), &respPublic)
	assert.NoError(t, err)
	assert.True(t, respPublic.Status)
	assert.True(t, *respPublic.Data.Shareable) // <--- PASTIKAN INI DEREFERENCE *bool
	assert.Equal(t, uploadedFile.ID, respPublic.Data.ID)


	// 2. Ubah kembali menjadi PRIVATE
	payloadPrivate := struct {
		Shareable bool `json:"shareable"`
	}{Shareable: false}
	bodyPrivate, _ := json.Marshal(payloadPrivate)
	reqPrivate, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPrivate))
	assert.NoError(t, err)
	reqPrivate.Header.Set("Content-Type", "application/json")
	reqPrivate.Header.Set("Authorization", "Bearer "+token)
	wPrivate := httptest.NewRecorder()
	routerTest.ServeHTTP(wPrivate, reqPrivate)

	assert.Equal(t, http.StatusOK, wPrivate.Code) // Harapan: Status 200 OK
	var respPrivate struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err = json.Unmarshal(wPrivate.Body.Bytes(), &respPrivate)
	assert.NoError(t, err)
	assert.True(t, respPrivate.Status)
	assert.False(t, *respPrivate.Data.Shareable) // <--- PASTIKAN INI DEREFERENCE *bool
	assert.Equal(t, uploadedFile.ID, respPrivate.Data.ID)
}

// Test_FileSharing_AccessPublicFile_OK: Menguji akses file publik tanpa autentikasi
func Test_FileSharing_AccessPublicFile_OK(t *testing.T) {
	SetupTestEnvironment() // Panggil Setup dari test_helpers.go
	defer TearDownTestEnvironment() // Panggil TearDown dari test_helpers.go

	// Buat user dan upload file
	user, token, err := createUserAndGetToken("public_owner", "public_owner123") // Helper dari test_helpers.go
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	uploadedFile, err := uploadTestFile(t, token, user.ID, "public_access.txt", "Ini file publik.") // Helper dari test_helpers.go
	assert.NoError(t, err)
	assert.NotNil(t, uploadedFile)

	// Ubah file menjadi PUBLIC (pastikan statusnya benar di database sebelum tes akses)
	routerTest.PATCH("/api/file/:id", func(c *gin.Context) { // RouterTest dari test_helpers.go
		tokenClaims, _ := config.NewJWTService().ValidateToken(token) // config.NewJWTService
		claims, ok := tokenClaims.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "error": "Invalid token claims type"})
			return
		}
		c.Set("user_id", fmt.Sprintf("%v", claims["user_id"]))
		fileControllerTest.UpdateByID(c) // fileControllerTest dari test_helpers.go
	})
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
	assert.Equal(t, http.StatusOK, wPublic.Code)

	// Coba akses file publik tanpa autentikasi (menggunakan /api/file/:id?view=true)
	// Router harus mendaftarkan rute ini tanpa middleware autentikasi ketat jika view=true
	routerTest.GET("/api/file/:id", fileControllerTest.GetFileByID) // routerTest & fileControllerTest dari test_helpers.go
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s?view=true", uploadedFile.ID), nil) // Tanpa header Auth
	assert.NoError(t, err)
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	assert.Equal(t, http.StatusOK, wAccess.Code)
	assert.Contains(t, wAccess.Body.String(), "Ini file publik.") // Asumsi konten file dikembalikan
}


// --- Test Baru: Menguji akses file private (BELUM ADA di kode Anda sebelumnya) ---

// Test_FileSharing_AccessPrivateFile_Unauthorized: Menguji akses file private tanpa autentikasi (anonim)
func Test_FileSharing_AccessPrivateFile_Unauthorized(t *testing.T) {
	SetupTestEnvironment()
	defer TearDownTestEnvironment()

	// 1. Buat user dan upload file, pastikan statusnya privat (default upload biasanya privat)
	user, token, err := createUserAndGetToken("private_owner", "private_owner123")
	assert.NoError(t, err)
	uploadedFile, err := uploadTestFile(t, token, user.ID, "private_doc_unauth.txt", "Ini dokumen privat.")
	assert.NoError(t, err)

	// Verifikasi file ini privat (shareable = false)
	// Jika default upload bukan privat, Anda perlu ubah jadi privat secara eksplisit di sini
	// Mengubah jadi privat secara eksplisit jika uploadTestFile membuat public secara default:
	routerTest.PATCH("/api/file/:id", func(c *gin.Context) {
		tokenClaims, _ := config.NewJWTService().ValidateToken(token)
		claims, ok := tokenClaims.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "error": "Invalid token claims type"})
			return
		}
		c.Set("user_id", fmt.Sprintf("%v", claims["user_id"]))
		fileControllerTest.UpdateByID(c)
	})
	payloadPrivateExplicit := struct { Shareable bool `json:"shareable"` }{Shareable: false}
	bodyPrivateExplicit, _ := json.Marshal(payloadPrivateExplicit)
	reqPrivateExplicit, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPrivateExplicit))
	assert.NoError(t, err)
	reqPrivateExplicit.Header.Set("Content-Type", "application/json")
	reqPrivateExplicit.Header.Set("Authorization", "Bearer "+token)
	wPrivateExplicit := httptest.NewRecorder()
	routerTest.ServeHTTP(wPrivateExplicit, reqPrivateExplicit)
	assert.Equal(t, http.StatusOK, wPrivateExplicit.Code)
	var respPrivateExplicit struct { Status bool `json:"status"`; Data entity.File `json:"data"` }
	err = json.Unmarshal(wPrivateExplicit.Body.Bytes(), &respPrivateExplicit)
	assert.NoError(t, err)
	assert.False(t, *respPrivateExplicit.Data.Shareable) // Pastikan sudah privat

	// 2. Coba akses file private tanpa autentikasi (anonim)
	routerTest.GET("/api/file/:id", fileControllerTest.GetFileByID) // Gunakan handler yang sama
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s?view=true", uploadedFile.ID), nil) // Tanpa header Auth
	assert.NoError(t, err)
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	// 3. Verifikasi respons
	assert.Equal(t, http.StatusUnauthorized, wAccess.Code) // Harapan: Status 401 Unauthorized atau 403 Forbidden
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

	// Pastikan file ini privat
	routerTest.PATCH("/api/file/:id", func(c *gin.Context) {
		tokenClaims, _ := config.NewJWTService().ValidateToken(tokenA)
		claims, ok := tokenClaims.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "error": "Invalid token claims type"})
			return
		}
		c.Set("user_id", fmt.Sprintf("%v", claims["user_id"]))
		fileControllerTest.UpdateByID(c)
	})
	payloadPrivateExplicit := struct { Shareable bool `json:"shareable"` }{Shareable: false}
	bodyPrivateExplicit, _ := json.Marshal(payloadPrivateExplicit)
	reqPrivateExplicit, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPrivateExplicit))
	assert.NoError(t, err)
	reqPrivateExplicit.Header.Set("Content-Type", "application/json")
	reqPrivateExplicit.Header.Set("Authorization", "Bearer "+tokenA)
	wPrivateExplicit := httptest.NewRecorder()
	routerTest.ServeHTTP(wPrivateExplicit, reqPrivateExplicit)
	assert.Equal(t, http.StatusOK, wPrivateExplicit.Code)
	var respPrivateExplicit struct { Status bool `json:"status"`; Data entity.File `json:"data"` }
	err = json.Unmarshal(wPrivateExplicit.Body.Bytes(), &respPrivateExplicit)
	assert.NoError(t, err)
	assert.False(t, *respPrivateExplicit.Data.Shareable) // Pastikan sudah privat

	// 2. Buat User B (bukan pemilik) dan login
	_, tokenB, err := createUserAndGetToken("userB_private", "passB123")
	assert.NoError(t, err)

	// 3. User B mencoba mengakses file milik User A (yang privat)
	routerTest.GET("/api/file/:id", middleware.Authenticate(jwtServiceTest), fileControllerTest.GetFileByID) // Gunakan middleware Autentikasi untuk User B
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s", uploadedFile.ID), nil) // Tanpa ?view=true, karena token sudah ada
	assert.NoError(t, err)
	reqAccess.Header.Set("Authorization", "Bearer "+tokenB) // Token User B
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	// 4. Verifikasi respons
	assert.Equal(t, http.StatusForbidden, wAccess.Code) // Harapan: Status 403 Forbidden
	assert.Contains(t, wAccess.Body.String(), "not authorized to perform this action") // Asumsi pesan otorisasi
}