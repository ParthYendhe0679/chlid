package ai

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const EmbedDims = 768

// LocalEmbedModel names the offline embedder used when no Gemini key is configured.
// It hashes words and word pairs into 768 buckets (feature hashing), so similar wording scores high.
const LocalEmbedModel = "local-hash-768"

// RulesModel is recorded in ai_runs when a feature ran on rules only, so results are never mistaken for AI output.
const RulesModel = "rules-only"

type Service struct {
	Gem  *Gemini // nil when AI is not configured
	Pool *pgxpool.Pool
}

func (s *Service) Enabled() bool { return s.Gem != nil }

// ModelName is what gets written to ai_runs.model.
func (s *Service) ModelName() string {
	if s.Gem == nil {
		return RulesModel
	}
	return s.Gem.Model
}

func (s *Service) EmbedModelName() string {
	if s.Gem == nil {
		return LocalEmbedModel
	}
	return s.Gem.EmbedModel
}

// Embed uses Gemini when available, otherwise the local embedder.
func (s *Service) Embed(ctx context.Context, texts []string, task string) ([][]float32, string, error) {
	if s.Gem != nil {
		v, err := s.Gem.Embed(ctx, texts, task)
		return v, s.Gem.EmbedModel, err
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = LocalEmbed(t)
	}
	return out, LocalEmbedModel, nil
}

// RecordRun stores one AI output in ai_runs (rule 9 of section 25) and returns its id.
func (s *Service) RecordRun(ctx context.Context, feature string, actor *uuid.UUID, targetType string, targetID *uuid.UUID, model string, latencyMs int, output any, confidence *float64) (uuid.UUID, error) {
	b, err := json.Marshal(output)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	var tt any
	if targetType != "" {
		tt = targetType
	}
	err = s.Pool.QueryRow(ctx, `INSERT INTO ai_runs (feature, actor_id, target_type, target_id, model, latency_ms, output, confidence)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, feature, actor, tt, targetID, model, latencyMs, b, confidence).Scan(&id)
	return id, err
}

func normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	n := float32(1 / math.Sqrt(sum))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * n
	}
	return out
}

var stop = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "to": true, "in": true, "is": true, "are": true, "and": true, "or": true,
	"for": true, "on": true, "be": true, "it": true, "if": true, "do": true, "does": true, "what": true, "how": true,
	"i": true, "my": true, "me": true, "we": true, "our": true, "you": true, "your": true, "can": true, "with": true,
	"this": true, "that": true, "by": true, "at": true, "as": true, "from": true, "will": true, "when": true, "which": true,
	"there": true, "has": true, "have": true, "was": true, "were": true, "not": false, "after": false, "before": false,
	"का": true, "की": true, "के": true, "है": true, "में": true, "और": true, "को": true, "से": true, "क्या": true, "हैं": true,
	"आहे": true, "आणि": true, "चा": true, "ची": true, "चे": true, "मध्ये": true, "काय": true, "तर": true, "हे": true,
}

// Tokens lowercases, splits on anything that is not a letter, digit or combining mark (so Devanagari works),
// drops stop words and strips simple English suffixes.
func Tokens(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r))
	})
	out := f[:0]
	for _, w := range f {
		if stop[w] {
			continue
		}
		if len(w) > 4 && w[0] < 128 {
			for _, suf := range []string{"ing", "ed", "es", "s"} {
				if strings.HasSuffix(w, suf) && len(w)-len(suf) >= 3 {
					w = strings.TrimSuffix(w, suf)
					break
				}
			}
		}
		out = append(out, w)
	}
	return out
}

// LocalEmbed is a deterministic bag-of-words embedding (unigrams + bigrams, signed feature hashing).
func LocalEmbed(text string) []float32 {
	v := make([]float32, EmbedDims)
	toks := Tokens(text)
	add := func(term string, w float32) {
		h := fnv.New64a()
		h.Write([]byte(term))
		x := h.Sum64()
		sign := float32(1)
		if x>>63 == 1 {
			sign = -1
		}
		v[x%EmbedDims] += sign * w
	}
	for i, t := range toks {
		add(t, 1)
		if i+1 < len(toks) {
			add(t+" "+toks[i+1], 0.6)
		}
	}
	for i, x := range v {
		if x != 0 {
			s := float32(1)
			if x < 0 {
				s = -1
			}
			v[i] = s * float32(math.Log1p(math.Abs(float64(x))))
		}
	}
	return normalize(v)
}

// VectorLiteral formats an embedding for a pgvector parameter ('[0.1,0.2,...]'::vector).
func VectorLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'g', 7, 32))
	}
	b.WriteByte(']')
	return b.String()
}
