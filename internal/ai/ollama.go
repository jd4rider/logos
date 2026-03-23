// Package ai provides Ollama integration for Logos AI.
// It handles streaming chat, model management, and Bible-aware prompts.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultHost is the Ollama REST API base URL.
const DefaultHost = "http://localhost:11434"

// DefaultModel is the best general-purpose model for Bible study tasks.
const DefaultModel = "llama3.1:8b"

// Client talks to a local Ollama instance.
type Client struct {
	host   string
	model  string
	http   *http.Client
}

// NewClient creates a Client, reading OLLAMA_HOST and OLLAMA_MODEL from env.
func NewClient() *Client {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = DefaultHost
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		host:  host,
		model: model,
		http:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }

// Host returns the configured host.
func (c *Client) Host() string { return c.host }

// IsAvailable checks if Ollama is reachable and the model is available.
func (c *Client) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.host+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ListModels returns the names of all locally available Ollama models.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.host+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama not reachable at %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// generateRequest is the payload for /api/generate.
type generateRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	System  string `json:"system,omitempty"`
	Options *Options `json:"options,omitempty"`
}

// Options tunes the Ollama generation.
type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// generateChunk is a single streaming response chunk.
type generateChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate calls /api/generate and streams tokens to the returned channel.
// The caller should range over the channel until it is closed (on done or error).
// cancelCtx can be used to abort the generation.
func (c *Client) Generate(ctx context.Context, system, prompt string, opts *Options) (<-chan string, <-chan error) {
	out := make(chan string, 64)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		body, _ := json.Marshal(generateRequest{
			Model:   c.model,
			Prompt:  prompt,
			System:  system,
			Stream:  true,
			Options: opts,
		})

		req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/api/generate", bytes.NewReader(body))
		if err != nil {
			errc <- err
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			errc <- fmt.Errorf("ollama: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			errc <- fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(b))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var chunk generateChunk
			if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
				continue
			}
			if chunk.Response != "" {
				select {
				case out <- chunk.Response:
				case <-ctx.Done():
					return
				}
			}
			if chunk.Done {
				return
			}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			errc <- err
		}
	}()

	return out, errc
}

// GenerateFull calls Generate and collects all tokens into a single string.
func (c *Client) GenerateFull(ctx context.Context, system, prompt string, opts *Options) (string, error) {
	tokens, errc := c.Generate(ctx, system, prompt, opts)
	var sb strings.Builder
	for t := range tokens {
		sb.WriteString(t)
	}
	if err := <-errc; err != nil {
		return sb.String(), err
	}
	return sb.String(), nil
}
