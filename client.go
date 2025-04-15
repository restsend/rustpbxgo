package rustpbxgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type OnEvent func(event string, payload string)
type OnIncoming func(event IncomingEvent)
type OnAnswer func(event AnswerEvent)
type OnReject func(event RejectEvent)
type OnHangup func(event HangupEvent)
type OnRinging func(event RingingEvent)
type OnAnswerMachineDetection func(event AnswerMachineDetectionEvent)
type OnSpeaking func(event SpeakingEvent)
type OnSilence func(event SilenceEvent)
type OnNoisy func(event NoisyEvent)
type OnDTMF func(event DTMFEvent)
type OnTrackStart func(event TrackStartEvent)
type OnTrackEnd func(event TrackEndEvent)
type OnInterruption func(event InterruptionEvent)
type OnAsrFinal func(event AsrFinalEvent)
type OnAsrDelta func(event AsrDeltaEvent)
type OnLLMFinal func(event LLMFinalEvent)
type OnLLMDelta func(event LLMDeltaEvent)
type OnMetrics func(event MetricsEvent)
type OnError func(event ErrorEvent)

type Client struct {
	ctx                      context.Context
	cancel                   context.CancelFunc
	endpoint                 string
	conn                     *websocket.Conn
	logger                   *logrus.Logger
	id                       string
	onAnswer                 OnAnswer
	OnEvent                  OnEvent
	OnIncoming               OnIncoming
	OnReject                 OnReject
	OnHangup                 OnHangup
	OnRinging                OnRinging
	OnAnswerMachineDetection OnAnswerMachineDetection
	OnSpeaking               OnSpeaking
	OnSilence                OnSilence
	OnNoisy                  OnNoisy
	OnDTMF                   OnDTMF
	OnTrackStart             OnTrackStart
	OnTrackEnd               OnTrackEnd
	OnInterruption           OnInterruption
	OnAsrFinal               OnAsrFinal
	OnAsrDelta               OnAsrDelta
	OnLLMFinal               OnLLMFinal
	OnLLMDelta               OnLLMDelta
	OnMetrics                OnMetrics
	OnError                  OnError
}

type event struct {
	Event string `json:"event"`
}

type IncomingEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Caller    string `json:"caller"`
	Callee    string `json:"callee"`
	Sdp       string `json:"sdp"`
}
type AnswerEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Sdp       string `json:"sdp"`
}

type RejectEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Reason    string `json:"reason"`
}

type RingingEvent struct {
	TrackID    string `json:"trackId"`
	Timestamp  uint64 `json:"timestamp"`
	EarlyMedia bool   `json:"earlyMedia"`
}

type HangupEvent struct {
	Timestamp uint64 `json:"timestamp"`
	Reason    string `json:"reason"`
	Initiator string `json:"initiator"`
}

type AnswerMachineDetectionEvent struct {
	Timestamp uint64 `json:"timestamp"`
	StartTime uint64 `json:"startTime"`
	EndTime   uint64 `json:"endTime"`
	Text      string `json:"text"`
}

type SpeakingEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	StartTime uint64 `json:"startTime"`
}

type SilenceEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	StartTime uint64 `json:"startTime"`
	Duration  uint64 `json:"duration"`
}

type NoisyEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	StartTime uint64 `json:"startTime"`
	Type      string `json:"type"`
}

type DTMFEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Digit     string `json:"digit"`
}

type TrackStartEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
}

type TrackEndEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
}

type InterruptionEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Position  uint64 `json:"position"`
}

type AsrFinalEvent struct {
	TrackID   string  `json:"trackId"`
	Timestamp uint64  `json:"timestamp"`
	Index     uint32  `json:"index"`
	StartTime *uint32 `json:"startTime,omitempty"`
	EndTime   *uint32 `json:"endTime,omitempty"`
	Text      string  `json:"text"`
}

type AsrDeltaEvent struct {
	TrackID   string  `json:"trackId"`
	Index     uint32  `json:"index"`
	Timestamp uint64  `json:"timestamp"`
	StartTime *uint32 `json:"startTime,omitempty"`
	EndTime   *uint32 `json:"endTime,omitempty"`
	Text      string  `json:"text"`
}

type LLMFinalEvent struct {
	Timestamp uint64 `json:"timestamp"`
	Text      string `json:"text"`
}

type LLMDeltaEvent struct {
	Timestamp uint64 `json:"timestamp"`
	Word      string `json:"word"`
}

type MetricsEvent struct {
	Timestamp uint64         `json:"timestamp"`
	Key       string         `json:"key"`
	Duration  uint32         `json:"duration"`
	Data      map[string]any `json:"data"`
}

type ErrorEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Sender    string `json:"sender"`
	Error     string `json:"error"`
}

// Command represents WebSocket commands to be sent to the server
type Command struct {
	Command string `json:"command"`
}

// InviteCommand initiates a call
type InviteCommand struct {
	Command string       `json:"command"`
	Options StreamOption `json:"options"`
}

// AcceptCommand accepts an incoming call
type AcceptCommand struct {
	Command string       `json:"command"`
	Options StreamOption `json:"options"`
}

// RejectCommand rejects an incoming call
type RejectCommand struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

// CandidateCommand sends ICE candidates
type CandidateCommand struct {
	Command    string   `json:"command"`
	Candidates []string `json:"candidates"`
}

// TtsCommand sends text to be synthesized
type TtsCommand struct {
	Command    string `json:"command"`
	Text       string `json:"text"`
	Speaker    string `json:"speaker,omitempty"`
	PlayID     string `json:"playId,omitempty"`
	AutoHangup bool   `json:"autoHangup,omitempty"`
}

// PlayCommand plays audio from a URL
type PlayCommand struct {
	Command    string `json:"command"`
	URL        string `json:"url"`
	AutoHangup bool   `json:"autoHangup,omitempty"`
}

// InterruptCommand interrupts current playback
type InterruptCommand struct {
	Command string `json:"command"`
}

// PauseCommand pauses current playback
type PauseCommand struct {
	Command string `json:"command"`
}

// ResumeCommand resumes paused playback
type ResumeCommand struct {
	Command string `json:"command"`
}

// HangupCommand ends the call
type HangupCommand struct {
	Command string `json:"command"`
}

// ReferCommand transfers the call
type ReferCommand struct {
	Command string       `json:"command"`
	Target  string       `json:"target"`
	Options *ReferOption `json:"options,omitempty"`
}

// MuteCommand mutes a track
type MuteCommand struct {
	Command string  `json:"command"`
	TrackID *string `json:"trackId,omitempty"`
}

// UnmuteCommand unmutes a track
type UnmuteCommand struct {
	Command string  `json:"command"`
	TrackID *string `json:"trackId,omitempty"`
}

type RecorderOption struct {
	Samplerate int `json:"samplerate,omitempty"`
	Ptime      int `json:"ptime,omitempty"`
}

type VADOption struct {
	Type                  string  `json:"type,omitempty" comment:"vad type, silero|webrtc"`
	Samplerate            uint32  `json:"samplerate,omitempty" comment:"vad samplerate, 16000|48000"`
	SpeechPadding         uint64  `json:"speechPadding,omitempty" comment:"vad speech padding, 120"`
	SilencePadding        uint64  `json:"silencePadding,omitempty" comment:"vad silence padding, 200"`
	Ratio                 float32 `json:"ratio,omitempty" comment:"vad ratio, 0.5|0.7"`
	VoiceThreshold        float32 `json:"voiceThreshold,omitempty" comment:"vad voice threshold, 0.5"`
	MaxBufferDurationSecs uint64  `json:"maxBufferDurationSecs,omitempty"`
}

type ASROption struct {
	Provider   string `json:"provider,omitempty" comment:"asr provider, tencent|aliyun"`
	Model      string `json:"model,omitempty"`
	Language   string `json:"language,omitempty"`
	AppID      string `json:"appId,omitempty"`
	SecretID   string `json:"secretId,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
	ModelType  string `json:"modelType"`
	BufferSize int    `json:"bufferSize"`
	SampleRate uint32 `json:"sampleRate"`
}

type TTSOption struct {
	Samplerate int32   `json:"samplerate,omitempty" comment:"tts samplerate, 16000|48000"`
	Provider   string  `json:"provider,omitempty" comment:"tts provider, tencent|aliyun"`
	Speed      float32 `json:"speed,omitempty"`
	AppID      string  `json:"appId,omitempty"`
	SecretID   string  `json:"secretId,omitempty"`
	SecretKey  string  `json:"secretKey,omitempty"`
	Volume     int32   `json:"volume,omitempty"`
	Speaker    string  `json:"speaker,omitempty"`
	Codec      string  `json:"codec,omitempty"`
	Subtitle   bool    `json:"subtitle,omitempty"`
	Emotion    string  `json:"emotion,omitempty"`
}

// StreamOption represents options for stream commands
type StreamOption struct {
	Denoise          bool            `json:"denoise,omitempty"`
	Offer            string          `json:"offer,omitempty"`
	Callee           string          `json:"callee,omitempty"`
	Caller           string          `json:"caller,omitempty"`
	Recorder         *RecorderOption `json:"recorder,omitempty"`
	VAD              *VADOption      `json:"vad,omitempty"`
	ASR              *ASROption      `json:"asr,omitempty"`
	TTS              *TTSOption      `json:"tts,omitempty"`
	HandshakeTimeout string          `json:"handshakeTimeout,omitempty"`
	EnableIPv6       bool            `json:"enableIpv6,omitempty"`
}

// ReferOption represents options for refer command
type ReferOption struct {
	Bypass      bool   `json:"bypass,omitempty"`
	Timeout     uint32 `json:"timeout,omitempty"`
	MusicOnHold string `json:"moh,omitempty"`
	AutoHangup  bool   `json:"autoHangup,omitempty"`
}

type ClientOption func(*Client)

func NewClient(endpoint string, opts ...ClientOption) *Client {
	c := &Client{
		endpoint: endpoint,
		logger:   logrus.StandardLogger(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithLogger(logger *logrus.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

func WithContext(ctx context.Context) ClientOption {
	return func(c *Client) {
		c.ctx, c.cancel = context.WithCancel(ctx)
	}
}

func WithID(id string) ClientOption {
	return func(c *Client) {
		c.id = id
	}
}

func (c *Client) Connect() error {
	if c.cancel == nil {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	}
	url := c.endpoint
	url += "/call/webrtc"
	if c.id != "" {
		url = fmt.Sprintf("%s?id=%s", url, c.id)
	}
	var err error
	c.conn, _, err = websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	eventChan := make(chan []byte)
	go func() {
		for {
			mt, message, err := c.conn.ReadMessage()
			if err != nil {
				c.logger.Errorf("Error reading message: %v", err)
				return
			}
			if mt != websocket.TextMessage {
				c.logger.Debugf("Received non-text message: %v", mt)
				continue
			}
			select {
			case eventChan <- message:
			case <-c.ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer close(eventChan)
		defer c.conn.Close()

		for {
			select {
			case <-c.ctx.Done():
				return
			case message, ok := <-eventChan:
				if !ok {
					return
				}
				c.processEvent(message)
			}
		}
	}()
	return nil
}

func (c *Client) processEvent(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("Panic in processEvent: %v %s", r, string(message))
		}
	}()
	var ev event
	err := json.Unmarshal(message, &ev)
	if err != nil {
		c.logger.Errorf("Error unmarshalling event: %v", err)
		return
	}
	c.logger.Debugf("Received event: %s %s", ev.Event, string(message))
	if c.OnEvent != nil {
		c.OnEvent(ev.Event, string(message))
	}
	switch ev.Event {
	case "incoming":
		var event IncomingEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling incoming event: %v", err)
			return
		}
		if c.OnIncoming != nil {
			c.OnIncoming(event)
		}
	case "answer":
		var event AnswerEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling answer event: %v", err)
			return
		}
		if c.onAnswer != nil {
			c.onAnswer(event)
		}
	case "reject":
		var event RejectEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling reject event: %v", err)
			return
		}
		if c.OnReject != nil {
			c.OnReject(event)
		}
	case "ringing":
		var event RingingEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling ringing event: %v", err)
			return
		}
		if c.OnRinging != nil {
			c.OnRinging(event)
		}
	case "hangup":
		var event HangupEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling hangup event: %v", err)
			return
		}
		if c.OnHangup != nil {
			c.OnHangup(event)
		}
	case "answerMachineDetection":
		var event AnswerMachineDetectionEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling answerMachineDetection event: %v", err)
			return
		}
		if c.OnAnswerMachineDetection != nil {
			c.OnAnswerMachineDetection(event)
		}
	case "speaking":
		var event SpeakingEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling speaking event: %v", err)
			return
		}
		if c.OnSpeaking != nil {
			c.OnSpeaking(event)
		}
	case "silence":
		var event SilenceEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling silence event: %v", err)
			return
		}
		if c.OnSilence != nil {
			c.OnSilence(event)
		}
	case "noisy":
		var event NoisyEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling noisy event: %v", err)
			return
		}
		if c.OnNoisy != nil {
			c.OnNoisy(event)
		}
	case "dtmf":
		var event DTMFEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling dtmf event: %v", err)
			return
		}
		if c.OnDTMF != nil {
			c.OnDTMF(event)
		}
	case "trackStart":
		var event TrackStartEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling trackStart event: %v", err)
			return
		}
		if c.OnTrackStart != nil {
			c.OnTrackStart(event)
		}
	case "trackEnd":
		var event TrackEndEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling trackEnd event: %v", err)
			return
		}
		if c.OnTrackEnd != nil {
			c.OnTrackEnd(event)
		}
	case "interruption":
		var event InterruptionEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling interruption event: %v", err)
			return
		}
		if c.OnInterruption != nil {
			c.OnInterruption(event)
		}
	case "asrFinal":
		var event AsrFinalEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling asrFinal event: %v", err)
			return
		}
		if c.OnAsrFinal != nil {
			c.OnAsrFinal(event)
		}
	case "asrDelta":
		var event AsrDeltaEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling asrDelta event: %v", err)
			return
		}
		if c.OnAsrDelta != nil {
			c.OnAsrDelta(event)
		}
	case "llmFinal":
		var event LLMFinalEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling llmFinal event: %v", err)
			return
		}
		if c.OnLLMFinal != nil {
			c.OnLLMFinal(event)
		}
	case "llmDelta":
		var event LLMDeltaEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling llmDelta event: %v", err)
			return
		}
		if c.OnLLMDelta != nil {
			c.OnLLMDelta(event)
		}
	case "metrics":
		var event MetricsEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling metrics event: %v", err)
			return
		}
		if c.OnMetrics != nil {
			c.OnMetrics(event)
		}
	case "error":
		var event ErrorEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling error event: %v", err)
			return
		}
		if c.OnError != nil {
			c.OnError(event)
		}
	default:
		c.logger.Debugf("Unhandled event type: %s", ev.Event)
	}
}
func (c *Client) Shutdown() error {
	c.cancel()
	return nil
}

// It returns a channel that will receive the answer event
func (c *Client) Invite(ctx context.Context, options StreamOption) (*AnswerEvent, error) {
	onAnswer := c.onAnswer
	ch := make(chan AnswerEvent, 1)
	c.onAnswer = func(event AnswerEvent) {
		ch <- event
	}
	defer func() {
		c.onAnswer = onAnswer
	}()
	cmd := InviteCommand{
		Command: "invite",
		Options: options,
	}
	err := c.sendCommand(cmd)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case event := <-ch:
		return &event, nil
	}
}

// Accept sends an accept command to accept an incoming call
func (c *Client) Accept(options StreamOption) error {
	cmd := AcceptCommand{
		Command: "accept",
		Options: options,
	}
	return c.sendCommand(cmd)
}

// Reject sends a reject command to reject an incoming call
func (c *Client) Reject(reason string) error {
	cmd := RejectCommand{
		Command: "reject",
		Reason:  reason,
	}
	return c.sendCommand(cmd)
}

// SendCandidates sends ICE candidates
func (c *Client) SendCandidates(candidates []string) error {
	cmd := CandidateCommand{
		Command:    "candidate",
		Candidates: candidates,
	}
	return c.sendCommand(cmd)
}

// TTS sends a text-to-speech command
func (c *Client) TTS(text string, speaker string, playID string, autoHangup bool) error {
	cmd := TtsCommand{
		Command:    "tts",
		Text:       text,
		Speaker:    speaker,
		PlayID:     playID,
		AutoHangup: autoHangup,
	}
	return c.sendCommand(cmd)
}

// Play sends a command to play audio from a URL
func (c *Client) Play(url string, autoHangup bool) error {
	cmd := PlayCommand{
		Command:    "play",
		URL:        url,
		AutoHangup: autoHangup,
	}
	return c.sendCommand(cmd)
}

// Interrupt sends a command to interrupt current playback
func (c *Client) Interrupt() error {
	cmd := InterruptCommand{
		Command: "interrupt",
	}
	return c.sendCommand(cmd)
}

// Pause sends a command to pause current playback
func (c *Client) Pause() error {
	cmd := PauseCommand{
		Command: "pause",
	}
	return c.sendCommand(cmd)
}

// Resume sends a command to resume paused playback
func (c *Client) Resume() error {
	cmd := ResumeCommand{
		Command: "resume",
	}
	return c.sendCommand(cmd)
}

// Hangup sends a command to end the call
func (c *Client) Hangup() error {
	cmd := HangupCommand{
		Command: "hangup",
	}
	return c.sendCommand(cmd)
}

// Refer sends a command to transfer the call
func (c *Client) Refer(target string, options *ReferOption) error {
	cmd := ReferCommand{
		Command: "refer",
		Target:  target,
		Options: options,
	}
	return c.sendCommand(cmd)
}

// Mute sends a command to mute a track
func (c *Client) Mute(trackID *string) error {
	cmd := MuteCommand{
		Command: "mute",
		TrackID: trackID,
	}
	return c.sendCommand(cmd)
}

// Unmute sends a command to unmute a track
func (c *Client) Unmute(trackID *string) error {
	cmd := UnmuteCommand{
		Command: "unmute",
		TrackID: trackID,
	}
	return c.sendCommand(cmd)
}

// sendCommand sends a command to the server
func (c *Client) sendCommand(cmd any) error {
	// Implementation will be added when the WebSocket connection is implemented
	if c.conn == nil {
		return errors.New("client not initialized")
	}
	c.logger.WithFields(logrus.Fields{
		"command": cmd,
	}).Debug("Sending command")
	return c.conn.WriteJSON(cmd)
}
