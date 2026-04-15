package controller

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	logoUploadFormField          = "logo"
	logoPublicPath               = "/api/logo"
	maxLogoUploadBytes     int64 = 5 << 20
	defaultLogoStoragePath       = "data/system-assets/logo-upload.bin"
	maxLogoThumbnailSize         = 512
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

func getRequestedLogoSize(c *gin.Context) int {
	sizeRaw := strings.TrimSpace(c.Query("size"))
	if sizeRaw == "" {
		return 0
	}

	size, err := strconv.Atoi(sizeRaw)
	if err != nil || size <= 0 {
		return 0
	}

	if size > maxLogoThumbnailSize {
		return maxLogoThumbnailSize
	}

	return size
}

func writeLogoCacheHeaders(c *gin.Context) {
	if strings.TrimSpace(c.Query("v")) != "" {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}

	c.Header("Cache-Control", "no-cache")
}

func decodeLogoImage(content []byte, contentType string) (image.Image, error) {
	reader := bytes.NewReader(content)

	if contentType == "image/webp" {
		return webp.Decode(reader)
	}

	img, _, err := image.Decode(reader)
	return img, err
}

func scaleLogoDimensions(width int, height int, maxEdge int) (int, int) {
	if width <= 0 || height <= 0 || maxEdge <= 0 {
		return width, height
	}

	if width <= maxEdge && height <= maxEdge {
		return width, height
	}

	if width >= height {
		scaledHeight := (height*maxEdge + width/2) / width
		if scaledHeight < 1 {
			scaledHeight = 1
		}
		return maxEdge, scaledHeight
	}

	scaledWidth := (width*maxEdge + height/2) / height
	if scaledWidth < 1 {
		scaledWidth = 1
	}
	return scaledWidth, maxEdge
}

func resizeLogoContent(content []byte, contentType string, maxEdge int) ([]byte, string, error) {
	img, err := decodeLogoImage(content, contentType)
	if err != nil {
		return nil, "", err
	}

	sourceBounds := img.Bounds()
	targetWidth, targetHeight := scaleLogoDimensions(
		sourceBounds.Dx(),
		sourceBounds.Dy(),
		maxEdge,
	)

	if targetWidth == sourceBounds.Dx() && targetHeight == sourceBounds.Dy() && contentType == "image/png" {
		return content, contentType, nil
	}

	dst := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, sourceBounds, draw.Over, nil)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, dst); err != nil {
		return nil, "", err
	}

	return buffer.Bytes(), "image/png", nil
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
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	contentType := http.DetectContentType(content)
	if !isAllowedLogoMIME(contentType) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	writeLogoCacheHeaders(c)

	if requestedSize := getRequestedLogoSize(c); requestedSize > 0 {
		resizedContent, resizedContentType, resizeErr := resizeLogoContent(
			content,
			contentType,
			requestedSize,
		)
		if resizeErr == nil {
			c.Data(http.StatusOK, resizedContentType, resizedContent)
			return
		}
	}

	c.Data(http.StatusOK, contentType, content)
}

func resolveLogoOptionValue() string {
	logo := strings.TrimSpace(common.Logo)
	if logo == "" || logo == "/logo.png" {
		return ""
	}

	if strings.HasPrefix(logo, logoPublicPath) {
		content, err := readStoredLogo()
		if err != nil || len(content) == 0 {
			return ""
		}

		contentType := http.DetectContentType(content)
		if !isAllowedLogoMIME(contentType) {
			return ""
		}
	}

	return logo
}
