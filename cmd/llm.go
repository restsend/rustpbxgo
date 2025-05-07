package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/google/uuid"
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

// QueryStream processes the LLM response as a stream and sends segments to TTS as they arrive
func (h *LLMHandler) QueryStream(model, text string, ttsCallback func(segment string, playID string, autoHangup bool) error) (string, error) {
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
		Stream:      true,
		Tools: []openai.Tool{
			{
				Type:     openai.ToolTypeFunction,
				Function: &functionDefinition,
			},
		},
	}

	// Generate a unique playID for this conversation
	playID := fmt.Sprintf("llm-%s", uuid.New().String())
	h.logger.WithField("playID", playID).Info("Starting LLM stream with playID")

	// Stream for handling responses
	stream, err := h.client.CreateChatCompletionStream(h.ctx, request)
	if err != nil {
		return "", fmt.Errorf("error creating chat completion stream: %w", err)
	}
	defer stream.Close()

	// Buffer to collect text until punctuation
	var buffer string
	fullResponse := ""
	var shouldHangup bool

	// Regular expression to detect punctuation followed by space or end of string
	punctuationRegex := regexp.MustCompile(`([.,;:!?，。！？；：])\s*`)

	// Process the stream of responses
	for {
		response, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				// Stream closed normally
				break
			}
			return "", fmt.Errorf("error receiving from stream: %w", err)
		}

		// Check for function calls (hangup)
		if len(response.Choices) > 0 && len(response.Choices[0].Delta.ToolCalls) > 0 {
			for _, toolCall := range response.Choices[0].Delta.ToolCalls {
				if toolCall.Function.Name == "hangup" {
					h.logger.Info("LLM requested hangup")
					shouldHangup = true
				}
			}
		}

		// Process content if available
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			content := response.Choices[0].Delta.Content
			buffer += content
			fullResponse += content

			// Check for punctuation in the buffer
			matches := punctuationRegex.FindAllStringSubmatchIndex(buffer, -1)
			if len(matches) > 0 {
				lastIdx := 0
				for _, match := range matches {
					// Extract the segment up to and including the punctuation
					segment := buffer[lastIdx:match[1]]
					if segment != "" {
						// Send this segment to TTS with the same playId
						if err := ttsCallback(segment, playID, false); err != nil {
							h.logger.WithError(err).Error("Failed to send TTS segment")
						}
					}
					lastIdx = match[1]
				}

				// Keep the remainder in the buffer
				if lastIdx < len(buffer) {
					buffer = buffer[lastIdx:]
				} else {
					buffer = ""
				}
			}
		}
	}

	// Send any remaining text in the buffer
	if err := ttsCallback(buffer, playID, shouldHangup); err != nil {
		h.logger.WithError(err).Error("Failed to send final TTS segment")
	}

	// Add assistant's complete response to conversation history
	h.messages = append(h.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: fullResponse,
	})

	h.logger.WithFields(logrus.Fields{
		"responseLength": len(fullResponse),
		"hangup":         shouldHangup,
	}).Info("LLM stream completed")

	return fullResponse, nil
}

// Query the LLM with text and get a response (non-streaming version, kept for compatibility)
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
