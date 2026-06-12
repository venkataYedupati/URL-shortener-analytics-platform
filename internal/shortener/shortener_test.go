package shortener

import "testing"

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode(7)
	if err != nil {
		t.Fatalf("GenerateCode returned error: %v", err)
	}
	if len(code) != 7 {
		t.Fatalf("expected length 7, got %d", len(code))
	}
	if err := ValidateCustomCode(code); err != nil {
		t.Fatalf("generated code should be a valid custom code: %v", err)
	}
}

func TestValidateTargetURL(t *testing.T) {
	valid := []string{"https://example.com/path", "http://go.example.test/demo"}
	for _, raw := range valid {
		if err := ValidateTargetURL(raw); err != nil {
			t.Fatalf("%s should be valid: %v", raw, err)
		}
	}

	invalid := []string{
		"ftp://example.com",
		"/relative/path",
		"https://",
		"http://localhost:8080/demo",
		"http://127.0.0.1/demo",
		"http://10.1.2.3/demo",
		"http://172.16.0.1/demo",
		"http://192.168.1.2/demo",
		"http://[::1]/demo",
	}
	for _, raw := range invalid {
		if err := ValidateTargetURL(raw); err == nil {
			t.Fatalf("%s should be invalid", raw)
		}
	}
}

func TestDeviceFromUserAgent(t *testing.T) {
	tests := map[string]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS)": "Mobile",
		"Mozilla/5.0 (iPad; CPU OS)":          "Tablet",
		"Googlebot/2.1":                       "Bot",
		"Mozilla/5.0 (Macintosh; Intel Mac)":  "Desktop",
		"":                                    "Unknown",
	}

	for ua, expected := range tests {
		if got := DeviceFromUserAgent(ua); got != expected {
			t.Fatalf("DeviceFromUserAgent(%q) = %q, want %q", ua, got, expected)
		}
	}
}

func TestReferrerDomain(t *testing.T) {
	tests := map[string]string{
		"":                                "Direct",
		"https://www.google.com/search?q": "google.com",
		"https://news.ycombinator.com/":   "news.ycombinator.com",
		"not a url":                       "Unknown",
	}

	for raw, expected := range tests {
		if got := ReferrerDomain(raw); got != expected {
			t.Fatalf("ReferrerDomain(%q) = %q, want %q", raw, got, expected)
		}
	}
}
