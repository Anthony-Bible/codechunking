package openai

// embeddingRequest is the JSON body posted to {base_url}/embeddings.
// Dimensions is omitted when zero so OpenAI-compatible servers that do not
// understand the field (older Ollama, some vLLM builds) are not sent it.
type embeddingRequest struct {
	Input          []string `json:"input"`
	Model          string   `json:"model"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	Dimensions     int      `json:"dimensions,omitempty"`
}

// embeddingResponse mirrors OpenAI's success response.
//
//	{
//	  "object": "list",
//	  "data": [{"object": "embedding", "index": 0, "embedding": [...]}, ...],
//	  "model": "text-embedding-3-small",
//	  "usage": {"prompt_tokens": 5, "total_tokens": 5}
//	}
type embeddingResponse struct {
	Object string                  `json:"object"`
	Data   []embeddingResponseItem `json:"data"`
	Model  string                  `json:"model"`
	Usage  embeddingUsage          `json:"usage"`
}

type embeddingResponseItem struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type embeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// errorResponse mirrors OpenAI's error envelope.
//
//	{"error": {"message": "...", "type": "...", "code": "..."}}
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Code    string  `json:"code"`
	Param   *string `json:"param,omitempty"`
}
