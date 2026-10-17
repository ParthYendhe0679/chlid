// Package ai contains the Gemini client and the five AI features (section 8). Every call has a
// 20-second limit and one retry, every output is stored in ai_runs, and every feature keeps working
// without AI: it falls back to rules or to "Needs human review" (section 12.3).
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/genai"
)

var ErrDisabled = errors.New("AI is not configured on this server")

type Gemini struct {
	client     *genai.Client
	Model      string
	EmbedModel string
}

func NewGemini(ctx context.Context, apiKey, model, embedModel string) (*Gemini, error) {
	if apiKey == "" {
		return nil, ErrDisabled
	}
	c, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		return nil, err
	}
	return &Gemini{client: c, Model: model, EmbedModel: embedModel}, nil
}

const callTimeout = 20 * time.Second

// retry runs fn with a 20-second limit, and once more if the first attempt fails.
func retry(ctx context.Context, fn func(context.Context) error) error {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		c, cancel := context.WithTimeout(ctx, callTimeout)
		err = fn(c)
		cancel()
		if err == nil || ctx.Err() != nil {
			return err
		}
	}
	return err
}

// JSON asks the model for output matching schema and decodes it into out.
// Replies that are not valid JSON are discarded (section 16.2).
func (g *Gemini) JSON(ctx context.Context, system string, parts []*genai.Part, schema *genai.Schema, out any) error {
	if g == nil {
		return ErrDisabled
	}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(system, genai.RoleUser),
		ResponseMIMEType:  "application/json",
		ResponseSchema:    schema,
		Temperature:       genai.Ptr[float32](0.2),
	}
	err := retry(ctx, func(c context.Context) error {
		resp, err := g.client.Models.GenerateContent(c, g.Model, []*genai.Content{genai.NewContentFromParts(parts, genai.RoleUser)}, cfg)
		if err != nil {
			return err
		}
		txt := strings.TrimSpace(resp.Text())
		txt = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(txt, "```json"), "```"), "```")
		if err := json.Unmarshal([]byte(txt), out); err != nil {
			return fmt.Errorf("model reply was not valid JSON: %w", err)
		}
		return nil
	})
	if err != nil {
		slog.Warn("gemini call failed; using fallback", "model", g.Model, "err", err)
	}
	return err
}

// Embed returns 768-number embeddings (gemini-embedding-001, section 8.2).
func (g *Gemini) Embed(ctx context.Context, texts []string, task string) ([][]float32, error) {
	if g == nil {
		return nil, ErrDisabled
	}
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += 50 {
		end := min(start+50, len(texts))
		var contents []*genai.Content
		for _, t := range texts[start:end] {
			contents = append(contents, genai.NewContentFromText(t, genai.RoleUser))
		}
		var resp *genai.EmbedContentResponse
		err := retry(ctx, func(c context.Context) error {
			var err error
			resp, err = g.client.Models.EmbedContent(c, g.EmbedModel, contents, &genai.EmbedContentConfig{
				TaskType: task, OutputDimensionality: genai.Ptr[int32](EmbedDims),
			})
			return err
		})
		if err != nil {
			return nil, err
		}
		if len(resp.Embeddings) != len(contents) {
			return nil, errors.New("embedding count mismatch")
		}
		for _, e := range resp.Embeddings {
			out = append(out, normalize(e.Values))
		}
	}
	return out, nil
}

// Schema helpers keep the response schemas short.
func obj(props map[string]*genai.Schema, required ...string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeObject, Properties: props, Required: required}
}
func str() *genai.Schema                { return &genai.Schema{Type: genai.TypeString} }
func nstr() *genai.Schema               { return &genai.Schema{Type: genai.TypeString, Nullable: genai.Ptr(true)} }
func num() *genai.Schema                { return &genai.Schema{Type: genai.TypeNumber} }
func boolean() *genai.Schema            { return &genai.Schema{Type: genai.TypeBoolean} }
func arr(s *genai.Schema) *genai.Schema { return &genai.Schema{Type: genai.TypeArray, Items: s} }
func enum(vals ...string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeString, Format: "enum", Enum: vals}
}
