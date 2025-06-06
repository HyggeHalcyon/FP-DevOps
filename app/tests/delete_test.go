package tests

import (
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

// SetupFileController: helper sama seperti di upload_test.go
func SetupDeleteController() controller.FileController {
	db := config.SetUpDatabaseConnection()
	fileRepo := repository.NewFileRepository(db)
	jwtService := config.NewJWTService()
	fileService := service.NewFileService(fileRepo)
	return controller.NewFileController(fileService, jwtService)
}

func setupRouterWithFileController() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	fileController := SetupFileController()
	r.POST("/api/upload", fileController.Create)
	r.DELETE("/api/delete/:filename", fileController.DeleteByID)
	return r
}

// Test_DeleteFile_OK: Upload file terlebih dahulu, lalu hapus dan verifikasi.
func Test_DeleteFile_OK(t *testing.T) {
	os.MkdirAll("uploads", os.ModePerm)
	t.Cleanup(func() { os.RemoveAll("uploads") })

	r := setupRouterWithFileController()

	// Upload file dummy
	fileContent := []byte("to be deleted")
	req, err := CreateMultipartRequest("file", "delete-me.txt", fileContent)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Pastikan file eksis
	filePath := "uploads/delete-me.txt"
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Delete file
	deleteReq, _ := http.NewRequest("DELETE", "/api/delete/delete-me.txt", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, deleteReq)

	assert.Equal(t, http.StatusOK, w2.Code)

	// Pastikan file sudah tidak ada
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

// Test_DeleteFile_NotFound: File tidak ada
func Test_DeleteFile_NotFound(t *testing.T) {
	r := setupRouterWithFileController()

	req, _ := http.NewRequest("DELETE", "/api/delete/notfound.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// OPTIONAL: Test jika endpoint protected pakai JWT (simulasikan tanpa token)
func Test_DeleteFile_Unauthorized(t *testing.T) {
	// Misal: controller.Delete cek token
	r := gin.Default()

	fileController := SetupFileController()
	authMiddleware := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	r.DELETE("/api/delete/:filename", authMiddleware, fileController.DeleteByID)

	req, _ := http.NewRequest("DELETE", "/api/delete/secure.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
