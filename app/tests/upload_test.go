package tests

import (
	"bytes"
	"io/ioutil"
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
)

func SetupFileController() controller.FileController {
	db := config.SetUpDatabaseConnection()
	fileRepo := repository.NewFileRepository(db)
	jwtService := config.NewJWTService()
	fileService := service.NewFileService(fileRepo)
	return controller.NewFileController(fileService, jwtService)
}

func CreateMultipartRequest(paramName, fileName string, fileContent []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(paramName, fileName)
	if err != nil {
		return nil, err
	}

	_, err = part.Write(fileContent)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "/api/upload", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func cleanUploadsDir(t *testing.T) {
	err := os.RemoveAll("uploads")
	if err != nil {
		t.Logf("Failed to cleanup uploads directory: %v", err)
	}
	err = os.MkdirAll("uploads", os.ModePerm)
	assert.NoError(t, err)
}

func InsertFileUser() (entity.User, error) {
	db := config.SetUpDatabaseConnection()
	user := entity.User{
		ID:       uuid.New(),
		Username: "uploadtestuser_" + uuid.New().String()[:8],
		Password: "password123",
	}
	if err := db.Create(&user).Error; err != nil {
		return entity.User{}, err
	}
	return user, nil
}

func CleanUpTestData(userID uuid.UUID) {
	db := config.SetUpDatabaseConnection()
	db.Where("user_id = ?", userID).Delete(&entity.File{})
	db.Where("id = ?", userID).Delete(&entity.User{})
}

func Test_UploadFile_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	user, err := InsertFileUser()
	assert.NoError(t, err)
	t.Cleanup(func() { CleanUpTestData(user.ID) })

	jwtSvc := config.NewJWTService()
	fileController := SetupFileController()

	r := gin.Default()
	r.Use(middleware.Authenticate(jwtSvc))
	r.POST("/api/upload", fileController.Create)

	token := jwtSvc.GenerateToken(user.ID.String(), user.Username)

	fileName := "dummy.txt"
	dummyContent := []byte("ini adalah konten file uji")
	req, err := CreateMultipartRequest("file", fileName, dummyContent)
	assert.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	if w.Code != http.StatusCreated {
		t.Logf("Response body: %s", w.Body.String())
	}

	db := config.SetUpDatabaseConnection()
	var fileInDB entity.File
	err = db.Where("filename = ? AND user_id = ?", fileName, user.ID).First(&fileInDB).Error
	assert.NoError(t, err)
	assert.Equal(t, fileName, fileInDB.Filename)
	assert.Equal(t, user.ID, fileInDB.UserID)
	assert.NotEmpty(t, fileInDB.ID)

	expectedFilePath := "uploads/" + user.ID.String() + "/" + fileName
	_, err = os.Stat(expectedFilePath)
	assert.NoError(t, err)

	readContent, err := ioutil.ReadFile(expectedFilePath)
	assert.NoError(t, err)
	assert.Equal(t, dummyContent, readContent)
}

func Test_UploadFile_TooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	user, err := InsertFileUser()
	assert.NoError(t, err)
	t.Cleanup(func() { CleanUpTestData(user.ID) })

	jwtSvc := config.NewJWTService()
	fileController := SetupFileController()

	r := gin.Default()
	r.MaxMultipartMemory = 20 << 20
	r.Use(middleware.Authenticate(jwtSvc))
	r.POST("/api/upload", fileController.Create)

	token := jwtSvc.GenerateToken(user.ID.String(), user.Username)

	largeData := make([]byte, 21<<20)
	req, err := CreateMultipartRequest("file", "large.txt", largeData)
	assert.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	db := config.SetUpDatabaseConnection()
	var fileInDB entity.File
	err = db.Where("filename = ? AND user_id = ?", "large.txt", user.ID).First(&fileInDB).Error
	assert.Error(t, err)
	assert.True(t, db.Error != nil && db.Error.Error() == "record not found")

	filePath := "uploads/" + user.ID.String() + "/large.txt"
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func Test_UploadFile_NoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	user, err := InsertFileUser()
	assert.NoError(t, err)
	t.Cleanup(func() { CleanUpTestData(user.ID) })

	jwtSvc := config.NewJWTService()
	fileController := SetupFileController()

	r := gin.Default()
	r.Use(middleware.Authenticate(jwtSvc))
	r.POST("/api/upload", fileController.Create)

	token := jwtSvc.GenerateToken(user.ID.String(), user.Username)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err = writer.Close()
	assert.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/upload", body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	db := config.SetUpDatabaseConnection()
	var fileInDB entity.File
	err = db.Where("user_id = ?", user.ID).First(&fileInDB).Error
	assert.Error(t, err)
	assert.True(t, db.Error != nil && db.Error.Error() == "record not found")

	userUploadDir := "uploads/" + user.ID.String()
	_, err = os.Stat(userUploadDir)
	if err == nil {
		files, err := ioutil.ReadDir(userUploadDir)
		assert.NoError(t, err)
		assert.Empty(t, files)
	} else {
		assert.True(t, os.IsNotExist(err))
	}
}

func Test_UploadFile_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanUploadsDir(t)
	t.Cleanup(func() { cleanUploadsDir(t) })

	jwtSvc := config.NewJWTService()
	fileController := SetupFileController()

	r := gin.Default()
	r.Use(middleware.Authenticate(jwtSvc))
	r.POST("/api/upload", fileController.Create)

	fileName := "unauth_file.txt"
	dummyContent := []byte("konten file tidak terautentikasi")
	req, err := CreateMultipartRequest("file", fileName, dummyContent)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	db := config.SetUpDatabaseConnection()
	var fileInDB entity.File
	err = db.Where("filename = ?", fileName).First(&fileInDB).Error
	assert.Error(t, err)
	assert.True(t, db.Error != nil && db.Error.Error() == "record not found")

	_, err = os.Stat("uploads/" + fileName)
	assert.True(t, os.IsNotExist(err))
}