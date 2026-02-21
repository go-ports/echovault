package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// doJSON executes an HTTP request, marshalling body as JSON and unmarshalling
// the response into out. Pass nil body for GET requests. Pass nil out to discard
// the response body. Returns an error on non-2xx status codes.
func doJSON(ctx context.Context, client *http.Client, method, url string, headers map[string]string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("doJSON marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("doJSON new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req) // #nosec G704 -- SSRF risk accepted; URL is the user-configured embedding provider endpoint
	if err != nil {
		return fmt.Errorf("doJSON request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("doJSON: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("doJSON decode: %w", err)
		}
	}
	return nil
}
