package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"FP-DevOps/entity"
	"FP-DevOps/dto" // Diperlukan untuk dto.FileUpdate

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Tes Fungsional untuk Fitur Berbagi File
// Bagian ini berisi kasus uji untuk fitur berbagi file (public/private access).
// =============================================================================

// Test_FileSharing_TogglePublic_OK: Menguji skenario sukses mengubah status shareable file
func Test_FileSharing_TogglePublic_OK(t *testing.T) {
	// Pastikan lingkungan tes siap. TestMain sudah memanggil SetupTestEnvironment.
	// Jika ingin menjalankan tes ini secara individual (`go test -run Test_FileSharing_TogglePublic_OK`),
	// Anda mungkin perlu memanggil `SetupTestEnvironment()` dan `defer TearDownTestEnvironment()` di sini.
	// Namun, dengan TestMain, ini tidak diperlukan lagi.
	// SetupTestEnvironment()
	// defer TearDownTestEnvironment()

	// 1. Buat pengguna dan dapatkan token mereka
	user, token, err := createUserAndGetToken("sharer", "sharer123")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	// 2. Unggah file sebagai pengguna tersebut
	uploadedFile, err := uploadTestFile(t, token, user.ID, "private_doc.txt", "Dokumen rahasia.")
	assert.NoError(t, err)
	assert.NotNil(t, uploadedFile)

	// 3. Ubah status file menjadi PUBLIC
	payloadPublic := dto.FileUpdate{Shareable: new(bool)} // Inisialisasi *bool
	*payloadPublic.Shareable = true                       // Set nilainya
	bodyPublic, _ := json.Marshal(payloadPublic)
	reqPublic, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPublic))
	assert.NoError(t, err)
	reqPublic.Header.Set("Content-Type", "application/json")
	reqPublic.Header.Set("Authorization", "Bearer "+token) // Token diperlukan untuk otorisasi
	wPublic := httptest.NewRecorder()
	routerTest.ServeHTTP(wPublic, reqPublic)

	// Verifikasi respons setelah mengubah ke PUBLIC
	assert.Equal(t, http.StatusOK, wPublic.Code) // Harapan: Status 200 OK
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
	payloadPrivate := dto.FileUpdate{Shareable: new(bool)} // Inisialisasi *bool
	*payloadPrivate.Shareable = false                      // Set nilainya
	bodyPrivate, _ := json.Marshal(payloadPrivate)
	reqPrivate, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPrivate))
	assert.NoError(t, err)
	reqPrivate.Header.Set("Content-Type", "application/json")
	reqPrivate.Header.Set("Authorization", "Bearer "+token) // Token diperlukan untuk otorisasi
	wPrivate := httptest.NewRecorder()
	routerTest.ServeHTTP(wPrivate, reqPrivate)

	// Verifikasi respons setelah mengubah kembali ke PRIVATE
	assert.Equal(t, http.StatusOK, wPrivate.Code) // Harapan: Status 200 OK
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
	// SetupTestEnvironment()
	// defer TearDownTestEnvironment()

	// 1. Buat pengguna dan upload file
	user, token, err := createUserAndGetToken("public_owner", "public_owner123")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)

	uploadedFile, err := uploadTestFile(t, token, user.ID, "public_access.txt", "Ini file publik.")
	assert.NoError(t, err)
	assert.NotNil(t, uploadedFile)

	// 2. Ubah file menjadi PUBLIC
	// rute PATCH untuk file sudah terdaftar dengan middleware autentikasi di SetupTestEnvironment().
	payloadPublic := dto.FileUpdate{Shareable: new(bool)}
	*payloadPublic.Shareable = true
	bodyPublic, _ := json.Marshal(payloadPublic)
	reqPublic, err := http.NewRequest("PATCH", fmt.Sprintf("/api/file/%s", uploadedFile.ID), bytes.NewBuffer(bodyPublic))
	assert.NoError(t, err)
	reqPublic.Header.Set("Content-Type", "application/json")
	reqPublic.Header.Set("Authorization", "Bearer "+token)
	wPublic := httptest.NewRecorder()
	routerTest.ServeHTTP(wPublic, reqPublic)
	assert.Equal(t, http.StatusOK, wPublic.Code) // Verifikasi perubahan status sukses

	// 3. Coba akses file publik tanpa autentikasi (menggunakan /api/file/:id?view=true)
	// rute GET untuk file sudah terdaftar di SetupTestEnvironment().
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
	// SetupTestEnvironment()
	// defer TearDownTestEnvironment()

	// 1. Buat pengguna dan unggah file, pastikan statusnya privat (default upload biasanya privat)
	user, token, err := createUserAndGetToken("private_owner", "private_owner123")
	assert.NoError(t, err)
	uploadedFile, err := uploadTestFile(t, token, user.ID, "private_doc_unauth.txt", "Ini dokumen privat.")
	assert.NoError(t, err)

	// 2. Verifikasi file ini privat (shareable = false) secara eksplisit jika diperlukan
	// rute PATCH untuk file sudah terdaftar dengan middleware autentikasi di SetupTestEnvironment().
	payloadPrivateExplicit := dto.FileUpdate{Shareable: new(bool)}
	*payloadPrivateExplicit.Shareable = false
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
	// rute GET untuk file sudah terdaftar di SetupTestEnvironment().
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
	// SetupTestEnvironment()
	// defer TearDownTestEnvironment()

	// 1. Buat User A (pemilik file)
	userA, tokenA, err := createUserAndGetToken("ownerA_private", "passA123")
	assert.NoError(t, err)
	uploadedFile, err := uploadTestFile(t, tokenA, userA.ID, "ownerA_private_file.txt", "File pribadi A.")
	assert.NoError(t, err)

	// 2. Pastikan file ini privat
	// rute PATCH untuk file sudah terdaftar dengan middleware autentikasi di SetupTestEnvironment().
	payloadPrivateExplicit := dto.FileUpdate{Shareable: new(bool)}
	*payloadPrivateExplicit.Shareable = false
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
	// rute GET untuk file sudah terdaftar dengan middleware autentikasi di SetupTestEnvironment().
	reqAccess, err := http.NewRequest("GET", fmt.Sprintf("/api/file/%s", uploadedFile.ID), nil) // Tanpa ?view=true, karena ada token
	assert.NoError(t, err)
	reqAccess.Header.Set("Authorization", "Bearer "+tokenB) // Menggunakan token User B
	wAccess := httptest.NewRecorder()
	routerTest.ServeHTTP(wAccess, reqAccess)

	// 5. Verifikasi respons
	assert.Equal(t, http.StatusForbidden, wAccess.Code) // Harapan: Status 403 Forbidden
	assert.Contains(t, wAccess.Body.String(), "not authorized to perform this action") // Asumsi pesan otorisasi dari controller
}