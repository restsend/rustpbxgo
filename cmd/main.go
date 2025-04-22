package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/pion/webrtc/v3"
	"github.com/restsend/rustpbxgo"
	"github.com/sirupsen/logrus"
)

func main() {
	godotenv.Load()
	// Parse command line flags
	var endpoint string = "wss://rustpbx.com"
	var codec string = "g722"
	var openaiKey string = ""
	var openaiModel string = ""
	var openaiEndpoint string = ""
	var systemPrompt string = "You are a helpful assistant. Provide concise responses. Use 'hangup' tool when the conversation is complete."
	var breakOnVad bool = false
	var speaker string = ""
	var callWithSip bool = false
	var record bool = false
	var ttsProvider string = "tencent"
	var asrProvider string = "tencent"
	flag.StringVar(&endpoint, "endpoint", endpoint, "Endpoint to connect to")
	flag.StringVar(&codec, "codec", codec, "Codec to use: g722, pcmu")
	flag.StringVar(&openaiKey, "openai-key", openaiKey, "OpenAI API key")
	flag.StringVar(&openaiModel, "model", openaiModel, "OpenAI model to use: qwen-14b, qwen-turbo")
	flag.StringVar(&openaiEndpoint, "openai-endpoint", openaiEndpoint, "OpenAI endpoint to use")
	flag.StringVar(&systemPrompt, "system-prompt", systemPrompt, "System prompt to use")
	flag.BoolVar(&breakOnVad, "break-on-vad", breakOnVad, "Break on VAD")
	flag.BoolVar(&callWithSip, "sip", callWithSip, "Call with SIP")
	flag.BoolVar(&record, "record", record, "Record the call")
	flag.StringVar(&ttsProvider, "tts", ttsProvider, "TTS provider to use: tencent, voiceapi")
	flag.StringVar(&asrProvider, "asr", asrProvider, "ASR provider to use: tencent, voiceapi")
	flag.StringVar(&speaker, "speaker", speaker, "Speaker to use")
	flag.Parse()
	u, err := url.Parse(endpoint)
	if err != nil {
		fmt.Printf("Failed to parse endpoint: %v", err)
		os.Exit(1)
	}
	endpointHost := u.Host
	endpointSecurity := strings.ToLower(u.Scheme) == "wss"
	if openaiKey == "" {
		openaiKey = os.Getenv("OPENAI_API_KEY")

	}
	if openaiEndpoint == "" {
		openaiEndpoint = os.Getenv("OPENAI_ENDPOINT")
	}
	if openaiModel == "" {
		openaiModel = os.Getenv("OPENAI_MODEL")
	}

	if openaiEndpoint == "" {
		openaiEndpoint = endpointHost
		if endpointSecurity {
			openaiEndpoint = "https://" + u.Host
		} else {
			openaiEndpoint = "http://" + u.Host
		}
		openaiEndpoint += "/llm/v1"
		if openaiModel == "" {
			openaiModel = "qwen-14b"
		}
	}

	// Create logger
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create LLM handler
	llmHandler := NewLLMHandler(ctx, openaiKey, openaiEndpoint, systemPrompt, logger)

	// Handle signals for graceful shutdown
	sigChan := make(chan bool, 1)

	// Create client
	client := rustpbxgo.NewClient(endpoint,
		rustpbxgo.WithLogger(logger),
		rustpbxgo.WithContext(ctx),
	)
	client.OnClose = func(reason string) {
		logger.Infof("Connection closed: %s", reason)
		os.Exit(0)
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

		err := llmHandler.QueryStream(openaiModel, event.Text, func(segment string, playID string, autoHangup bool) error {
			logger.WithFields(logrus.Fields{
				"segment":    segment,
				"playID":     playID,
				"autoHangup": autoHangup,
			}).Debug("Sending TTS segment")
			return client.TTS(segment, "", playID, autoHangup)
		})

		if err != nil {
			logger.Errorf("Error querying LLM stream: %v", err)
			return
		}
		logger.Infof("LLM streaming response completed in: %s", time.Since(startTime))
	}
	client.OnHangup = func(event rustpbxgo.HangupEvent) {
		logger.Infof("Hangup: %s", event.Reason)
		sigChan <- true
	}
	// Handle ASR Delta events (partial results)
	client.OnAsrDelta = func(event rustpbxgo.AsrDeltaEvent) {
		logger.Debugf("ASR Delta: %s", event.Text)
		if breakOnVad {
			return
		}
		if err := client.Interrupt(); err != nil {
			logger.Warnf("Failed to interrupt TTS: %v", err)
		}
	}
	client.OnSpeaking = func(event rustpbxgo.SpeakingEvent) {
		if !breakOnVad {
			return
		}
		logger.Infof("Interrupting TTS")
		if err := client.Interrupt(); err != nil {
			logger.Warnf("Failed to interrupt TTS: %v", err)
		}
	}
	callType := "webrtc"
	if callWithSip {
		callType = "sip"
	}
	// Connect to server
	err = client.Connect(callType)
	if err != nil {
		logger.Fatalf("Failed to connect to server: %v", err)
	}
	defer client.Shutdown()

	// Create media handler
	mediaHandler, err := NewMediaHandler(ctx, logger)
	if err != nil {
		logger.Fatalf("Failed to create media handler: %v", err)
	}
	defer mediaHandler.Stop()
	var recorder *rustpbxgo.RecorderOption
	if record {
		recorder = &rustpbxgo.RecorderOption{
			Samplerate: 16000,
		}
	}
	options := rustpbxgo.StreamOption{
		Recorder: recorder,
		Denoise:  true,
		VAD: &rustpbxgo.VADOption{
			Type: "silero",
		},
		ASR: &rustpbxgo.ASROption{
			Provider: asrProvider,
		},
		TTS: &rustpbxgo.TTSOption{
			Provider: ttsProvider,
			Speaker:  speaker,
		},
	}
	if callWithSip {
		options.Caller = os.Getenv("SIP_CALLER")
		options.Callee = os.Getenv("SIP_CALLEE")
		if options.Caller == "" || options.Callee == "" {
			logger.Fatalf("SIP_CALLER and SIP_CALLEE must be set")
		}
		sipOption := rustpbxgo.SipOption{
			Username: os.Getenv("SIP_USERNAME"),
			Password: os.Getenv("SIP_PASSWORD"),
			Domain:   os.Getenv("SIP_DOMAIN"),
		}
		options.Sip = &sipOption
	} else {
		var iceServers []webrtc.ICEServer
		//http.Get("")
		iceUrl := "https://"
		if !endpointSecurity {
			iceUrl = "http://"
		}
		iceUrl = fmt.Sprintf("%s%s/iceservers", iceUrl, endpointHost)
		resp, err := http.Get(iceUrl)
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			json.Unmarshal(body, &iceServers)
			logger.WithFields(logrus.Fields{
				"iceservers": len(iceServers),
			}).Info("get iceservers")
		}

		localSdp, err := mediaHandler.Setup(codec, iceServers)
		if err != nil {
			logger.Fatalf("Failed to get local SDP: %v", err)
		}
		logger.Infof("Offer SDP: %v", localSdp)
		options.Offer = localSdp
	}
	// Start the call
	answer, err := client.Invite(ctx, options)
	if err != nil {
		logger.Fatalf("Failed to invite: %v", err)
	}
	logger.Infof("Answer SDP: %v", answer.Sdp)

	if !callWithSip {
		// local media
		err = mediaHandler.SetupAnswer(answer.Sdp)
		if err != nil {
			logger.Fatalf("Failed to setup answer: %v", err)
		}
	}

	// Initial greeting
	client.TTS("Hello, how can I help you?", "", "", false)

	<-sigChan
	fmt.Println("Shutting down...")
}
