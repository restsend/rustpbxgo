package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

// LLMHandler manages interactions with OpenAI
type LLMHandler struct {
	client      *openai.Client
	systemMsg   string
	mutex       sync.Mutex
	logger      *logrus.Logger
	ctx         context.Context
	messages    []openai.ChatCompletionMessage
	hangupChan  chan struct{}
	interruptCh chan struct{}
}

// ToolCall represents a function call from the LLM
type HangupTool struct {
	Reason string `json:"reason"`
}

// NewLLMHandler creates a new LLM handler
func NewLLMHandler(ctx context.Context, apiKey, endpoint, systemPrompt string, logger *logrus.Logger) *LLMHandler {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = endpoint
	client := openai.NewClientWithConfig(config)
	// Create system message
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	return &LLMHandler{
		client:      client,
		systemMsg:   systemPrompt,
		logger:      logger,
		ctx:         ctx,
		messages:    messages,
		hangupChan:  make(chan struct{}),
		interruptCh: make(chan struct{}, 1),
	}
}

// Query the LLM with text and get a response
func (h *LLMHandler) Query(model, text string) (string, *HangupTool, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Add user message to history
	h.messages = append(h.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	})

	// Define the function for hanging up
	functionDefinition := openai.FunctionDefinition{
		Name:        "hangup",
		Description: "End the conversation and hang up the call",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"reason": {
					"type": "string",
					"description": "Reason for hanging up the call"
				}
			},
			"required": []
		}`),
	}

	// Construct the OpenAI request
	if model == "" {
		model = openai.GPT4o
	}
	request := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    h.messages,
		Temperature: 0.7,
		Tools: []openai.Tool{
			{
				Type:     openai.ToolTypeFunction,
				Function: &functionDefinition,
			},
		},
	}

	// Send the request to OpenAI
	response, err := h.client.CreateChatCompletion(h.ctx, request)
	if err != nil {
		return "", nil, fmt.Errorf("error querying OpenAI: %w", err)
	}

	// Process the response
	message := response.Choices[0].Message
	h.messages = append(h.messages, message)

	// Check if there's a tool call for hangup
	var hangupTool *HangupTool
	// Check for tool calls
	if len(message.ToolCalls) > 0 {
		for _, toolCall := range message.ToolCalls {
			if toolCall.Function.Name == "hangup" {
				hangupTool = &HangupTool{}
				// Parse the arguments
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), hangupTool); err != nil {
					h.logger.WithError(err).Error("Failed to parse hangup arguments")
				} else {
					h.logger.WithField("reason", hangupTool.Reason).Info("llm: Hangup reason")
				}
			}
		}
	}

	return message.Content, hangupTool, nil
}

// Reset clears the conversation history but keeps the system prompt
func (h *LLMHandler) Reset() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Reset to just the system message
	h.messages = []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: h.systemMsg,
		},
	}
}
