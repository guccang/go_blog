package http

import (
	"encoding/json"
	h "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondLoginSuccessUsesJSONForXHR(t *testing.T) {
	request := httptest.NewRequest(h.MethodPost, "/login", nil)
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	recorder := httptest.NewRecorder()

	respondLoginSuccess(recorder, request)

	if recorder.Code != h.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, h.StatusOK)
	}
	if location := recorder.Header().Get("Location"); location != "" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["redirect"] != "/main" {
		t.Fatalf("redirect = %q, want /main", response["redirect"])
	}
}

func TestRespondLoginSuccessKeepsRedirectForTraditionalClient(t *testing.T) {
	request := httptest.NewRequest(h.MethodPost, "/login", nil)
	recorder := httptest.NewRecorder()

	respondLoginSuccess(recorder, request)

	if recorder.Code != h.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, h.StatusFound)
	}
	if location := recorder.Header().Get("Location"); location != "/main" {
		t.Fatalf("location = %q, want /main", location)
	}
}

func TestNeedsPIProviderMigration(t *testing.T) {
	tests := []struct {
		name    string
		configs map[string]string
		want    bool
	}{
		{name: "current config", configs: map[string]string{"pi_providers": `{}`}, want: false},
		{name: "missing providers", configs: map[string]string{}, want: true},
		{name: "legacy deepseek", configs: map[string]string{"pi_providers": `{}`, "deepseek_model": "legacy"}, want: true},
		{name: "legacy provider key", configs: map[string]string{"pi_providers": `{}`, "pi_provider_old": "legacy"}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := needsPIProviderMigration(test.configs); got != test.want {
				t.Fatalf("needsPIProviderMigration() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestBuildConfigContentWithCommentsSortsExtraKeys(t *testing.T) {
	configs := map[string]string{
		"port":     "8080",
		"z_custom": "z",
		"a_custom": "a",
	}
	first := buildConfigContentWithComments(configs, nil)
	for index := 0; index < 50; index++ {
		if got := buildConfigContentWithComments(configs, nil); got != first {
			t.Fatalf("config output changed between builds")
		}
	}
	if strings.Index(first, "a_custom=a") > strings.Index(first, "z_custom=z") {
		t.Fatalf("extra config keys are not sorted: %s", first)
	}
}
