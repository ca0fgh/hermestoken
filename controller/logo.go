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
	logoUploadFormField      = "logo"
	logoPublicPath           = "/api/logo"
	maxLogoUploadBytes int64 = 5 << 20
)

var (
	logoStoragePath = filepath.Join("data", "system-assets", "logo-upload.bin")
	allowedLogoMIMEs = map[string]struct{}{
		"image/gif":  {},
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	}
)

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

	if err := os.MkdirAll(filepath.Dir(logoStoragePath), 0o755); err != nil {
		writeLogoError(c, http.StatusInternalServerError, "创建 Logo 存储目录失败")
		return
	}
	if err := os.WriteFile(logoStoragePath, content, 0o644); err != nil {
		writeLogoError(c, http.StatusInternalServerError, "保存 Logo 文件失败")
		return
	}

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
	content, err := os.ReadFile(logoStoragePath)
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
