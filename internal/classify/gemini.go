package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	defaultGeminiModel = "gemini-3.5-flash-lite"
	responseMIMEType   = "application/json"
	maxOutputTokens    = 32
)

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

func (g *Gemini) Query(name string, filePaths []string) (MediaType, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	filePaths = filePaths[:min(len(filePaths), 100)]

	contentConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: responseMIMEType,
		MaxOutputTokens:  maxOutputTokens,
		ResponseSchema:   responseSchema,
	}

	contents := genai.Text(
		fmt.Sprintf(
			"Classify archive %s containing the listed files into one of the following categories: %s.\n Files: %s",
			name,
			strings.Join(AllMediaTypeNames(), ", "),
			strings.Join(filePaths, ", ")),
	)

	result, err := g.client.Models.GenerateContent(ctx, g.model, contents, contentConfig)
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
