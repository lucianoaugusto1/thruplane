package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func InspectChatRequest(body []byte) (Requirements, error) {
	var request struct {
		Messages []struct {
			Role      string            `json:"role"`
			Content   json.RawMessage   `json:"content"`
			ToolCalls []json.RawMessage `json:"tool_calls"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Strict bool `json:"strict"`
			} `json:"function"`
		} `json:"tools"`
		ParallelToolCalls *bool `json:"parallel_tool_calls"`
		ResponseFormat    struct {
			Type string `json:"type"`
		} `json:"response_format"`
		Modalities []string `json:"modalities"`
		Stream     bool     `json:"stream"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return Requirements{}, fmt.Errorf("inspect chat request: %w", err)
	}

	requirements := Requirements{Operation: "chat", Streaming: request.Stream}
	inputs := make(map[string]bool)
	for _, message := range request.Messages {
		if message.Role == "tool" || len(message.ToolCalls) > 0 {
			requirements.Tools = true
		}
		trimmed := bytes.TrimSpace(message.Content)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
			continue
		}
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			inputs["text"] = true
			continue
		}
		var parts []struct {
			Type     string `json:"type"`
			ImageURL struct {
				URL string `json:"url"`
			} `json:"image_url"`
		}
		if err := json.Unmarshal(trimmed, &parts); err != nil {
			requirements.UnknownContent = true
			continue
		}
		for _, part := range parts {
			switch part.Type {
			case "text", "input_text":
				inputs["text"] = true
			case "image_url", "input_image", "image":
				inputs["image"] = true
				if part.Type == "image_url" && !strings.HasPrefix(part.ImageURL.URL, "data:") {
					requirements.RemoteImage = true
				}
			case "input_audio", "audio":
				inputs["audio"] = true
			case "file", "input_file", "document":
				inputs["document"] = true
			default:
				requirements.UnknownContent = true
			}
		}
	}
	requirements.InputModalities = orderedModalities(inputs)

	outputs := make(map[string]bool)
	if len(request.Modalities) == 0 {
		outputs["text"] = true
	} else {
		for _, modality := range request.Modalities {
			switch modality {
			case "text", "audio", "image":
				outputs[modality] = true
			default:
				requirements.UnknownContent = true
			}
		}
	}
	requirements.OutputModalities = orderedModalities(outputs)

	if len(request.Tools) > 0 {
		requirements.Tools = true
		for _, tool := range request.Tools {
			if tool.Type != "function" {
				requirements.UnknownContent = true
			}
			if tool.Function.Strict {
				requirements.StrictTools = true
			}
		}
	}
	requirements.ParallelTools = request.ParallelToolCalls != nil && *request.ParallelToolCalls
	requirements.DisableParallelTools = request.ParallelToolCalls != nil && !*request.ParallelToolCalls
	requirements.StructuredOutputs = request.ResponseFormat.Type != "" && request.ResponseFormat.Type != "text"
	return requirements, nil
}

func orderedModalities(set map[string]bool) []string {
	order := []string{"text", "image", "audio", "video", "document"}
	result := make([]string, 0, len(set))
	for _, modality := range order {
		if set[modality] {
			result = append(result, modality)
		}
	}
	return result
}
