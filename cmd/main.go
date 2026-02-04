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

	"github.com/pion/webrtc/v3"
	"github.com/restsend/rustpbxgo"
	"github.com/sirupsen/logrus"
)

type CreateClientOption struct {
	Endpoint       string
	Logger         *logrus.Logger
	SigChan        chan bool
	LLMHandler     *LLMHandler
	OpenaiKey      string
	OpenaiEndpoint string
	OpenaiModel    string
	SystemPrompt   string
	BreakOnVad     bool
	CallOption     rustpbxgo.CallOption
	Ringtone       string
	ReferCaller    string
	ReferCallee    string
	Greeting       string
	StreamingTTS   bool
	MaxCallTime    uint
}

func createClient(ctx context.Context, option CreateClientOption, id string) *rustpbxgo.Client {
	client := rustpbxgo.NewClient(option.Endpoint,
		rustpbxgo.WithLogger(option.Logger),
		rustpbxgo.WithContext(ctx),
		rustpbxgo.WithID(id),
		rustpbxgo.WithDumpEvents(true),
	)
	option.LLMHandler = NewLLMHandler(ctx, option.OpenaiKey, option.OpenaiEndpoint, option.SystemPrompt, option.Logger)
	option.LLMHandler.ReferTarget = option.ReferCallee

	client.OnTrackEnd = func(event rustpbxgo.TrackEndEvent) {
		playId := ""
		if event.PlayId != nil {
			playId = *event.PlayId
		}
		option.Logger.WithFields(logrus.Fields{
			"trackID":  event.TrackID,
			"duration": event.Duration,
			"playId":   playId,
		}).Infof("Track ended")
	}
	client.OnClose = func(reason string) {
		option.Logger.Infof("Connection closed: %s", reason)
		option.SigChan <- true
	}
	client.OnEvent = func(event string, payload string) {
		option.Logger.Debugf("Received event: %s %s", event, payload)
	}
	client.OnError = func(event rustpbxgo.ErrorEvent) {
		option.Logger.Errorf("Error: %v", event)
	}
	client.OnDTMF = func(event rustpbxgo.DTMFEvent) {
		option.Logger.Infof("DTMF: %s", event.Digit)
	}
	client.OnBinary = func(trackIdx int, data []byte) {
		option.Logger.Debugf("Binary: trackIdx=%d data len=%d", trackIdx, len(data))
	}
	client.OnRinging = func(event rustpbxgo.RingingEvent) {
		option.Logger.WithFields(logrus.Fields{
			"id":         event.TrackID,
			"earlyMedia": event.EarlyMedia,
		}).Info("Ringing")
	}
	// Handle ASR Final events
	client.OnAsrFinal = func(event rustpbxgo.AsrFinalEvent) {
		if event.Text != "" {
			client.History("user", event.Text)
		}

		client.Interrupt(true)
		startTime := time.UnixMilli(int64(*event.StartTime))
		endTime := time.UnixMilli(int64(*event.EndTime))
		option.Logger.Infof("ASR Final: %s startTime: %s endTime: %s", event.Text, startTime.String(), endTime.String())
		if event.Text == "" {
			return
		}

		startTime = time.Now()
		response, err := option.LLMHandler.QueryStream(option.OpenaiModel, event.Text, option.StreamingTTS, client, option.ReferCaller)
		if err != nil {
			option.Logger.Errorf("Error querying LLM stream: %v", err)
			return
		}
		option.Logger.Infof("LLM streaming response completed in: %s", time.Since(startTime))
		client.History("bot", response)
	}
	client.OnHangup = func(event rustpbxgo.HangupEvent) {
		option.Logger.Infof("Hangup: %s", event.Reason)
		//option.SigChan <- true
	}
	// Handle ASR Delta events (partial results)
	client.OnAsrDelta = func(event rustpbxgo.AsrDeltaEvent) {
		startTime := time.UnixMilli(int64(*event.StartTime))
		endTime := time.UnixMilli(int64(*event.EndTime))
		option.Logger.Debugf("ASR Delta: %s startTime: %s endTime: %s", event.Text, startTime.String(), endTime.String())
		if option.BreakOnVad {
			return
		}
		if err := client.Interrupt(true); err != nil {
			option.Logger.Warnf("Failed to interrupt TTS: %v", err)
		}
	}
	client.OnSpeaking = func(event rustpbxgo.SpeakingEvent) {
		option.Logger.Infof("OnSpeaking")
		if !option.BreakOnVad {
			return
		}
		option.Logger.Infof("Interrupting TTS")
		if err := client.Interrupt(true); err != nil {
			option.Logger.Warnf("Failed to interrupt TTS: %v", err)
		}
	}
	client.OnInterruption = func(event rustpbxgo.InterruptionEvent) {
		fields := logrus.Fields{
			"current":       event.Current,
			"totalDuration": event.TotalDuration,
		}
		if event.Subtitle != nil {
			fields["subtitle"] = *event.Subtitle
		}
		if event.Position != nil {
			fields["position"] = *event.Position
		}
		option.Logger.WithFields(fields).Info("OnInterruption")
	}
	return client
}

func main() {
	// Parse command line flags
	var endpoint string = "wss://rustpbx.com"
	var codec string = "g722"
	var openaiKey string = ""
	var openaiModel string = ""
	var openaiEndpoint string = ""
	var systemPrompt string = "You are a helpful assistant. Provide concise responses. Use 'hangup' tool when the conversation is complete."
	var breakOnVad bool = false
	var speaker string = "601003"
	var callWithSip bool = false
	var record bool = false
	var ttsProvider string = "tencent"
	var asrProvider string = "tencent"
	var caller string = ""
	var callee string = ""
	var username string = ""
	var password string = ""
	var realm string = ""
	var asrEndpoint string = ""
	var asrAppID string = ""
	var asrSecretID string = ""
	var asrSecretKey string = ""
	var asrModelType string = ""
	var ttsEndpoint string = ""
	var ttsAppID string = ""
	var ttsSecretID string = ""
	var ttsSecretKey string = ""
	var vadModel string = "silero"
	var vadEndpoint string = ""
	var vadSecretKey string = ""
	var webhookAddr string = ""
	var webhookPrefix string = "/webhook"
	var eouType string = ""
	var eouEndpoint string = ""
	var silenceTimeout uint = 5000
	var referCallee string = ""
	var forceRefer bool = false
	var greeting string = "Hello, how can I help you?"
	var level = "info"
	var streamingTTS bool = false
	var ringtone string = ""
	var maxCallTime uint = 0
	var subscribe bool = false
	flag.StringVar(&level, "log-level", level, "Log level: debug, info, warn, error")
	flag.StringVar(&endpoint, "endpoint", endpoint, "Endpoint to connect to")
	flag.StringVar(&codec, "codec", codec, "Codec to use: g722, pcmu, pcma")
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
	flag.StringVar(&caller, "caller", caller, "Caller to use")
	flag.StringVar(&callee, "callee", callee, "Callee to use")
	flag.StringVar(&username, "username", username, "SIP username to use")
	flag.StringVar(&password, "password", password, "SIP password to use")
	flag.StringVar(&realm, "realm", realm, "SIP realm to use")
	flag.StringVar(&asrEndpoint, "asr-endpoint", asrEndpoint, "ASR endpoint to use")
	flag.StringVar(&asrModelType, "asr-model-type", asrModelType, "ASR model type to use")
	flag.StringVar(&asrAppID, "asr-app-id", asrAppID, "ASR app id to use")
	flag.StringVar(&asrSecretID, "asr-secret-id", asrSecretID, "ASR secret id to use")
	flag.StringVar(&asrSecretKey, "asr-secret-key", asrSecretKey, "ASR secret key to use")
	flag.StringVar(&ttsEndpoint, "tts-endpoint", ttsEndpoint, "TTS endpoint to use")
	flag.StringVar(&ttsAppID, "tts-app-id", ttsAppID, "TTS app id to use")
	flag.StringVar(&ttsSecretID, "tts-secret-id", ttsSecretID, "TTS secret id to use")
	flag.StringVar(&ttsSecretKey, "tts-secret-key", ttsSecretKey, "TTS secret key to use")
	flag.BoolVar(&streamingTTS, "streaming", streamingTTS, "Use streaming TTS if supported")
	flag.StringVar(&vadModel, "vad-model", vadModel, "VAD model to use")
	flag.StringVar(&vadEndpoint, "vad-endpoint", vadEndpoint, "VAD endpoint to use")
	flag.StringVar(&vadSecretKey, "vad-secret-key", vadSecretKey, "VAD secret key to use")
	flag.StringVar(&webhookAddr, "webhook-addr", webhookAddr, "Webhook address to use")
	flag.StringVar(&webhookPrefix, "webhook-prefix", webhookPrefix, "Webhook prefix to use")
	flag.StringVar(&eouType, "eou-type", eouType, "EOU type to use")
	flag.StringVar(&eouEndpoint, "eou-endpoint", eouEndpoint, "EOU endpoint to use")
	flag.UintVar(&silenceTimeout, "silence-timeout", silenceTimeout, "VAD silence timeout in milliseconds")
	flag.StringVar(&referCallee, "refer", referCallee, "Refer callee for SIP REFER")
	flag.BoolVar(&forceRefer, "force-refer", forceRefer, "Force refer to callee")
	flag.StringVar(&greeting, "greeting", greeting, "Initial greeting message")
	flag.StringVar(&ringtone, "ringtone", ringtone, "Ringtone file to play when ringing")
	flag.UintVar(&maxCallTime, "max-call-time", maxCallTime, "Maximum call time in seconds (0 for unlimited)")
	flag.BoolVar(&subscribe, "subscribe", subscribe, "Subscribe to real-time audio")

	flag.Parse()
	u, err := url.Parse(endpoint)
	if err != nil {
		fmt.Printf("Failed to parse endpoint: %v", err)
		os.Exit(1)
	}

	if referCallee != "" && caller == "" {
		fmt.Printf("Caller must be set when refer is used")
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
			openaiModel = "qwen3-14b"
		}
	}
	// Create logger
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	if lel, err := logrus.ParseLevel(level); err == nil {
		logger.SetLevel(lel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan bool)
	option := CreateClientOption{
		Endpoint:       endpoint,
		Logger:         logger,
		LLMHandler:     nil,
		OpenaiKey:      openaiKey,
		OpenaiEndpoint: openaiEndpoint,
		OpenaiModel:    openaiModel,
		SystemPrompt:   systemPrompt,
		Greeting:       greeting,
		SigChan:        sigChan,
		BreakOnVad:     breakOnVad,
		StreamingTTS:   streamingTTS,
	}
	var recorder *rustpbxgo.RecorderOption
	if record {
		recorder = &rustpbxgo.RecorderOption{
			Samplerate: 16000,
		}
	}
	var asrOption *rustpbxgo.ASROption
	if asrProvider != "" {
		asrOption = &rustpbxgo.ASROption{
			Provider:  asrProvider,
			Endpoint:  asrEndpoint,
			AppID:     asrAppID,
			SecretID:  asrSecretID,
			SecretKey: asrSecretKey,
			ModelType: asrModelType,
		}
	}
	var vadOption *rustpbxgo.VADOption
	if vadModel != "" {
		vadOption = &rustpbxgo.VADOption{
			Type:           vadModel,
			Endpoint:       vadEndpoint,
			SecretKey:      vadSecretKey,
			SilenceTimeout: silenceTimeout,
		}
	}

	callOption := rustpbxgo.CallOption{
		Recorder: recorder,
		Denoise:  true,
		VAD:      vadOption,
		ASR:      asrOption,
		TTS: &rustpbxgo.TTSOption{
			Provider:           ttsProvider,
			Speaker:            speaker,
			Endpoint:           ttsEndpoint,
			AppID:              ttsAppID,
			SecretID:           ttsSecretID,
			SecretKey:          ttsSecretKey,
			MaxConcurrentTasks: 3,
		},
	}
	if eouType != "" {
		callOption.Eou = &rustpbxgo.EouOption{
			Type:     eouType,
			Endpoint: eouEndpoint,
		}
	}
	callOption.Subscribe = subscribe
	if callWithSip {
		callOption.Caller = caller
		callOption.Callee = callee
		if callOption.Caller == "" || callOption.Callee == "" {
			logger.Fatalf("caller and callee must be set")
		}
		sipOption := rustpbxgo.SipOption{
			Username: username,
			Password: password,
			Realm:    realm,
		}
		callOption.Sip = &sipOption
	}
	option.Ringtone = ringtone
	option.CallOption = callOption
	option.ReferCallee = referCallee
	option.ReferCaller = caller
	option.MaxCallTime = maxCallTime
	if webhookAddr != "" {
		serveWebhook(ctx, option, webhookAddr, webhookPrefix)
		return
	}

	// Create media handler
	mediaHandler, err := NewMediaHandler(ctx, logger)
	if err != nil {
		logger.Fatalf("Failed to create media handler: %v", err)
	}
	defer mediaHandler.Stop()

	callType := "webrtc"
	if callWithSip {
		callType = "sip"
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
		callOption.Offer = localSdp
	}
	client := createClient(ctx, option, "")
	// Connect to server
	err = client.Connect(callType)
	if err != nil {
		logger.Fatalf("Failed to connect to server: %v", err)
	}
	defer client.Shutdown()
	// Start the call
	answer, err := client.Invite(ctx, callOption)
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
	if forceRefer {
		option.Logger.Infof("Forcing refer to target: %s => %s", option.ReferCaller, option.ReferCallee)
		if err := client.Refer(option.ReferCaller, option.ReferCallee, nil); err != nil {
			option.Logger.Errorf("Failed to refer: %v", err)
			return
		}
	} else {
		client.TTS(greeting, "", "1", true, false, nil, nil, false)
	}

	if maxCallTime > 0 {
		time.Sleep(time.Duration(maxCallTime) * time.Second)
		logger.Infof("Max call time reached, hanging up...")
		client.Hangup("max call time reached")
	}
	// Wait for signal to shutdown
	<-sigChan
	fmt.Println("Shutting down...")
}
