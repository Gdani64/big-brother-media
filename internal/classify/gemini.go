package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sethvargo/go-retry"
	"google.golang.org/genai"
)

const (
	defaultGeminiModel = "gemini-3.5-flash-lite"
	responseMIMEType   = "application/json"
	maxOutputTokens    = 32
)

type Input struct {
	Name      string
	FilePaths []string
}

// classification is the shape Gemini must return, per responseSchema below.
type classification struct {
	MediaType string `json:"media_type"`
}

var responseSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"media_type": {
			Type: genai.TypeString,
			Enum: AllMediaTypeNames(),
		},
	},
	Required: []string{"media_type"},
}

type Gemini struct {
	client *genai.Client
	model  string
}

type addGeminiParams struct {
	model string
}

type AddGeminiOption func(*addGeminiParams)

func WithModel(model string) AddGeminiOption {
	return func(p *addGeminiParams) {
		p.model = model
	}
}

func NewGemini(opts ...AddGeminiOption) (*Gemini, error) {
	params := &addGeminiParams{}
	for _, opt := range opts {
		opt(params)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	model := params.model
	if model == "" {
		model = defaultGeminiModel
	}

	return &Gemini{
		client: client,
		model:  model,
	}, nil
}

func (g *Gemini) Query(in Input) (MediaType, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 650*time.Second) // Keep this timeout value higher than
	// the exponential backoff10+20+40+80+160+320 = 630s
	defer cancel()

	filePaths := in.FilePaths[:min(len(in.FilePaths), 100)]

	contentConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: responseMIMEType,
		MaxOutputTokens:  maxOutputTokens,
		ResponseSchema:   responseSchema,
	}

	contents := genai.Text(
		fmt.Sprintf(
			"Classify archive %s containing the listed files into one of the following categories: %s.\n Files: %s",
			in.Name,
			strings.Join(AllMediaTypeNames(), ", "),
			strings.Join(filePaths, ", ")),
	)

	b := retry.NewExponential(10 * time.Second)
	result, err := retry.DoValue(ctx, retry.WithMaxRetries(6, b), func(ctx context.Context) (*genai.GenerateContentResponse, error) {
		result, err := g.client.Models.GenerateContent(ctx, g.model, contents, contentConfig)
		if err != nil {
			return nil, retry.RetryableError(fmt.Errorf("error from google AI studio API: %v", err))
		}

		return result, nil
	})
	if err != nil {
		return TypeUnknown, err
	}

	var parsed classification
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		return TypeUnknown, err
	}

	mt, ok := MediaTypeFromName(parsed.MediaType)
	if !ok {
		return TypeUnknown, nil
	}
	return mt, nil
}
