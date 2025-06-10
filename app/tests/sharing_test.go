package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"FP-DevOps/middleware"
	"FP-DevOps/repository"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Insert a single test user and return it
func InsertFileUser() (entity.User, error) {
	db := config.SetUpDatabaseConnection()
	user := entity.User{
		ID:       uuid.New(),
		Username: "testuser",
		Password: "password123", // adjust if you hash passwords in your model
	}
	if err := db.Create(&user).Error; err != nil {
		return entity.User{}, err
	}
	return user, nil
}

// Insert files for the given userID
func InsertTestFiles(userID uuid.UUID) ([]entity.File, error) {
	db := config.SetUpDatabaseConnection()
	files := []entity.File{
		{
			ID:       uuid.New(),
			Filename: "a.txt",
			UserID:   userID,
		},
		{
			ID:       uuid.New(),
			Filename: "KEGANTICOY.txt",
			UserID:   userID,
		},
	}
	for _, f := range files {
		if err := db.Create(&f).Error; err != nil {
			return nil, err
		}
	}
	return files, nil
}

// Clean up both files and the test user
func CleanUpTestData(userID uuid.UUID) {
	db := config.SetUpDatabaseConnection()
	db.Exec("DELETE FROM files")
	db.Exec("DELETE FROM users WHERE id = ?", userID)
}

func TestEditFile(t *testing.T) {
	CleanUpTestData(uuid.Nil)

	// 1) insert user
	user, err := InsertFileUser()
	assert.NoError(t, err)

	// 2) insert files for that user
	files, err := InsertTestFiles(user.ID)
	assert.NoError(t, err)
	assert.Len(t, files, 2)

	// 3) set up JWT service & controller
	jwtSvc := config.NewJWTService()
	fc := controller.NewFileController(
		service.NewFileService(repository.NewFileRepository(config.SetUpDatabaseConnection())),
		jwtSvc,
	)

	// 4) build a Gin router with your Authenticate middleware
	r := gin.Default()
	fg := r.Group("/files")
	fg.Use(middleware.Authenticate(jwtSvc))
	fg.PUT("/:id", fc.UpdateByID)

	// 5) generate a valid token for our test user
	token := jwtSvc.GenerateToken(user.ID.String(), user.Username)

	// 6) craft the PUT /files/:id request
	payload := dto.FileUpdate{Filename: "updated.txt"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/files/"+files[0].ID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// 7) execute
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// debug output on failure
	if w.Code != http.StatusCreated {
		t.Logf("got status %d, body: %s", w.Code, w.Body.String())
	}
	assert.Equal(t, http.StatusCreated, w.Code)

	// 8) tear down
	CleanUpTestData(user.ID)
}
