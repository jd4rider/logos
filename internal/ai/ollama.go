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
	"os/exec"
	"strings"
	"time"
)

// DefaultHost is the Ollama REST API base URL.
const DefaultHost = "http://localhost:11434"

// DefaultModel is the recommended small local chat model for Logos AI.
const DefaultModel = "llama3.2:3b"

// DefaultEmbedModel is the recommended Ollama embeddings model for local search.
const DefaultEmbedModel = "embeddinggemma"

// Client talks to a local Ollama instance.
type Client struct {
	host       string
	model      string
	embedModel string
	http       *http.Client
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
	embedModel := os.Getenv("OLLAMA_EMBED_MODEL")
	if embedModel == "" {
		embedModel = DefaultEmbedModel
	}
	return &Client{
		host:       host,
		model:      model,
		embedModel: embedModel,
		http:       &http.Client{Timeout: 5 * time.Minute},
	}
}

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }

// SetModel overrides the chat/generation model used for future requests.
func (c *Client) SetModel(model string) {
	model = strings.TrimSpace(model)
	if model != "" {
		c.model = model
	}
}

// EmbedModel returns the configured embeddings model name.
func (c *Client) EmbedModel() string { return c.embedModel }

// SetEmbedModel overrides the embeddings model used for future requests.
func (c *Client) SetEmbedModel(model string) {
	model = strings.TrimSpace(model)
	if model != "" {
		c.embedModel = model
	}
}

// Host returns the configured host.
func (c *Client) Host() string { return c.host }

// IsInstalled reports whether the Ollama CLI is available on PATH.
func (c *Client) IsInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// EnsureRunning starts `ollama serve` in the background when the local host is
// configured and the daemon is not already reachable.
func (c *Client) EnsureRunning(ctx context.Context) error {
	if !c.IsInstalled() {
		return fmt.Errorf("ollama is not installed")
	}
	if c.IsAvailable(ctx) {
		return nil
	}
	if c.host != DefaultHost {
		return fmt.Errorf("ollama not reachable at %s", c.host)
	}
	cmd := exec.CommandContext(ctx, "ollama", "serve")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if c.IsAvailable(ctx) {
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("ollama started but did not become ready in time")
}

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

// PullModel downloads a model into the local Ollama store.
func (c *Client) PullModel(ctx context.Context, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("model name is required")
	}

	body, _ := json.Marshal(struct {
		Name   string `json:"name"`
		Stream bool   `json:"stream"`
	}{
		Name:   model,
		Stream: false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && strings.TrimSpace(result.Error) != "" {
		return fmt.Errorf("ollama pull: %s", result.Error)
	}
	return nil
}

// generateRequest is the payload for /api/generate.
type generateRequest struct {
	Model   string   `json:"model"`
	Prompt  string   `json:"prompt"`
	Stream  bool     `json:"stream"`
	System  string   `json:"system,omitempty"`
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

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
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

// Embed generates an embeddings vector using the configured Ollama embeddings model.
func (c *Client) Embed(ctx context.Context, text string) ([]float64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("embedding text is empty")
	}

	body, _ := json.Marshal(embedRequest{
		Model: c.embedModel,
		Input: []string{text},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama returned no embeddings")
	}
	return result.Embeddings[0], nil
}
