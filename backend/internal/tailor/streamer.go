package tailor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIStreamer streams chat completions from any OpenAI-compatible API
// (Groq, OpenAI, OpenRouter, …). The API key never leaves the server.
type OpenAIStreamer struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewOpenAIStreamer builds a streamer. baseURL is the provider's
// OpenAI-compatible root (Groq: https://api.groq.com/openai/v1).
func NewOpenAIStreamer(baseURL, apiKey, model string) *OpenAIStreamer {
	return &OpenAIStreamer{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 90 * time.Second},
	}
}

type chatRequest struct {
	Model           string        `json:"model"`
	Stream          bool          `json:"stream"`
	Temperature     float64       `json:"temperature"`
	MaxTokens       int           `json:"max_tokens"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Messages        []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamText implements Deps.StreamText: accumulates streamed deltas and
// invokes onLine for each completed JSONL line of model output.
func (s *OpenAIStreamer) StreamText(ctx context.Context, system, user string, onLine func(string)) error {
	// Reasoning models (gpt-oss) think before answering; keep the effort low
	// and the budget generous so thinking cannot starve the actual output.
	effort := ""
	if strings.Contains(s.model, "gpt-oss") {
		effort = "low"
	}
	body, _ := json.Marshal(chatRequest{
		Model: s.model, Stream: true, Temperature: 0.2, MaxTokens: 8192,
		ReasoningEffort: effort,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("model request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("model returned status %d", resp.StatusCode)
	}

	var pending strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	flush := func() {
		for {
			text := pending.String()
			idx := strings.IndexByte(text, '\n')
			if idx < 0 {
				return
			}
			onLine(text[:idx])
			pending.Reset()
			pending.WriteString(text[idx+1:])
		}
	}
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // partial/keepalive frames: skip
		}
		if len(chunk.Choices) > 0 {
			pending.WriteString(chunk.Choices[0].Delta.Content)
			flush()
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("model stream failed")
	}
	flush()
	if rest := strings.TrimSpace(pending.String()); rest != "" {
		onLine(rest)
	}
	return nil
}
