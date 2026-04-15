package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
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
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
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

func withWorkingDirectory(t *testing.T, dir string) {
	t.Helper()

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})
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

func sampleLargePNGBytes(t *testing.T, width int, height int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	widthRange := width - 1
	if widthRange < 1 {
		widthRange = 1
	}
	heightRange := height - 1
	if heightRange < 1 {
		heightRange = 1
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{
				R: uint8((x * 255) / widthRange),
				G: uint8((y * 255) / heightRange),
				B: 180,
				A: 255,
			})
		}
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("failed to encode large sample png: %v", err)
	}

	return buffer.Bytes()
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

func TestUploadLogoStoresImageInMountedDataDirectory(t *testing.T) {
	setupLogoControllerTestDB(t)

	tempRoot := t.TempDir()
	dataDir := filepath.Join(tempRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("failed to create mounted data directory: %v", err)
	}
	withWorkingDirectory(t, dataDir)

	req, err := createMultipartLogoRequest(
		t,
		logoUploadFormField,
		"logo.png",
		samplePNGBytes(t),
	)
	if err != nil {
		t.Fatalf("failed to create upload request: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	UploadLogo(ctx)

	response := decodeLogoUploadResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected upload to succeed, got message: %s", response.Message)
	}

	expectedStoragePath := filepath.Join(dataDir, "system-assets", "logo-upload.bin")
	storedContent, err := os.ReadFile(expectedStoragePath)
	if err != nil {
		t.Fatalf("expected uploaded logo at mounted data directory %q: %v", expectedStoragePath, err)
	}
	if !bytes.Equal(storedContent, samplePNGBytes(t)) {
		t.Fatalf("stored logo bytes did not match uploaded content")
	}

	legacyStoragePath := filepath.Join(dataDir, "data", "system-assets", "logo-upload.bin")
	if _, err := os.Stat(legacyStoragePath); err == nil {
		t.Fatalf("expected no file to be written to legacy nested data directory %q", legacyStoragePath)
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
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Fatalf("expected unversioned logo to disable cache, got %q", cacheControl)
	}
}

func TestGetLogoServesVersionedThumbnailWithImmutableCache(t *testing.T) {
	setTempLogoStoragePath(t)

	content := sampleLargePNGBytes(t, 240, 120)
	if err := os.MkdirAll(filepath.Dir(logoStoragePath), 0o755); err != nil {
		t.Fatalf("failed to create temp logo storage directory: %v", err)
	}
	if err := os.WriteFile(logoStoragePath, content, 0o644); err != nil {
		t.Fatalf("failed to write stored logo: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/logo?v=123&size=64",
		nil,
	)

	GetLogo(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("expected resized logo to be served as image/png, got %q", contentType)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("expected versioned thumbnail to be immutable, got %q", cacheControl)
	}

	resizedConfig, _, err := image.DecodeConfig(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to decode resized logo: %v", err)
	}
	if resizedConfig.Width != 64 || resizedConfig.Height != 32 {
		t.Fatalf(
			"expected 240x120 logo to resize to 64x32, got %dx%d",
			resizedConfig.Width,
			resizedConfig.Height,
		)
	}
}

func TestGetLogoServesImageFromMountedDataDirectory(t *testing.T) {
	tempRoot := t.TempDir()
	dataDir := filepath.Join(tempRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("failed to create mounted data directory: %v", err)
	}
	withWorkingDirectory(t, dataDir)

	content := samplePNGBytes(t)
	expectedStoragePath := filepath.Join(dataDir, "system-assets", "logo-upload.bin")
	if err := os.MkdirAll(filepath.Dir(expectedStoragePath), 0o755); err != nil {
		t.Fatalf("failed to create mounted logo storage directory: %v", err)
	}
	if err := os.WriteFile(expectedStoragePath, content, 0o644); err != nil {
		t.Fatalf("failed to write mounted logo file: %v", err)
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
		t.Fatalf("served body did not match mounted logo bytes")
	}
}

func TestGetLogoFallsBackToLegacyNestedDataDirectory(t *testing.T) {
	tempRoot := t.TempDir()
	dataDir := filepath.Join(tempRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("failed to create mounted data directory: %v", err)
	}
	withWorkingDirectory(t, dataDir)

	content := samplePNGBytes(t)
	legacyStoragePath := filepath.Join(
		dataDir,
		"data",
		"system-assets",
		"logo-upload.bin",
	)
	if err := os.MkdirAll(filepath.Dir(legacyStoragePath), 0o755); err != nil {
		t.Fatalf("failed to create legacy logo storage directory: %v", err)
	}
	if err := os.WriteFile(legacyStoragePath, content, 0o644); err != nil {
		t.Fatalf("failed to write legacy logo file: %v", err)
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
		t.Fatalf("served body did not match legacy logo bytes")
	}
}

func TestGetLogoReturnsNotFoundWithoutStoredImage(t *testing.T) {
	setTempLogoStoragePath(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/logo", nil)

	GetLogo(ctx)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestResolveLogoOptionValueHidesDefaultFallbackAndMissingUpload(t *testing.T) {
	setTempLogoStoragePath(t)

	originalLogo := common.Logo
	t.Cleanup(func() {
		common.Logo = originalLogo
	})

	common.Logo = "/logo.png"
	if got := resolveLogoOptionValue(); got != "" {
		t.Fatalf("expected default logo fallback to be hidden, got %q", got)
	}

	common.Logo = "/api/logo?v=123"
	if got := resolveLogoOptionValue(); got != "" {
		t.Fatalf("expected missing uploaded logo to be hidden, got %q", got)
	}

	content := samplePNGBytes(t)
	if err := os.MkdirAll(filepath.Dir(logoStoragePath), 0o755); err != nil {
		t.Fatalf("failed to create temp logo storage directory: %v", err)
	}
	if err := os.WriteFile(logoStoragePath, content, 0o644); err != nil {
		t.Fatalf("failed to write stored logo: %v", err)
	}

	if got := resolveLogoOptionValue(); got != common.Logo {
		t.Fatalf("expected uploaded logo URL to be preserved, got %q", got)
	}
}
