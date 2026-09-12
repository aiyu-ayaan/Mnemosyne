// Package embed turns text into vectors through a pluggable provider.
//
// The default is Ollama on localhost, because a memory store whose semantic
// search ships user memories to a third party by default would be the wrong
// default for a local, single-user tool. Any OpenAI-compatible endpoint works
// as an explicit opt-in.
//
// Nothing here is required: with no provider reachable, keyword search still
// works and the semantic modes report themselves unavailable rather than
// failing a query.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider names, as they appear in the config file.
const (
	ProviderOllama = "ollama"
	ProviderOpenAI = "openai"
	ProviderNone   = "none"
)

// Defaults for the local-first path. nomic-embed-text is small, runs on a CPU,
// and is what `ollama pull nomic-embed-text` gets.
const (
	DefaultProvider = ProviderOllama
	DefaultModel    = "nomic-embed-text"
	DefaultEndpoint = "http://localhost:11434"
)

// requestTimeout bounds one batch. Embedding is index work, so a slow or wedged
// provider must not hang a search indefinitely.
const requestTimeout = 60 * time.Second

// MaxBatch is how many texts go in one request. Large enough to amortise the
// round trip, small enough that one failure does not lose much work.
const MaxBatch = 32

// Settings is the embedding section of the config file.
type Settings struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`

	// APIKeyEnv names an environment variable holding the key. The key itself
	// is deliberately not storable in the config file: that file is plain text
	// in the user's profile and gets copied into bug reports.
	APIKeyEnv string `json:"apiKeyEnv,omitempty"`
}

// WithDefaults fills the blanks, so an empty config means "local Ollama".
func (s Settings) WithDefaults() Settings {
	if s.Provider == "" {
		s.Provider = DefaultProvider
	}
	if s.Model == "" && s.Provider == ProviderOllama {
		s.Model = DefaultModel
	}
	if s.Endpoint == "" && s.Provider == ProviderOllama {
		s.Endpoint = DefaultEndpoint
	}
	return s
}

// Provider produces vectors.
type Provider interface {
	// Embed returns one vector per input, in order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Model identifies what produced the vectors. It is stored alongside them:
	// vectors from two models are not comparable, so a model change has to
	// invalidate what is already indexed.
	Model() string

	// Available reports whether the provider answers right now.
	Available(ctx context.Context) error
}

// New builds a provider from settings. A nil Provider and a nil error mean the
// user turned embeddings off, which is a valid configuration rather than a
// failure — callers check for nil.
func New(s Settings) (Provider, error) {
	s = s.WithDefaults()

	switch s.Provider {
	case ProviderNone:
		return nil, nil

	case ProviderOllama:
		return &ollama{endpoint: strings.TrimRight(s.Endpoint, "/"), model: s.Model}, nil

	case ProviderOpenAI:
		if s.Endpoint == "" {
			s.Endpoint = "https://api.openai.com"
		}
		if s.Model == "" {
			return nil, fmt.Errorf("the %s provider needs a model in the config file", ProviderOpenAI)
		}
		key := ""
		if s.APIKeyEnv != "" {
			key = os.Getenv(s.APIKeyEnv)
		}
		return &openAI{endpoint: strings.TrimRight(s.Endpoint, "/"), model: s.Model, key: key}, nil

	default:
		return nil, fmt.Errorf("unknown embedding provider %q: use %q, %q, or %q",
			s.Provider, ProviderOllama, ProviderOpenAI, ProviderNone)
	}
}

// Normalise scales v to unit length in place and returns it, so similarity is a
// plain dot product later. Doing it once at index time rather than per
// comparison is what keeps the brute-force search cheap.
func Normalise(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	norm := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= norm
	}
	return v
}

// Similarity is the dot product, which equals cosine similarity for normalised
// vectors. Mismatched lengths score zero rather than panicking: that means two
// different models, and the caller's reindex will replace them.
func Similarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// --- Ollama ---

type ollama struct {
	endpoint string
	model    string
}

func (o *ollama) Model() string { return o.model }

func (o *ollama) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error"`
	}
	body := map[string]any{"model": o.model, "input": texts}

	if err := post(ctx, o.endpoint+"/api/embed", nil, body, &out); err != nil {
		return nil, fmt.Errorf("ollama at %s: %w", o.endpoint, err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("ollama at %s: %s", o.endpoint, out.Error)
	}
	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama returned %d vectors for %d inputs", len(out.Embeddings), len(texts))
	}
	return out.Embeddings, nil
}

func (o *ollama) Available(ctx context.Context) error {
	// One real embedding, not a version check: a running Ollama with the model
	// not pulled would pass a version check and fail every query.
	vectors, err := o.Embed(ctx, []string{"ping"})
	if err != nil {
		return err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return fmt.Errorf("ollama at %s returned an empty vector for model %q", o.endpoint, o.model)
	}
	return nil
}

// --- OpenAI-compatible ---

type openAI struct {
	endpoint string
	model    string
	key      string
}

func (a *openAI) Model() string { return a.model }

func (a *openAI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	headers := map[string]string{}
	if a.key != "" {
		headers["Authorization"] = "Bearer " + a.key
	}
	body := map[string]any{"model": a.model, "input": texts}

	if err := post(ctx, a.endpoint+"/v1/embeddings", headers, body, &out); err != nil {
		return nil, fmt.Errorf("embedding endpoint %s: %w", a.endpoint, err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("embedding endpoint %s: %s", a.endpoint, out.Error.Message)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("endpoint returned %d vectors for %d inputs", len(out.Data), len(texts))
	}

	// The response carries an index per item, and the spec does not promise
	// they arrive in order.
	vectors := make([][]float32, len(texts))
	for _, item := range out.Data {
		if item.Index < 0 || item.Index >= len(vectors) {
			return nil, fmt.Errorf("endpoint returned out-of-range index %d", item.Index)
		}
		vectors[item.Index] = item.Embedding
	}
	for i, v := range vectors {
		if v == nil {
			return nil, fmt.Errorf("endpoint returned no vector for input %d", i)
		}
	}
	return vectors, nil
}

func (a *openAI) Available(ctx context.Context) error {
	vectors, err := a.Embed(ctx, []string{"ping"})
	if err != nil {
		return err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return fmt.Errorf("embedding endpoint %s returned an empty vector", a.endpoint)
	}
	return nil
}

// --- shared transport ---

func post(ctx context.Context, url string, headers map[string]string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// The body is where these APIs put the reason, and a bare status code
		// is not enough to act on.
		var snippet bytes.Buffer
		snippet.ReadFrom(res.Body)
		return fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(snippet.String()))
	}
	return json.NewDecoder(res.Body).Decode(out)
}
