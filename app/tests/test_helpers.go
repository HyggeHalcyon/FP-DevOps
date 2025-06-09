// app/tests/test_helpers.go
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
	"path/filepath"
	"testing"

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/entity"
	"FP-DevOps/middleware"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// --- Variabel Global untuk Lingkungan Tes (HANYA di sini definisinya) ---
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

// --- Setup dan TearDown Lingkungan Tes (HANYA di sini definisinya) ---
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

	// Definisikan semua rute yang mungkin diuji di sini
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

func TearDownTestEnvironment() {
	sqlDB, err := dbTest.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
	}
	if sqlDB != nil {
		sqlDB.Close()
	}
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

// --- Helper Functions untuk Tes (HANYA di sini definisinya) ---

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

func uploadTestFile(t *testing.T, token string, userID uuid.UUID, filename, content string) (entity.File, error) {
	routerTest.POST("/api/file", func(c *gin.Context) {
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
		fileControllerTest.Create(c)
	})

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