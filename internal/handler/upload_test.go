package handler

import (
	"mime/multipart"
	"net/textproto"
	"testing"
)

func TestResolveAllowedUploadExtensionRejectsHTML(t *testing.T) {
	file := &multipart.FileHeader{
		Filename: "payload.html",
		Header:   textproto.MIMEHeader{"Content-Type": []string{"image/png"}},
	}
	if _, err := resolveAllowedUploadExtension(file, "image/png", imageUploadRule); err == nil {
		t.Fatal("expected .html upload to be rejected")
	}
}

func TestResolveAllowedUploadExtensionAcceptsKnownImage(t *testing.T) {
	file := &multipart.FileHeader{
		Filename: "avatar.PNG",
		Header:   textproto.MIMEHeader{"Content-Type": []string{"image/png"}},
	}
	ext, err := resolveAllowedUploadExtension(file, "image/png", imageUploadRule)
	if err != nil {
		t.Fatalf("expected image upload to be accepted: %v", err)
	}
	if ext != ".png" {
		t.Fatalf("unexpected extension: %s", ext)
	}
}
