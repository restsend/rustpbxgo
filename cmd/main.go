package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/restsend/rustpbxgo"
	"github.com/sirupsen/logrus"
)

func main() {
	godotenv.Load()
	// Parse command line flags
	endpoint := flag.String("endpoint", "ws://localhost:8080", "WebSocket endpoint")
	codec := flag.String("codec", "g722", "Codec to use, g722 or pcmu")
	openaiKey := flag.String("openai-key", "", "OpenAI API key")
	openaiModel := flag.String("openai-model", "", "OpenAI model to use")
	openaiEndpoint := flag.String("openai-endpoint", "", "OpenAI endpoint to use")
	systemPrompt := flag.String("system-prompt", "You are a helpful assistant. Provide concise responses. Use 'hangup' tool when the conversation is complete.", "System prompt for LLM")
	breakOnVad := flag.Bool("break-on-vad", false, "Break on VAD")
	flag.Parse()

	if *openaiKey == "" {
		*openaiKey = os.Getenv("OPENAI_API_KEY")
		if *openaiKey == "" {
			fmt.Println("OpenAI API key is required. Set it with --openai-key flag or OPENAI_API_KEY environment variable.")
			os.Exit(1)
		}
	}
	if *openaiEndpoint == "" {
		*openaiEndpoint = os.Getenv("OPENAI_ENDPOINT")
	}
	if *openaiModel == "" {
		*openaiModel = os.Getenv("OPENAI_MODEL")
	}

	// Create logger
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create LLM handler
	llmHandler := NewLLMHandler(ctx, *openaiKey, *openaiEndpoint, *systemPrompt, logger)

	// Create media handler
	mediaHandler, err := NewMediaHandler(ctx, logger)
	if err != nil {
		logger.Fatalf("Failed to create media handler: %v", err)
	}
	defer mediaHandler.Stop()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create client
	client := rustpbxgo.NewClient(*endpoint,
		rustpbxgo.WithLogger(logger),
		rustpbxgo.WithContext(ctx),
	)
	client.OnClose = func(reason string) {
		logger.Infof("Connection closed: %s", reason)
		sigChan <- syscall.SIGTERM
	}
	client.OnEvent = func(event string, payload string) {
		logger.Debugf("Received event: %s %s", event, payload)
	}

	// Handle ASR Final events
	client.OnAsrFinal = func(event rustpbxgo.AsrFinalEvent) {
		client.Interrupt()
		logger.Infof("ASR Final: %s", event.Text)
		if event.Text == "" {
			return
		}
		startTime := time.Now()
		response, shouldHangup, err := llmHandler.Query(*openaiModel, event.Text)
		if err != nil {
			logger.Errorf("Error querying LLM: %v", err)
			return
		}
		logger.Infof("LLM response: %s duration: %s", response, time.Since(startTime))
		if shouldHangup != nil {
			logger.Infof("LLM shouldHangup: %v", shouldHangup)
		}
		// Speak the response
		if response != "" {
			if err := client.TTS(response, "", "", shouldHangup != nil); err != nil {
				logger.Errorf("Error sending TTS: %v", err)
			}
		} else if shouldHangup != nil {
			client.Hangup(shouldHangup.Reason)
		}
	}
	client.OnHangup = func(event rustpbxgo.HangupEvent) {
		logger.Infof("Hangup: %s", event.Reason)
		sigChan <- syscall.SIGTERM
	}
	// Handle ASR Delta events (partial results)
	client.OnAsrDelta = func(event rustpbxgo.AsrDeltaEvent) {
		logger.Debugf("ASR Delta: %s", event.Text)
		if *breakOnVad {
			return
		}
		if err := client.Interrupt(); err != nil {
			logger.Warnf("Failed to interrupt TTS: %v", err)
		}
	}
	client.OnSpeaking = func(event rustpbxgo.SpeakingEvent) {
		if !*breakOnVad {
			return
		}
		logger.Infof("Interrupting TTS")
		if err := client.Interrupt(); err != nil {
			logger.Warnf("Failed to interrupt TTS: %v", err)
		}
	}
	// Connect to server
	err = client.Connect()
	if err != nil {
		logger.Fatalf("Failed to connect to server: %v", err)
	}
	defer client.Shutdown()

	localSdp, err := mediaHandler.Setup(*codec)
	if err != nil {
		logger.Fatalf("Failed to get local SDP: %v", err)
	}
	logger.Infof("Offer SDP: %v", localSdp)

	// Start the call
	answer, err := client.Invite(ctx, rustpbxgo.StreamOption{
		Denoise: true,
		VAD: &rustpbxgo.VADOption{
			Type: "silero",
		},
		ASR: &rustpbxgo.ASROption{
			Provider: "tencent",
		},
		TTS: &rustpbxgo.TTSOption{
			Provider: "tencent",
		},
		Offer: localSdp,
	})
	if err != nil {
		logger.Fatalf("Failed to invite: %v", err)
	}
	logger.Infof("Answer SDP: %v", answer.Sdp)

	err = mediaHandler.SetupAnswer(answer.Sdp)
	if err != nil {
		logger.Fatalf("Failed to setup answer: %v", err)
	}

	// Initial greeting
	client.TTS("Hello, how can I help you?", "", "", false)

	<-sigChan
	fmt.Println("Shutting down...")
}
