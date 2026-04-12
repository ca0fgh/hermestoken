package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	logoUploadFormField          = "logo"
	logoPublicPath               = "/api/logo"
	maxLogoUploadBytes     int64 = 5 << 20
	defaultLogoStoragePath       = "data/system-assets/logo-upload.bin"
)

var (
	logoStoragePath  = defaultLogoStoragePath
	allowedLogoMIMEs = map[string]struct{}{
		"image/gif":  {},
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	}
)

func getLogoStoragePath() string {
	if logoStoragePath != defaultLogoStoragePath {
		return logoStoragePath
	}

	if dataDir := os.Getenv("ELECTRON_DATA_DIR"); dataDir != "" {
		return filepath.Join(dataDir, "system-assets", "logo-upload.bin")
	}

	workingDir, err := os.Getwd()
	if err == nil && filepath.Base(filepath.Clean(workingDir)) == "data" {
		return filepath.Join(workingDir, "system-assets", "logo-upload.bin")
	}

	return defaultLogoStoragePath
}

func getLogoReadPaths() []string {
	primaryPath := getLogoStoragePath()
	if logoStoragePath != defaultLogoStoragePath {
		return []string{primaryPath}
	}
	if filepath.Clean(primaryPath) == filepath.Clean(defaultLogoStoragePath) {
		return []string{primaryPath}
	}
	return []string{primaryPath, defaultLogoStoragePath}
}

func readStoredLogo() ([]byte, error) {
	var lastErr error

	for _, path := range getLogoReadPaths() {
		content, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		if len(content) == 0 {
			lastErr = os.ErrNotExist
			continue
		}
		return content, nil
	}

	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return nil, lastErr
}

func removeLegacyLogoFile(primaryPath string) {
	if logoStoragePath != defaultLogoStoragePath {
		return
	}

	if filepath.Clean(primaryPath) == filepath.Clean(defaultLogoStoragePath) {
		return
	}

	_ = os.Remove(defaultLogoStoragePath)
}

func isAllowedLogoMIME(contentType string) bool {
	_, ok := allowedLogoMIMEs[contentType]
	return ok
}

func writeLogoError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"success": false,
		"message": message,
	})
}

func UploadLogo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLogoUploadBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(maxLogoUploadBytes); err != nil {
		writeLogoError(c, http.StatusBadRequest, "Logo 文件过大或表单无效")
		return
	}

	fileHeader, err := c.FormFile(logoUploadFormField)
	if err != nil {
		writeLogoError(c, http.StatusBadRequest, "请上传 Logo 图片")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		writeLogoError(c, http.StatusBadRequest, "无法读取上传的 Logo 文件")
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxLogoUploadBytes+1))
	if err != nil {
		writeLogoError(c, http.StatusBadRequest, "无法读取上传的 Logo 文件")
		return
	}
	if len(content) == 0 {
		writeLogoError(c, http.StatusBadRequest, "Logo 文件不能为空")
		return
	}
	if int64(len(content)) > maxLogoUploadBytes {
		writeLogoError(c, http.StatusBadRequest, "Logo 文件不能超过 5 MB")
		return
	}

	contentType := http.DetectContentType(content)
	if !isAllowedLogoMIME(contentType) {
		writeLogoError(c, http.StatusBadRequest, "仅支持 PNG、JPEG、GIF、WebP 格式的 Logo 图片")
		return
	}

	storagePath := getLogoStoragePath()
	if err := os.MkdirAll(filepath.Dir(storagePath), 0o755); err != nil {
		writeLogoError(c, http.StatusInternalServerError, "创建 Logo 存储目录失败")
		return
	}
	if err := os.WriteFile(storagePath, content, 0o644); err != nil {
		writeLogoError(c, http.StatusInternalServerError, "保存 Logo 文件失败")
		return
	}
	removeLegacyLogoFile(storagePath)

	logoURL := fmt.Sprintf("%s?v=%d", logoPublicPath, time.Now().UnixNano())
	if err := model.UpdateOption("Logo", logoURL); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"url": logoURL,
		},
	})
}

func GetLogo(c *gin.Context) {
	content, err := readStoredLogo()
	if err != nil || len(content) == 0 {
		c.Redirect(http.StatusFound, "/logo.png")
		return
	}

	contentType := http.DetectContentType(content)
	if !isAllowedLogoMIME(contentType) {
		c.Redirect(http.StatusFound, "/logo.png")
		return
	}

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, contentType, content)
}
