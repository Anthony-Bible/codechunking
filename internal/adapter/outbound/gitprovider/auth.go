package gitprovider

import "net/http"

// addBearerAuth sets the Authorization: Bearer <token> header if token is non-nil and non-empty.
func addBearerAuth(req *http.Request, token *string) {
	if token != nil && *token != "" {
		req.Header.Set("Authorization", "Bearer "+*token)
	}
}
