// Package ai is the sole net/http boundary for the optional local-model
// PR title/description suggestion (design.md: "New internal/ai package is
// the sole net/http boundary, mirrors internal/update/checker.go").
// internal/app never imports this package directly; it reaches it ONLY
// through the plain-typed scalar Deps.GenerateSummary main.go composes
// (design ADR-1), and internal/prereq.Checker.AI holds the typed Client
// directly (like Checker.GH).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxResponseBodyBytes bounds how much of a response GenerateSummary/Doctor
// will read, so a hostile or huge response cannot exhaust memory (mirrors
// internal/update.Checker's maxResponseBodyBytes).
const maxResponseBodyBytes = 1 << 20 // 1 MiB

// doctorProbeTimeout bounds the Doctor reachability probe. The doctor check
// runs at TUI startup and in `deploydeck doctor` with an unbounded context,
// so without this deadline a configured-but-unresponsive endpoint (accepts the
// TCP connection but never replies) would hang startup indefinitely. It is a
// var (not const) so tests can shorten it. GenerateSummary is separately bound
// by its caller's context (commands.go aiSuggestCmd, 15s).
var doctorProbeTimeout = 5 * time.Second

// DoctorState classifies AI endpoint reachability and configured-model
// availability, mirroring github.AuthState's total, no-error 3-state shape.
type DoctorState int

const (
	Unreachable DoctorState = iota
	ModelMissing
	Ready
)

// SummaryRequest carries GenerateSummary's typed request. It stays behind
// this package's boundary — main.go's closure is the only place that talks
// in these types; internal/app only ever sees the scalar Deps.GenerateSummary
// signature (design ADR-1).
type SummaryRequest struct {
	Ticket           string
	CommitSubjects   []string
	ComponentSummary string
}

// SummaryResult is GenerateSummary's typed, already-parsed result.
type SummaryResult struct {
	Title       string
	Description string
}

// Client is the local-model shim: GenerateSummary drafts a PR
// title/description from an OpenAI-compatible chat-completions endpoint;
// Doctor reports reachability and configured-model availability. Mirrors
// internal/github.Client's typed, injectable shape.
type Client interface {
	// GenerateSummary POSTs {Endpoint}/v1/chat/completions with a
	// system/user prompt built from req (via Build) and parses the
	// response (via ParseSummary). Any transport/timeout/malformed/
	// no-usable-title failure returns a non-nil error and a zero
	// SummaryResult — callers treat every failure identically (design
	// ADR-2: silent degrade to "no suggestion").
	GenerateSummary(ctx context.Context, req SummaryRequest) (SummaryResult, error)
	// Doctor GETs {Endpoint}/v1/models and classifies the result. It is
	// TOTAL — no error return — mirroring github.Client.AuthStatus: every
	// outcome (unreachable, reachable-but-missing, reachable-and-listed)
	// is itself a valid, informative DoctorState.
	Doctor(ctx context.Context) DoctorState
}

// client is the HTTP-backed Client implementation. An OpenAI-compatible
// /v1/chat/completions + /v1/models surface covers Ollama, LM Studio, and
// llama.cpp with one client (design's "Approach 1").
type client struct {
	Endpoint   string
	Model      string
	HTTPClient *http.Client
}

// New returns a Client backed by endpoint/model, using hc for requests.
func New(endpoint, model string, hc *http.Client) Client {
	return &client{Endpoint: endpoint, Model: model, HTTPClient: hc}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// GenerateSummary implements Client.GenerateSummary. It relies entirely on
// the caller-provided ctx for request bounding (no internal timeout is
// added), mirroring internal/update.Checker.Latest.
func (c *client) GenerateSummary(ctx context.Context, req SummaryRequest) (SummaryResult, error) {
	if c.HTTPClient == nil {
		return SummaryResult{}, fmt.Errorf("ai: no HTTP client configured")
	}

	system, user := Build(req.Ticket, req.CommitSubjects, req.ComponentSummary)

	body, err := json.Marshal(chatCompletionRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream: false,
	})
	if err != nil {
		return SummaryResult{}, fmt.Errorf("ai: encoding request: %w", err)
	}

	url := c.Endpoint + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return SummaryResult{}, fmt.Errorf("ai: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return SummaryResult{}, fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SummaryResult{}, fmt.Errorf("ai: unexpected status %d from %s", resp.StatusCode, url)
	}

	var decoded chatCompletionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&decoded); err != nil {
		return SummaryResult{}, fmt.Errorf("ai: decoding response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return SummaryResult{}, fmt.Errorf("ai: response had no choices")
	}

	title, description, ok := ParseSummary(decoded.Choices[0].Message.Content)
	if !ok {
		return SummaryResult{}, fmt.Errorf("ai: response did not contain a usable title")
	}

	return SummaryResult{Title: title, Description: description}, nil
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Doctor implements Client.Doctor.
func (c *client) Doctor(ctx context.Context) DoctorState {
	if c.HTTPClient == nil {
		return Unreachable
	}

	ctx, cancel := context.WithTimeout(ctx, doctorProbeTimeout)
	defer cancel()

	url := c.Endpoint + "/v1/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Unreachable
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return Unreachable
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Unreachable
	}

	var decoded modelsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&decoded); err != nil {
		return Unreachable
	}

	for _, m := range decoded.Data {
		if m.ID == c.Model {
			return Ready
		}
	}
	return ModelMissing
}
