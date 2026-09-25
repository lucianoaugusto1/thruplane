package provider

import (
	"encoding/json"
	"strings"
)

func nativeTools(request chatRequest) (nativeToolContract, error) {
	if hasJSONValue(request.LegacyFunctions) || hasJSONValue(request.LegacyFunctionCall) {
		return nativeToolContract{}, &RequestError{Code: "unsupported_legacy_functions", Message: "Native adapters support tools and tool_choice, not the legacy functions fields."}
	}
	contract := nativeToolContract{CallNames: make(map[string]string)}
	for _, tool := range request.Tools {
		if tool.Type != "function" {
			return nativeToolContract{}, &RequestError{Code: "unsupported_tool_type", Message: "Native adapters currently support function tools only."}
		}
		if tool.Function.Name == "" {
			return nativeToolContract{}, &RequestError{Code: "invalid_tool", Message: "Every function tool must have a name."}
		}
		if tool.Function.Strict != nil && *tool.Function.Strict {
			return nativeToolContract{}, &RequestError{Code: "unsupported_tool_option", Message: "Native adapters do not yet provide portable strict function schemas."}
		}
		if len(tool.Function.Parameters) == 0 {
			tool.Function.Parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil || schema == nil {
			return nativeToolContract{}, &RequestError{Code: "invalid_tool_schema", Message: "Function parameters must be a JSON Schema object."}
		}
		contract.Definitions = append(contract.Definitions, tool)
	}

	for _, message := range request.Messages {
		if message.Role != "tool" && message.ToolCallID != "" {
			return nativeToolContract{}, &RequestError{Code: "unsupported_field", Message: "tool_call_id is supported only on tool messages."}
		}
		for _, call := range message.ToolCalls {
			if call.Type != "function" {
				return nativeToolContract{}, &RequestError{Code: "unsupported_tool_type", Message: "Native adapters support function tool calls only."}
			}
			if message.Role != "assistant" || call.ID == "" || call.Function.Name == "" {
				return nativeToolContract{}, &RequestError{Code: "invalid_tool_call", Message: "Assistant tool calls require an id and function name."}
			}
			if _, err := toolArguments(call.Function.Arguments); err != nil {
				return nativeToolContract{}, err
			}
			contract.CallNames[call.ID] = call.Function.Name
		}
	}
	for _, message := range request.Messages {
		if message.Role == "tool" && (message.ToolCallID == "" || contract.CallNames[message.ToolCallID] == "") {
			return nativeToolContract{}, &RequestError{Code: "unmatched_tool_result", Message: "Every tool result must match an earlier assistant tool call."}
		}
	}

	choice, err := parseNativeToolChoice(request.ToolChoice)
	if err != nil {
		return nativeToolContract{}, err
	}
	if (choice.Mode == "required" || choice.Mode == "named") && len(contract.Definitions) == 0 {
		return nativeToolContract{}, &RequestError{Code: "unsupported_tool_choice", Message: "tool_choice requires at least one function tool."}
	}
	if choice.Mode == "named" {
		found := false
		for _, definition := range contract.Definitions {
			if definition.Function.Name == choice.Name {
				found = true
				break
			}
		}
		if !found {
			return nativeToolContract{}, &RequestError{Code: "unsupported_tool_choice", Message: "The named tool_choice must reference a configured function tool."}
		}
	}
	if request.ParallelToolCalls != nil && !*request.ParallelToolCalls {
		choice.DisableParallel = true
	}
	contract.Choice = choice
	return contract, nil
}

func parseNativeToolChoice(raw json.RawMessage) (nativeToolChoice, error) {
	if !hasJSONValue(raw) {
		return nativeToolChoice{}, nil
	}
	var mode string
	if err := json.Unmarshal(raw, &mode); err == nil {
		switch mode {
		case "auto", "none", "required":
			return nativeToolChoice{Mode: mode}, nil
		default:
			return nativeToolChoice{}, &RequestError{Code: "unsupported_tool_choice", Message: "tool_choice must be auto, none, required, or a named function."}
		}
	}
	var named struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := decodeStrictJSON(raw, &named); err != nil || named.Type != "function" || named.Function.Name == "" {
		return nativeToolChoice{}, &RequestError{Code: "unsupported_tool_choice", Message: "tool_choice must be auto, none, required, or a named function."}
	}
	return nativeToolChoice{Mode: "named", Name: named.Function.Name}, nil
}

func toolArguments(arguments string) (json.RawMessage, error) {
	if strings.TrimSpace(arguments) == "" {
		return json.RawMessage(`{}`), nil
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(arguments), &object); err != nil || object == nil {
		return nil, &RequestError{Code: "invalid_tool_arguments", Message: "Function arguments must encode a JSON object."}
	}
	return json.RawMessage(arguments), nil
}
