package script

import "testing"

func TestReadUploadSourceRejectsLocalPath(t *testing.T) {
	if _, _, _, err := readUploadSource("/etc/passwd"); err == nil {
		t.Fatal("expected local path source to be rejected")
	}
}

func TestValidateRemoteUploadSourceRejectsPrivateAddress(t *testing.T) {
	for _, src := range []string{
		"http://127.0.0.1/file.png",
		"http://localhost/file.png",
		"http://169.254.169.254/latest/meta-data",
	} {
		if _, err := validateRemoteUploadSource(src); err == nil {
			t.Fatalf("expected private upload source %q to be rejected", src)
		}
	}
}
