package gemini

// EmbedContentRequest represents a request to the Gemini embedding API.
type EmbedContentRequest struct {
	Model                string  `json:"model"`
	Content              Content `json:"content"`
	TaskType             string  `json:"taskType,omitempty"`
	OutputDimensionality int     `json:"outputDimensionality,omitempty"`
}

// BatchEmbedContentRequest represents a batch request for multiple embeddings.
type BatchEmbedContentRequest struct {
	Requests []EmbedContentRequest `json:"requests"`
}

// Content represents the content to be embedded.
type Content struct {
	Parts []Part `json:"parts"`
}

// Part represents a part of the content.
type Part struct {
	Text string `json:"text"`
}

// EmbedContentResponse represents a response from the Gemini embedding API.
type EmbedContentResponse struct {
	Embedding ContentEmbedding `json:"embedding"`
}

// BatchEmbedContentResponse represents a batch response with multiple embeddings.
type BatchEmbedContentResponse struct {
	Embeddings []ContentEmbedding `json:"embeddings"`
}

// ContentEmbedding represents the embedding values.
type ContentEmbedding struct {
	Values []float64 `json:"values"`
}

// ErrorResponse represents an error response from the Gemini API.
type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

// ErrorDetails contains detailed error information.
type ErrorDetails struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Status  string        `json:"status"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// ErrorDetail contains additional error detail information.
type ErrorDetail struct {
	Type        string            `json:"@type,omitempty"`
	Description string            `json:"description,omitempty"`
	Field       string            `json:"field,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
