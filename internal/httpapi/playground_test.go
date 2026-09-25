package httpapi

import (
	"strings"
	"testing"
)

func TestPlaygroundClientContract(t *testing.T) {
	scriptBytes, err := playgroundAssets.ReadFile("playground/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptBytes)
	for _, required := range []string{
		`fetch("/v1/models"`,
		`/v1/chat/completions`,
		`AbortController`,
		`getReader()`,
		`image_url`,
		`file_data`,
		`input_audio`,
		`NEXOROUTE_API_KEY`,
		`X-NexoRoute-Provider`,
		`X-NexoRoute-Attempts`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("app.js missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"localStorage",
		"sessionStorage",
		"document.cookie",
		"innerHTML",
		"http://",
		"https://",
	} {
		if strings.Contains(script, forbidden) {
			t.Errorf("app.js contains forbidden %q", forbidden)
		}
	}
}
