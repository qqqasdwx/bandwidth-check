package kuma

import "testing"

func TestSanitizeKumaBodyMasksPushTokenURL(t *testing.T) {
	got := sanitizeKumaBody("http://kuma.example/api/push/secret-token?status=up")
	want := "http://kuma.example/api/push/***?status=up"
	if got != want {
		t.Fatalf("sanitizeKumaBody() = %q, want %q", got, want)
	}
}
