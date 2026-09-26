package httpapi

import (
	"regexp"
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
		`fetch("/playground/config.json"`,
		`/playground/api/credentials/chat/completions`,
		`AbortController`,
		`getReader()`,
		`image_url`,
		`file_data`,
		`input_audio`,
		`NEXOROUTE_API_KEY`,
		`PROVIDER_API_KEY`,
		`GOOGLE_ACCESS_TOKEN`,
		`AWS_ACCESS_KEY_ID`,
		`AWS_SECRET_ACCESS_KEY`,
		`credentialVerifiedRevision`,
		`buildCredentialEnvelope`,
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

func TestPlaygroundInterfaceContract(t *testing.T) {
	htmlBytes, err := playgroundAssets.ReadFile("playground/index.html")
	if err != nil {
		t.Fatal(err)
	}
	cssBytes, err := playgroundAssets.ReadFile("playground/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	scriptBytes, err := playgroundAssets.ReadFile("playground/app.js")
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlBytes)
	css := string(cssBytes)
	script := string(scriptBytes)

	for _, required := range []string{
		`<html lang="en">`,
		`name="viewport"`,
		`<header class="app-header">`,
		`<main class="workspace">`,
		`aria-live="polite"`,
		`aria-label="Attach image, PDF, WAV, or MP3"`,
		`<label for="api-key">`,
		`<label for="model">`,
		`for="prompt"`,
		`id="credential-mode-section" hidden`,
		`id="source-credential"`,
		`id="provider-type"`,
		`value="nexoroute-inference"`,
		`value="xai"`,
		`id="provider-secret-access-key" type="password"`,
		`id="test-credential"`,
		`Test connection makes a real request.`,
	} {
		if !strings.Contains(html, required) {
			t.Errorf("index.html missing %q", required)
		}
	}
	for _, forbidden := range []string{"<style", "onclick=", "onchange=", "onload="} {
		if strings.Contains(html, forbidden) {
			t.Errorf("index.html contains forbidden %q", forbidden)
		}
	}

	idPattern := regexp.MustCompile(`byID\("([a-z0-9-]+)"\)`)
	for _, match := range idPattern.FindAllStringSubmatch(script, -1) {
		if !strings.Contains(html, `id="`+match[1]+`"`) {
			t.Errorf("index.html missing app.js element id %q", match[1])
		}
	}
	for _, required := range []string{
		":focus-visible",
		"@media (max-width:",
		"@media (prefers-reduced-motion: reduce)",
		".composer-tools input.file-input",
		"grid-template-columns:",
		"min-height: 100dvh",
		".source-switch",
		".credential-card",
		".cost-warning",
		`:not([type="radio"])`,
	} {
		if !strings.Contains(css, required) {
			t.Errorf("styles.css missing %q", required)
		}
	}
}
