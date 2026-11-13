package curl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// execCommandContext wraps exec.CommandContext so dns.go and curl.go share
// one entry point for invoking the curl binary.
func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// Get makes a GET request via curl and returns the response body
func Get(ctx context.Context, url string, headers map[string]string) (string, error) {
	args := []string{"-s", "-L", url}

	for k, v := range headers {
		args = append(args, "-H", k+": "+v)
	}

	cmd := exec.CommandContext(ctx, "curl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("curl failed: %w", err)
	}

	return string(output), nil
}

// Post makes a POST request via curl and returns the response body
func Post(ctx context.Context, url string, headers map[string]string, body string) (string, error) {
	args := []string{"-s", "-L", "-X", "POST", "-H", "Content-Type: application/json"}

	for k, v := range headers {
		args = append(args, "-H", k+": "+v)
	}

	if body != "" {
		args = append(args, "-d", body)
	}

	args = append(args, url)

	cmd := exec.CommandContext(ctx, "curl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("curl failed: %w", err)
	}

	return string(output), nil
}

// PostRaw sends POST request with raw body (for GraphQL)
func PostRaw(ctx context.Context, url string, headers map[string]string, body []byte) ([]byte, error) {
	args := []string{"-s", "-X", "POST"}

	for k, v := range headers {
		args = append(args, "-H", k+": "+v)
	}

	args = append(args, url)

	cmd := exec.CommandContext(ctx, "curl", args...)
	cmd.Stdin = bytes.NewReader(body)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	return output, nil
}

// ParseJSONBody extracts JSON from curl output (handles empty wrapper)
func ParseJSONBody(data []byte) ([]byte, error) {
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	// Check for "data" wrapper
	if d, ok := wrapper["data"]; ok {
		if d == nil {
			return nil, fmt.Errorf("null data in response")
		}
		return json.Marshal(d)
	}

	return data, nil
}