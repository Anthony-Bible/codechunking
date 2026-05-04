package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmbeddingRequest_MarshalsExpectedShape(t *testing.T) {
	req := embeddingRequest{
		Input:          []string{"hello", "world"},
		Model:          "text-embedding-3-small",
		EncodingFormat: "float",
		Dimensions:     768,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"input":["hello","world"]`,
		`"model":"text-embedding-3-small"`,
		`"encoding_format":"float"`,
		`"dimensions":768`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected JSON to contain %q, got %s", want, got)
		}
	}
}

func TestEmbeddingRequest_OmitsZeroDimensions(t *testing.T) {
	req := embeddingRequest{
		Input: []string{"hello"},
		Model: "text-embedding-3-small",
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), "dimensions") {
		t.Errorf("expected dimensions to be omitted when zero, got %s", string(b))
	}
}

func TestEmbeddingResponse_UnmarshalsAndPreservesOrder(t *testing.T) {
	body := `{
		"object": "list",
		"data": [
			{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]},
			{"object": "embedding", "index": 1, "embedding": [0.4, 0.5, 0.6]}
		],
		"model": "text-embedding-3-small",
		"usage": {"prompt_tokens": 5, "total_tokens": 5}
	}`
	var resp embeddingResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Data[0].Index != 0 || resp.Data[1].Index != 1 {
		t.Errorf("expected indices [0,1], got [%d,%d]", resp.Data[0].Index, resp.Data[1].Index)
	}
	if got := resp.Data[0].Embedding[0]; got != 0.1 {
		t.Errorf("expected first embedding[0]=0.1, got %v", got)
	}
	if resp.Model != "text-embedding-3-small" {
		t.Errorf("expected model text-embedding-3-small, got %q", resp.Model)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Errorf("expected total_tokens 5, got %d", resp.Usage.TotalTokens)
	}
}

func TestErrorResponse_Unmarshal(t *testing.T) {
	body := `{
		"error": {
			"message": "Incorrect API key provided",
			"type": "invalid_request_error",
			"code": "invalid_api_key",
			"param": null
		}
	}`
	var er errorResponse
	if err := json.Unmarshal([]byte(body), &er); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if er.Error.Message != "Incorrect API key provided" {
		t.Errorf("unexpected message: %q", er.Error.Message)
	}
	if er.Error.Type != "invalid_request_error" {
		t.Errorf("unexpected type: %q", er.Error.Type)
	}
	if er.Error.Code != "invalid_api_key" {
		t.Errorf("unexpected code: %q", er.Error.Code)
	}
	if er.Error.Param != nil {
		t.Errorf("expected Param to be nil for JSON null, got %q", *er.Error.Param)
	}
}

// TestErrorResponse_UnmarshalsParamString guards the other half of the
// nullable-Param contract: when OpenAI returns a non-null param string, we
// must surface it via the pointer.
func TestErrorResponse_UnmarshalsParamString(t *testing.T) {
	body := `{
		"error": {
			"message": "you must provide a model parameter",
			"type": "invalid_request_error",
			"code": "missing_required_parameter",
			"param": "model"
		}
	}`
	var er errorResponse
	if err := json.Unmarshal([]byte(body), &er); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if er.Error.Param == nil {
		t.Fatal("expected Param to be set, got nil")
	}
	if *er.Error.Param != "model" {
		t.Errorf("expected Param=model, got %q", *er.Error.Param)
	}
}
