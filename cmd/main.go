package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/restsend/rustpbxgo"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	endpoint := flag.String("endpoint", "ws://localhost:8080", "WebSocket endpoint")
	flag.Parse()

	// Create logger
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create media handler
	mediaHandler, err := NewMediaHandler(ctx, logger)
	if err != nil {
		logger.Fatalf("Failed to create media handler: %v", err)
	}
	defer mediaHandler.Stop()
	// Create client
	client := rustpbxgo.NewClient(*endpoint,
		rustpbxgo.WithLogger(logger),
		rustpbxgo.WithContext(ctx),
	)
	// Connect to server
	err = client.Connect()
	if err != nil {
		logger.Fatalf("Failed to connect to server: %v", err)
	}
	defer client.Shutdown()

	localSdp, err := mediaHandler.Setup()
	if err != nil {
		logger.Fatalf("Failed to get local SDP: %v", err)
	}
	logger.Infof("Offer SDP: %v", localSdp)
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

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	<-sigChan
	fmt.Println("Shutting down...")
}
