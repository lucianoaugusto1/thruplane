package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lucianoaugusto1/thruplane/internal/config"
)

type conformanceFixture struct {
	Provider string            `json:"provider"`
	Model    string            `json:"model"`
	Cases    []conformanceCase `json:"cases"`
}

type conformanceCase struct {
	Name          string              `json:"name"`
	Request       json.RawMessage     `json:"request"`
	WantRequest   conformanceRequest  `json:"want_request"`
	Upstream      conformanceUpstream `json:"upstream"`
	WantResponse  conformanceResponse `json:"want_response"`
	WantErrorCode string              `json:"want_error_code"`
}

type conformanceRequest struct {
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	RawQuery         string            `json:"raw_query"`
	Headers          map[string]string `json:"headers"`
	HeaderPrefixes   map[string]string `json:"header_prefixes"`
	JSON             json.RawMessage   `json:"json"`
	BodySHA256Header bool              `json:"body_sha256_header"`
}

type conformanceUpstream struct {
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers"`
	BodyJSON json.RawMessage   `json:"body_json"`
	BodyText string            `json:"body_text"`
}

type conformanceResponse struct {
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	JSONSubset json.RawMessage   `json:"json_subset"`
	BodyExact  string            `json:"body_exact"`
	Contains   []string          `json:"contains"`
}

type compatibleConformanceFixture struct {
	Model     string                          `json:"model"`
	Providers []compatibleConformanceProvider `json:"providers"`
	Cases     []compatibleConformanceCase     `json:"cases"`
}

type compatibleConformanceProvider struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	APIVersion string            `json:"api_version"`
	Path       string            `json:"path"`
	RawQuery   string            `json:"raw_query"`
	Headers    map[string]string `json:"headers"`
}

type compatibleConformanceCase struct {
	Name           string              `json:"name"`
	Request        json.RawMessage     `json:"request"`
	WantJSON       json.RawMessage     `json:"want_json"`
	WantOllamaJSON json.RawMessage     `json:"want_ollama_json"`
	Upstream       conformanceUpstream `json:"upstream"`
	WantResponse   conformanceResponse `json:"want_response"`
	WantErrorCode  string              `json:"want_error_code"`
}

type capturedConformanceRequest struct {
	Method   string
	Path     string
	RawQuery string
	Headers  http.Header
	Body     []byte
}

func TestNativeProviderConformanceFixtures(t *testing.T) {
	t.Parallel()
	for _, providerType := range []string{"anthropic", "gemini", "vertex", "bedrock"} {
		providerType := providerType
		t.Run(providerType, func(t *testing.T) {
			t.Parallel()
			fixture := loadConformanceFixture(t, providerType)
			if fixture.Provider != providerType {
				t.Fatalf("fixture provider = %q, want %q", fixture.Provider, providerType)
			}
			if fixture.Model == "" || len(fixture.Cases) == 0 {
				t.Fatal("fixture requires a model and at least one case")
			}
			names := make(map[string]bool, len(fixture.Cases))
			for _, testCase := range fixture.Cases {
				if testCase.Name == "" || names[testCase.Name] {
					t.Fatalf("case names must be non-empty and unique: %q", testCase.Name)
				}
				names[testCase.Name] = true
				t.Run(testCase.Name, func(t *testing.T) {
					runConformanceCase(t, fixture, testCase)
				})
			}
			streamCase := "stream-text-tools"
			if providerType == "bedrock" {
				streamCase = "streaming-unsupported"
			}
			if !names[streamCase] {
				t.Errorf("fixture is missing required case %q", streamCase)
			}
		})
	}
}

func TestCompatibleProviderConformanceFixtures(t *testing.T) {
	t.Parallel()
	fixture := loadCompatibleConformanceFixture(t)
	if fixture.Model == "" || len(fixture.Providers) == 0 || len(fixture.Cases) == 0 {
		t.Fatal("compatible fixture requires a model, providers, and cases")
	}

	wantTypes := map[string]bool{
		"openai": true, "azure-openai": true, "ollama": true,
		"openai-compatible": true, "thruplane-inference": true, "xai": true,
	}
	seenTypes := make(map[string]bool, len(fixture.Providers))
	for _, fixtureProvider := range fixture.Providers {
		if fixtureProvider.Name == "" || !wantTypes[fixtureProvider.Type] || seenTypes[fixtureProvider.Type] {
			t.Fatalf("invalid or duplicate compatible provider fixture: %#v", fixtureProvider)
		}
		seenTypes[fixtureProvider.Type] = true
		fixtureProvider := fixtureProvider
		t.Run(fixtureProvider.Name, func(t *testing.T) {
			t.Parallel()
			for _, testCase := range fixture.Cases {
				testCase := testCase
				t.Run(testCase.Name, func(t *testing.T) {
					runCompatibleConformanceCase(t, fixture.Model, fixtureProvider, testCase)
				})
			}
		})
	}
	if !reflect.DeepEqual(seenTypes, wantTypes) {
		t.Fatalf("compatible provider types = %#v, want %#v", seenTypes, wantTypes)
	}
}

func loadCompatibleConformanceFixture(t *testing.T) compatibleConformanceFixture {
	t.Helper()
	path := filepath.Join("testdata", "conformance", "compatible.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var fixture compatibleConformanceFixture
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		t.Fatalf("fixture %s must contain exactly one JSON value", path)
	}
	return fixture
}

func runCompatibleConformanceCase(t *testing.T, model string, fixtureProvider compatibleConformanceProvider, testCase compatibleConformanceCase) {
	t.Helper()
	var calls atomic.Int32
	captured := make(chan capturedConformanceRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		captured <- capturedConformanceRequest{
			Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery,
			Headers: r.Header.Clone(), Body: body,
		}
		for name, value := range testCase.Upstream.Headers {
			w.Header().Set(name, value)
		}
		status := testCase.Upstream.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if len(testCase.Upstream.BodyJSON) > 0 {
			_, _ = w.Write(testCase.Upstream.BodyJSON)
		} else {
			_, _ = io.WriteString(w, testCase.Upstream.BodyText)
		}
	}))
	t.Cleanup(server.Close)

	providerConfig := config.ProviderConfig{
		Type: fixtureProvider.Type, BaseURL: server.URL,
		APIKey: "fixture-compatible-key", APIVersion: fixtureProvider.APIVersion,
	}
	providerAdapter, err := newAdapter(providerConfig)
	if err != nil {
		t.Fatalf("newAdapter() error = %v", err)
	}
	client := &Client{httpClient: server.Client(), adapter: providerAdapter}
	response, err := client.Do(context.Background(), testCase.Request, model)
	if testCase.WantErrorCode != "" {
		var requestError *RequestError
		if !errors.As(err, &requestError) || requestError.Code != testCase.WantErrorCode {
			t.Fatalf("Do() error = %v, want code %q", err, testCase.WantErrorCode)
		}
		if calls.Load() != 0 {
			t.Fatalf("upstream calls = %d, want 0", calls.Load())
		}
		return
	}
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}

	gotRequest := <-captured
	wantJSON := testCase.WantJSON
	if fixtureProvider.Type == "ollama" && len(testCase.WantOllamaJSON) > 0 {
		wantJSON = testCase.WantOllamaJSON
	}
	assertConformanceRequest(t, gotRequest, conformanceRequest{
		Path: fixtureProvider.Path, RawQuery: fixtureProvider.RawQuery,
		Headers: fixtureProvider.Headers, JSON: wantJSON,
	})
	if fixtureProvider.Type == "azure-openai" && gotRequest.Headers.Get("Authorization") != "" {
		t.Error("Azure request unexpectedly contains Authorization")
	}
	if fixtureProvider.Type != "azure-openai" && gotRequest.Headers.Get("api-key") != "" {
		t.Error("bearer-auth request unexpectedly contains api-key")
	}
	assertConformanceResponse(t, response, testCase.WantResponse)
}

func loadConformanceFixture(t *testing.T, providerType string) conformanceFixture {
	t.Helper()
	path := filepath.Join("testdata", "conformance", providerType+".json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var fixture conformanceFixture
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		t.Fatalf("fixture %s must contain exactly one JSON value", path)
	}
	return fixture
}

func runConformanceCase(t *testing.T, fixture conformanceFixture, testCase conformanceCase) {
	t.Helper()
	var calls atomic.Int32
	captured := make(chan capturedConformanceRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		captured <- capturedConformanceRequest{
			Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery,
			Headers: r.Header.Clone(), Body: body,
		}
		for name, value := range testCase.Upstream.Headers {
			w.Header().Set(name, value)
		}
		status := testCase.Upstream.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if len(testCase.Upstream.BodyJSON) > 0 {
			_, _ = w.Write(testCase.Upstream.BodyJSON)
		} else {
			_, _ = io.WriteString(w, testCase.Upstream.BodyText)
		}
	}))
	t.Cleanup(server.Close)

	client := newConformanceClient(t, fixture.Provider, server)
	response, err := client.Do(context.Background(), testCase.Request, fixture.Model)
	if testCase.WantErrorCode != "" {
		var requestError *RequestError
		if !errors.As(err, &requestError) || requestError.Code != testCase.WantErrorCode {
			t.Fatalf("Do() error = %v, want code %q", err, testCase.WantErrorCode)
		}
		if calls.Load() != 0 {
			t.Fatalf("upstream calls = %d, want 0", calls.Load())
		}
		return
	}
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}
	gotRequest := <-captured
	assertConformanceRequest(t, gotRequest, testCase.WantRequest)
	assertConformanceResponse(t, response, testCase.WantResponse)
}

func newConformanceClient(t *testing.T, providerType string, server *httptest.Server) *Client {
	t.Helper()
	cfg := config.ProviderConfig{Type: providerType, BaseURL: server.URL}
	switch providerType {
	case "anthropic":
		cfg.APIKey = "fixture-anthropic-key"
		cfg.APIVersion = "2023-06-01"
	case "gemini":
		cfg.APIKey = "fixture-gemini-key"
	case "vertex":
		cfg.AccessToken = "fixture-google-token"
		cfg.Project = "fixture-project"
		cfg.Location = "us-central1"
	case "bedrock":
		cfg.Region = "us-east-1"
		cfg.AccessKeyID = "FIXTUREACCESSKEY"
		cfg.SecretAccessKey = "fixture-secret-key"
		cfg.SessionToken = "fixture-session-token"
	}
	providerAdapter, err := newAdapter(cfg)
	if err != nil {
		t.Fatalf("newAdapter() error = %v", err)
	}
	if bedrock, ok := providerAdapter.(*bedrockAdapter); ok {
		bedrock.now = func() time.Time {
			return time.Date(2026, time.September, 24, 12, 0, 0, 0, time.UTC)
		}
	}
	httpClient := server.Client()
	httpClient.Timeout = 2 * time.Second
	return &Client{httpClient: httpClient, adapter: providerAdapter}
}

func assertConformanceRequest(t *testing.T, got capturedConformanceRequest, want conformanceRequest) {
	t.Helper()
	method := want.Method
	if method == "" {
		method = http.MethodPost
	}
	if got.Method != method || got.Path != want.Path || got.RawQuery != want.RawQuery {
		t.Errorf("upstream request = %s %s?%s, want %s %s?%s", got.Method, got.Path, got.RawQuery, method, want.Path, want.RawQuery)
	}
	for name, value := range want.Headers {
		if actual := got.Headers.Get(name); actual != value {
			t.Errorf("header %s = %q, want %q", name, actual, value)
		}
	}
	for name, prefix := range want.HeaderPrefixes {
		if actual := got.Headers.Get(name); !strings.HasPrefix(actual, prefix) {
			t.Errorf("header %s = %q, want prefix %q", name, actual, prefix)
		}
	}
	if want.BodySHA256Header && got.Headers.Get("X-Amz-Content-Sha256") != sha256Hex(got.Body) {
		t.Errorf("X-Amz-Content-Sha256 does not match request body")
	}
	if len(want.JSON) > 0 {
		assertSemanticJSONEqual(t, got.Body, want.JSON, "upstream request body")
	}
}

func assertConformanceResponse(t *testing.T, response *http.Response, want conformanceResponse) {
	t.Helper()
	status := want.Status
	if status == 0 {
		status = http.StatusOK
	}
	if response.StatusCode != status {
		t.Errorf("response status = %d, want %d", response.StatusCode, status)
	}
	for name, value := range want.Headers {
		if actual := response.Header.Get(name); actual != value {
			t.Errorf("response header %s = %q, want %q", name, actual, value)
		}
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if len(want.JSONSubset) > 0 {
		assertJSONSubset(t, body, want.JSONSubset)
	}
	if want.BodyExact != "" && string(body) != want.BodyExact {
		t.Errorf("response body = %q, want %q", body, want.BodyExact)
	}
	for _, fragment := range want.Contains {
		if !bytes.Contains(body, []byte(fragment)) {
			t.Errorf("response body does not contain %q: %s", fragment, body)
		}
	}
}

func assertSemanticJSONEqual(t *testing.T, got, want []byte, label string) {
	t.Helper()
	actual := decodeFixtureJSON(t, got, label)
	expected := decodeFixtureJSON(t, want, "fixture "+label)
	if !reflect.DeepEqual(actual, expected) {
		actualJSON, _ := json.Marshal(actual)
		expectedJSON, _ := json.Marshal(expected)
		t.Errorf("%s = %s, want %s", label, actualJSON, expectedJSON)
	}
}

func assertJSONSubset(t *testing.T, got, want []byte) {
	t.Helper()
	actual := decodeFixtureJSON(t, got, "normalized response")
	expected := decodeFixtureJSON(t, want, "fixture response subset")
	if err := fixtureSubset(actual, expected, "$"); err != nil {
		t.Error(err)
	}
}

func decodeFixtureJSON(t *testing.T, data []byte, label string) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode %s: %v; body=%s", label, err, data)
	}
	return value
}

func fixtureSubset(actual, expected any, path string) error {
	switch expectedValue := expected.(type) {
	case map[string]any:
		actualValue, ok := actual.(map[string]any)
		if !ok {
			return fmt.Errorf("%s = %T, want object", path, actual)
		}
		for key, child := range expectedValue {
			got, exists := actualValue[key]
			if !exists {
				return fmt.Errorf("%s.%s is missing", path, key)
			}
			if err := fixtureSubset(got, child, path+"."+key); err != nil {
				return err
			}
		}
		return nil
	case []any:
		actualValue, ok := actual.([]any)
		if !ok || len(actualValue) != len(expectedValue) {
			return fmt.Errorf("%s = %#v, want array length %d", path, actual, len(expectedValue))
		}
		for index := range expectedValue {
			if err := fixtureSubset(actualValue[index], expectedValue[index], fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		return nil
	default:
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("%s = %#v, want %#v", path, actual, expected)
		}
		return nil
	}
}
