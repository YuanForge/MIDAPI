package sanitize

import "testing"

func TestRedactStringHeaders(t *testing.T) {
	got := RedactStringHeaders(map[string]string{
		"Authorization": "Bearer sk-1234567890",
		"Content-Type":  "application/json",
	})
	if got["Authorization"] == "Bearer sk-1234567890" {
		t.Fatalf("Authorization was not redacted")
	}
	if got["Content-Type"] != "application/json" {
		t.Fatalf("non-sensitive header changed: %q", got["Content-Type"])
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://example.com/v1?key=abcdef123456&model=x")
	if got == "https://example.com/v1?key=abcdef123456&model=x" {
		t.Fatalf("sensitive query was not redacted")
	}
	if got != "https://example.com/v1?key=abcd%2A%2A%2A3456&model=x" {
		t.Fatalf("unexpected redacted URL: %s", got)
	}
}
