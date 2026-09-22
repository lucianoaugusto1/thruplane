package provider

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
)

type nativeContentPart struct {
	Kind     string
	Text     string
	MIMEType string
	Data     string
	URL      string
}

func parseNativeContent(raw json.RawMessage) ([]nativeContentPart, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		return []nativeContentPart{{Kind: "text", Text: text}}, nil
	}
	var input []struct {
		Type     string  `json:"type"`
		Text     *string `json:"text"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
		File struct {
			FileData string `json:"file_data"`
			FileID   string `json:"file_id"`
		} `json:"file"`
		InputAudio struct {
			Data   string `json:"data"`
			Format string `json:"format"`
		} `json:"input_audio"`
	}
	if err := json.Unmarshal(trimmed, &input); err != nil {
		return nil, unsupportedContent("Message content must be text or an array of supported content parts.")
	}
	parts := make([]nativeContentPart, 0, len(input))
	for _, item := range input {
		switch item.Type {
		case "text", "input_text":
			if item.Text == nil {
				return nil, unsupportedContent("Text content parts require a text field.")
			}
			parts = append(parts, nativeContentPart{Kind: "text", Text: *item.Text})
		case "image_url":
			image, err := parseImageSource(item.ImageURL.URL)
			if err != nil {
				return nil, err
			}
			parts = append(parts, image)
		case "file":
			if item.File.FileID != "" || item.File.FileData == "" {
				return nil, unsupportedContent("Native adapters require inline PDF file_data; provider file IDs are not portable.")
			}
			mime, data, err := parseDataURI(item.File.FileData)
			if err != nil {
				return nil, err
			}
			if mime != "application/pdf" {
				return nil, unsupportedContent("Native adapters support PDF file_data only.")
			}
			parts = append(parts, nativeContentPart{Kind: "document", MIMEType: mime, Data: data})
		case "input_audio":
			mime := ""
			switch item.InputAudio.Format {
			case "wav":
				mime = "audio/wav"
			case "mp3":
				mime = "audio/mp3"
			default:
				return nil, unsupportedContent("Native adapters support WAV and MP3 input_audio only.")
			}
			if err := validateBase64(item.InputAudio.Data); err != nil {
				return nil, err
			}
			parts = append(parts, nativeContentPart{Kind: "audio", MIMEType: mime, Data: item.InputAudio.Data})
		default:
			return nil, unsupportedContent("This content part type is not supported by native adapters.")
		}
	}
	return parts, nil
}

func parseImageSource(source string) (nativeContentPart, error) {
	if strings.HasPrefix(source, "data:") {
		mime, data, err := parseDataURI(source)
		if err != nil {
			return nativeContentPart{}, err
		}
		switch mime {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			return nativeContentPart{Kind: "image", MIMEType: mime, Data: data}, nil
		default:
			return nativeContentPart{}, unsupportedContent("This inline image MIME type is not supported by native adapters.")
		}
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nativeContentPart{}, unsupportedContent("Image URLs must be absolute HTTPS URLs or supported data URIs.")
	}
	return nativeContentPart{Kind: "image", URL: source}, nil
}

func parseDataURI(source string) (string, string, error) {
	meta, data, ok := strings.Cut(source, ",")
	if !ok || !strings.HasPrefix(meta, "data:") || !strings.HasSuffix(meta, ";base64") {
		return "", "", unsupportedContent("Inline media must use a base64 data URI.")
	}
	mime := strings.TrimSuffix(strings.TrimPrefix(meta, "data:"), ";base64")
	if err := validateBase64(data); err != nil {
		return "", "", err
	}
	return mime, data, nil
}

func validateBase64(data string) error {
	if data == "" {
		return &RequestError{Code: "invalid_media", Message: "Inline media must not be empty."}
	}
	if _, err := base64.StdEncoding.Strict().DecodeString(data); err != nil {
		return &RequestError{Code: "invalid_media", Message: "Inline media must contain valid base64 data."}
	}
	return nil
}

func unsupportedContent(message string) error {
	return &RequestError{Code: "unsupported_content", Message: message}
}
