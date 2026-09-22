package catalog

import (
	"reflect"
	"testing"
)

func TestInspectChatRequestDetectsModalitiesAndFeatures(t *testing.T) {
	body := []byte(`{
  "messages": [{
    "role": "user",
    "content": [
      {"type":"text","text":"Compare these inputs"},
      {"type":"image_url","image_url":{"url":"data:image/png;base64,AA=="}},
      {"type":"input_audio","input_audio":{"data":"AA==","format":"wav"}},
      {"type":"file","file":{"file_id":"file-1"}}
    ]
  }],
  "tools": [{"type":"function","function":{"name":"save","strict":true}}],
  "parallel_tool_calls": true,
  "response_format": {"type":"json_schema","json_schema":{"name":"answer"}},
  "modalities": ["text", "audio"],
  "stream": true
}`)

	got, err := InspectChatRequest(body)
	if err != nil {
		t.Fatalf("InspectChatRequest() error = %v", err)
	}
	want := Requirements{
		Operation:         "chat",
		InputModalities:   []string{"text", "image", "audio", "document"},
		OutputModalities:  []string{"text", "audio"},
		Tools:             true,
		StrictTools:       true,
		ParallelTools:     true,
		Streaming:         true,
		StructuredOutputs: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InspectChatRequest() = %#v, want %#v", got, want)
	}
}

func TestInspectChatRequestMarksUnknownContent(t *testing.T) {
	got, err := InspectChatRequest([]byte(`{"messages":[{"role":"user","content":[{"type":"future_media"}]}]}`))
	if err != nil {
		t.Fatalf("InspectChatRequest() error = %v", err)
	}
	if !got.UnknownContent {
		t.Fatal("UnknownContent = false, want true")
	}
}

func TestInspectChatRequestRejectsInvalidJSON(t *testing.T) {
	if _, err := InspectChatRequest([]byte(`{`)); err == nil {
		t.Fatal("InspectChatRequest() error = nil")
	}
}

func TestInspectChatRequestDetectsToolHistoryAndDisabledParallelCalls(t *testing.T) {
	got, err := InspectChatRequest([]byte(`{
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"f","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"done"}
		],
		"parallel_tool_calls":false
	}`))
	if err != nil {
		t.Fatalf("InspectChatRequest() error = %v", err)
	}
	if !got.Tools || !got.DisableParallelTools || got.ParallelTools {
		t.Fatalf("InspectChatRequest() = %#v, want tool history and disabled parallel calls", got)
	}
}

func TestInspectChatRequestDetectsRemoteImageSource(t *testing.T) {
	got, err := InspectChatRequest([]byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://images.example.com/photo.png"}}]}]}`))
	if err != nil {
		t.Fatalf("InspectChatRequest() error = %v", err)
	}
	if !got.RemoteImage || !reflect.DeepEqual(got.InputModalities, []string{"image"}) {
		t.Fatalf("InspectChatRequest() = %#v, want remote image", got)
	}
}
