package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type logoUploadResponse struct {
	Success bool `json:"success"`
	Message string `json:"message"`
	Data struct {
		URL string `json:"url"`
	} `json:"data"`
}

func setupLogoControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.OptionMap = map[string]string{}

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.Option{}); err != nil {
		t.Fatalf("failed to migrate option table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func setTempLogoStoragePath(t *testing.T) string {
	t.Helper()

	previousPath := logoStoragePath
	logoStoragePath = filepath.Join(t.TempDir(), "logo-upload.bin")
	t.Cleanup(func() {
		logoStoragePath = previousPath
	})
	return logoStoragePath
}

func decodeLogoUploadResponse(t *testing.T, recorder *httptest.ResponseRecorder) logoUploadResponse {
	t.Helper()

	var response logoUploadResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}
	return response
}

func createMultipartLogoRequest(t *testing.T, fieldName string, fileName string, content []byte) (*http.Request, error) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, err
	}
	if _, err := fileWriter.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, "/api/option/logo", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func samplePNGBytes(t *testing.T) []byte {
	t.Helper()

	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aJkQAAAAASUVORK5CYII="
	data, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		t.Fatalf("failed to decode sample png: %v", err)
	}
	return data
}

func sampleGIFBytes(t *testing.T) []byte {
	t.Helper()

	const gifBase64 = "R0lGODdhAQABAIABAP///wAAACwAAAAAAQABAAACAkQBADs="
	data, err := base64.StdEncoding.DecodeString(gifBase64)
	if err != nil {
		t.Fatalf("failed to decode sample gif: %v", err)
	}
	return data
}

func TestUploadLogoUpdatesOptionAndOverwritesExistingFile(t *testing.T) {
	setupLogoControllerTestDB(t)
	storagePath := setTempLogoStoragePath(t)

	firstRequest, err := createMultipartLogoRequest(t, logoUploadFormField, "logo.png", samplePNGBytes(t))
	if err != nil {
		t.Fatalf("failed to create first multipart request: %v", err)
	}
	firstRecorder := httptest.NewRecorder()
	firstContext, _ := gin.CreateTestContext(firstRecorder)
	firstContext.Request = firstRequest

	UploadLogo(firstContext)

	firstResponse := decodeLogoUploadResponse(t, firstRecorder)
	if !firstResponse.Success {
		t.Fatalf("expected first upload to succeed, got message: %s", firstResponse.Message)
	}
	if !strings.HasPrefix(firstResponse.Data.URL, "/api/logo?v=") {
		t.Fatalf("expected versioned api logo url, got %q", firstResponse.Data.URL)
	}
	if common.Logo != firstResponse.Data.URL {
		t.Fatalf("expected common.Logo to be updated to %q, got %q", firstResponse.Data.URL, common.Logo)
	}

	storedAfterFirstUpload, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("failed to read stored logo after first upload: %v", err)
	}
	if !bytes.Equal(storedAfterFirstUpload, samplePNGBytes(t)) {
		t.Fatalf("stored file did not match first upload")
	}

	secondRequest, err := createMultipartLogoRequest(t, logoUploadFormField, "logo.gif", sampleGIFBytes(t))
	if err != nil {
		t.Fatalf("failed to create second multipart request: %v", err)
	}
	secondRecorder := httptest.NewRecorder()
	secondContext, _ := gin.CreateTestContext(secondRecorder)
	secondContext.Request = secondRequest

	UploadLogo(secondContext)

	secondResponse := decodeLogoUploadResponse(t, secondRecorder)
	if !secondResponse.Success {
		t.Fatalf("expected second upload to succeed, got message: %s", secondResponse.Message)
	}

	storedAfterSecondUpload, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("failed to read stored logo after second upload: %v", err)
	}
	if !bytes.Equal(storedAfterSecondUpload, sampleGIFBytes(t)) {
		t.Fatalf("expected second upload to overwrite stored file")
	}
}

func TestUploadLogoRejectsInvalidFileType(t *testing.T) {
	setupLogoControllerTestDB(t)
	storagePath := setTempLogoStoragePath(t)

	req, err := createMultipartLogoRequest(t, logoUploadFormField, "logo.txt", []byte("not an image"))
	if err != nil {
		t.Fatalf("failed to create invalid upload request: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	UploadLogo(ctx)

	response := decodeLogoUploadResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected invalid file upload to fail")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request status, got %d", recorder.Code)
	}
	if _, err := os.Stat(storagePath); !os.IsNotExist(err) {
		t.Fatalf("expected no file to be stored for invalid upload")
	}
}

func TestGetLogoServesStoredImage(t *testing.T) {
	setTempLogoStoragePath(t)

	content := samplePNGBytes(t)
	if err := os.MkdirAll(filepath.Dir(logoStoragePath), 0o755); err != nil {
		t.Fatalf("failed to create temp logo storage directory: %v", err)
	}
	if err := os.WriteFile(logoStoragePath, content, 0o644); err != nil {
		t.Fatalf("failed to write stored logo: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/logo", nil)

	GetLogo(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("expected image/png content type, got %q", contentType)
	}
	if !bytes.Equal(recorder.Body.Bytes(), content) {
		t.Fatalf("served body did not match stored logo bytes")
	}
}
