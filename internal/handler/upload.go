package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var uploadImageCategories = map[string]string{
	"reference":    "reference",
	"channel-icon": "channel-icons",
	"site-setting": "site-settings",
	"payment-qr":   "payment-qr",
}

var uploadVideoCategories = map[string]string{
	"reference-video": "reference-videos",
}

type uploadRule struct {
	maxSize         int64
	contentPrefixes []string
	allowedExts     map[string]bool
	sniffPrefixes   []string
	defaultExt      string
	emptyFileMsg    string
	tooLargeMsg     string
	invalidTypeMsg  string
	saveFailedMsg   string
}

var imageUploadRule = uploadRule{
	maxSize:         10 * 1024 * 1024,
	contentPrefixes: []string{"image/"},
	allowedExts: map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	},
	sniffPrefixes:  []string{"image/"},
	defaultExt:     ".png",
	emptyFileMsg:   "请选择要上传的图片",
	tooLargeMsg:    "图片不能超过 10MB",
	invalidTypeMsg: "仅支持上传图片文件",
	saveFailedMsg:  "保存图片失败",
}

var videoUploadRule = uploadRule{
	maxSize:         200 * 1024 * 1024,
	contentPrefixes: []string{"video/"},
	allowedExts: map[string]bool{
		".mp4": true, ".mov": true, ".webm": true,
	},
	defaultExt:     ".mp4",
	emptyFileMsg:   "请选择要上传的视频",
	tooLargeMsg:    "视频不能超过 200MB",
	invalidTypeMsg: "仅支持上传视频文件",
	saveFailedMsg:  "保存视频失败",
}

func hasAllowedContentType(contentType string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

func parseContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return strings.ToLower(mediaType)
}

func resolveAllowedUploadExtension(file *multipart.FileHeader, contentType string, rule uploadRule) (string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		extensions, _ := mime.ExtensionsByType(contentType)
		for _, candidate := range extensions {
			candidate = strings.ToLower(candidate)
			if rule.allowedExts[candidate] {
				ext = candidate
				break
			}
		}
	}
	if ext == "" {
		ext = rule.defaultExt
	}
	if !rule.allowedExts[ext] {
		return "", errors.New(rule.invalidTypeMsg)
	}
	return ext, nil
}

func sniffUploadContentType(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	header := make([]byte, 512)
	n, err := src.Read(header)
	if err != nil && n == 0 {
		return "", err
	}
	return http.DetectContentType(header[:n]), nil
}

func validateUploadedMedia(file *multipart.FileHeader, rule uploadRule) (string, error) {
	contentType := parseContentType(file.Header.Get("Content-Type"))
	if contentType == "" || !hasAllowedContentType(contentType, rule.contentPrefixes) {
		return "", errors.New(rule.invalidTypeMsg)
	}
	ext, err := resolveAllowedUploadExtension(file, contentType, rule)
	if err != nil {
		return "", err
	}
	detectedType, err := sniffUploadContentType(file)
	if err != nil {
		return "", errors.New(rule.invalidTypeMsg)
	}
	for _, prefix := range rule.sniffPrefixes {
		if strings.HasPrefix(detectedType, prefix) {
			return ext, nil
		}
	}
	if strings.HasPrefix(detectedType, "text/") || detectedType == "application/xml" || detectedType == "application/octet-stream" && strings.HasPrefix(contentType, "image/") {
		return "", errors.New(rule.invalidTypeMsg)
	}
	return ext, nil
}

func saveUploadedMedia(c *gin.Context, category string, rule uploadRule) {
	userID := c.MustGet("user_id").(int64)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": rule.emptyFileMsg})
		return
	}
	if file.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "上传文件不能为空"})
		return
	}
	if file.Size > rule.maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": rule.tooLargeMsg})
		return
	}

	ext, err := validateUploadedMedia(file, rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": rule.invalidTypeMsg})
		return
	}

	subdir := filepath.Join("uploads", category)
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败"})
		return
	}

	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成文件名失败"})
		return
	}
	filename := fmt.Sprintf("%d_%d_%s%s", userID, time.Now().Unix(), hex.EncodeToString(randomBytes), ext)
	fullPath := filepath.Join(subdir, filename)
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": rule.saveFailedMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url": requestBaseURL(c) + fmt.Sprintf("/uploads/%s/%s", category, filename),
	})
}

// UploadImage POST /upload/image
func UploadImage(c *gin.Context) {
	categoryKey := c.PostForm("category")
	category, ok := uploadImageCategories[categoryKey]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的上传分类"})
		return
	}
	saveUploadedMedia(c, category, imageUploadRule)
}

// UploadVideo POST /upload/video
func UploadVideo(c *gin.Context) {
	categoryKey := c.PostForm("category")
	category, ok := uploadVideoCategories[categoryKey]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的上传分类"})
		return
	}
	saveUploadedMedia(c, category, videoUploadRule)
}

// UploadReferenceImage POST /user/reference-images
func UploadReferenceImage(c *gin.Context) {
	saveUploadedMedia(c, uploadImageCategories["reference"], imageUploadRule)
}
