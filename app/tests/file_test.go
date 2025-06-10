package tests

import (
	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/dto"
	"FP-DevOps/entity"
	"FP-DevOps/middleware"
	"FP-DevOps/repository"
	"FP-DevOps/service"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func SetupControllerFile() controller.FileController {
	var (
		db             = config.SetUpDatabaseConnection()
		fileRepo       = repository.NewFileRepository(db)
		jwtService     = config.NewJWTService()
		fileService    = service.NewFileService(fileRepo)
		fileController = controller.NewFileController(fileService, jwtService)
	)

	return fileController
}

func loginTestAccount(t *testing.T, username string, password string) string {
	r := SetUpRoutes()
	uc := SetupControllerUser()
	InsertTestUser()

	r.POST("/api/user/login", uc.Login)

	payload := dto.UserRequest{
		Username: username,
		Password: password,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	type Resp struct {
		Data struct {
			Token string `json:"token"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	var resp Resp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, err)
	assert.NotEmpty(t, resp.Data.Token)

	return resp.Data.Token
}

func uploadTestFile(t *testing.T, router http.Handler, token string, filename, content string) dto.FileResponse {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	assert.NoError(t, err)

	_, err = io.Copy(part, bytes.NewBufferString(content))
	assert.NoError(t, err)

	writer.Close()

	req, _ := http.NewRequest("POST", "/api/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)

	var response struct {
		Status bool             `json:"status"`
		Data   dto.FileResponse `json:"data"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Status)
	assert.Equal(t, filename, response.Data.Filename)
	assert.True(t, response.Data.ID != "")

	return response.Data
}

func Test_FileDelete_OK(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	r.DELETE("/api/file/:id", middleware.Authenticate(jwtService), fc.DeleteByID)

	file := uploadTestFile(t, r, token, "file-to-delete.txt", "content to delete")

	req, _ := http.NewRequest("DELETE", "/api/file/"+file.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func Test_FileDelete_NotOwner(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	r.DELETE("/api/file/:id", middleware.Authenticate(jwtService), fc.DeleteByID)

	file := uploadTestFile(t, r, token, "file-to-protect.txt", "content")

	otherToken := loginTestAccount(t, "admin", "admin123")
	req, _ := http.NewRequest("DELETE", "/api/file/"+file.ID, nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func Test_FileDelete_NotFound(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.DELETE("/api/file/:id", middleware.Authenticate(jwtService), fc.DeleteByID)

	req, _ := http.NewRequest("DELETE", "/api/file/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func Test_FileDelete_NoToken(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	r.DELETE("/api/file/:id", middleware.Authenticate(jwtService), fc.DeleteByID)

	file := uploadTestFile(t, r, token, "file-to-delete.txt", "content to delete")

	req, _ := http.NewRequest("DELETE", "/api/file/"+file.ID, nil)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func Test_FileEdit_Rename_OK(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	r.PATCH("/api/file/:id", middleware.Authenticate(jwtService), fc.UpdateByID)

	file := uploadTestFile(t, r, token, "old-name.txt", "content to rename")

	newFileName := "new-name.txt"

	payload := dto.FileUpdate{
		Filename: newFileName,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PATCH", "/api/file/"+file.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Status bool        `json:"status"`
		Data   entity.File `json:"data"`
	}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Status)
	assert.Equal(t, newFileName, response.Data.Filename)
}

func Test_FileEdit_Rename_NotOwner(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	r.PATCH("/api/file/:id", middleware.Authenticate(jwtService), fc.UpdateByID)

	file := uploadTestFile(t, r, token, "file-to-protect.txt", "content")

	otherToken := loginTestAccount(t, "admin", "admin123")
	payload := dto.FileUpdate{
		Filename: "new-name.txt",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PATCH", "/api/file/"+file.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+otherToken)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func Test_FileSharing_TogglePublic_OK(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	r.PATCH("/api/file/:id", middleware.Authenticate(jwtService), fc.UpdateByID)

	file := uploadTestFile(t, r, token, "file-to-share.txt", "content to share")

	_true := true
	payload := dto.FileUpdate{
		Shareable: &_true,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PATCH", "/api/file/"+file.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Status bool             `json:"status"`
		Data   dto.FileResponse `json:"data"`
	}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Status)
	assert.True(t, *response.Data.Shareable)
}

func Test_FileSharing_AccessPublicFile_OK(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.GET("/api/file/:id", middleware.AuthenticateIfExists(jwtService), fc.GetFileByID)
	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	r.PATCH("/api/file/:id", middleware.Authenticate(jwtService), fc.UpdateByID)

	file := uploadTestFile(t, r, token, "public-file.txt", "public content")

	_true := true
	payload := dto.FileUpdate{
		Shareable: &_true,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PATCH", "/api/file/"+file.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	req, _ = http.NewRequest("GET", "/api/file/"+file.ID, nil)
	recorder = httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "public content", recorder.Body.String())
}

func Test_FileUpload_OK(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()
	token := loginTestAccount(t, "user", "user123")

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)

	filename := "uploaded-file.txt"
	content := "This is a test file upload."

	file := uploadTestFile(t, r, token, filename, content)

	assert.Equal(t, filename, file.Filename)
	assert.True(t, file.ID != "")
}

func Test_FileUpload_Unauthorized(t *testing.T) {
	r := SetUpRoutes()
	fc := SetupControllerFile()
	jwtService := config.NewJWTService()
	CleanUpTestUsers()

	r.POST("/api/file", middleware.Authenticate(jwtService), fc.Create)
	content := "This file upload should fail."

	req, _ := http.NewRequest("POST", "/api/file", bytes.NewBufferString(content))
	req.Header.Set("Content-Type", "application/octet-stream")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
