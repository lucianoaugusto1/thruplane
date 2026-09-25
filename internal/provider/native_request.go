package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func decodeChatRequest(body []byte) (chatRequest, error) {
	var request chatRequest
	if trimmed := bytes.TrimSpace(body); len(trimmed) == 0 || trimmed[0] != '{' {
		return chatRequest{}, &RequestError{Code: "invalid_json", Message: "The request body must be a JSON object."}
	}
	if err := decodeStrictJSON(body, &request); err != nil {
		if isUnknownJSONField(err) {
			return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters do not support " + strings.TrimPrefix(err.Error(), "json: unknown field ") + "."}
		}
		return chatRequest{}, &RequestError{Code: "invalid_json", Message: "The request body must be a JSON object with supported field types."}
	}
	if request.N != nil && *request.N != 1 {
		return chatRequest{}, &RequestError{Code: "unsupported_n", Message: "Native adapters support n=1 only."}
	}
	if request.MaxTokens != nil && request.MaxCompletionTokens != nil {
		return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters accept max_tokens or max_completion_tokens, not both."}
	}
	if (request.MaxTokens != nil && *request.MaxTokens <= 0) || (request.MaxCompletionTokens != nil && *request.MaxCompletionTokens <= 0) {
		return chatRequest{}, &RequestError{Code: "invalid_max_tokens", Message: "The output token limit must be greater than zero."}
	}
	if hasJSONValue(request.Modalities) {
		var modalities []string
		if err := json.Unmarshal(request.Modalities, &modalities); err != nil || len(modalities) != 1 || modalities[0] != "text" {
			return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters support text-only output modalities."}
		}
	}
	if hasJSONValue(request.ResponseFormat) {
		var format struct {
			Type string `json:"type"`
		}
		if err := decodeStrictJSON(request.ResponseFormat, &format); err != nil || format.Type != "text" {
			return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters support response_format type text only."}
		}
	}
	return request, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected extra JSON value")
	}
	return nil
}

func isUnknownJSONField(err error) bool {
	return strings.HasPrefix(err.Error(), "json: unknown field ")
}

func hasJSONValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func textContent(raw json.RawMessage) (string, error) {
	if !hasJSONValue(raw) {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []struct {
		Type string  `json:"type"`
		Text *string `json:"text"`
	}
	if err := decodeStrictJSON(raw, &parts); err != nil {
		return "", &RequestError{Code: "unsupported_content", Message: "Native provider adapters currently support text message content only."}
	}
	var builder strings.Builder
	for _, part := range parts {
		if (part.Type != "text" && part.Type != "input_text") || part.Text == nil {
			return "", &RequestError{Code: "unsupported_content", Message: "Native provider adapters currently support text message content only."}
		}
		builder.WriteString(*part.Text)
	}
	return builder.String(), nil
}

func stopSequences(raw json.RawMessage) ([]string, error) {
	if !hasJSONValue(raw) {
		return nil, nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return []string{one}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, &RequestError{Code: "invalid_stop", Message: "The stop field must be a string or an array of strings."}
	}
	return many, nil
}

func maxOutputTokens(request chatRequest, fallback int) int {
	if request.MaxCompletionTokens != nil {
		return *request.MaxCompletionTokens
	}
	if request.MaxTokens != nil {
		return *request.MaxTokens
	}
	return fallback
}
