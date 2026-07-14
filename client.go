package rustpbxgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type OnEvent func(event string, payload string)
type OnIncoming func(event IncomingEvent)
type OnAnswer func(event AnswerEvent)
type OnMediaReady func(event MediaReadyEvent)
type OnReject func(event RejectEvent)
type OnHangup func(event HangupEvent)
type OnRinging func(event RingingEvent)
type OnAnswerMachineDetection func(event AnswerMachineDetectionEvent)
type OnSpeaking func(event SpeakingEvent)
type OnSilence func(event SilenceEvent)
type OnDTMF func(event DTMFEvent)
type OnHold func(event HoldEvent)
type OnTransferRequest func(event TransferRequestEvent)
type OnMessage func(event MessageEvent)
type OnTrackStart func(event TrackStartEvent)
type OnTrackEnd func(event TrackEndEvent)
type OnInterruption func(event InterruptionEvent)
type OnAsrFinal func(event AsrFinalEvent)
type OnAsrDelta func(event AsrDeltaEvent)
type OnLLMFinal func(event LLMFinalEvent)
type OnLLMDelta func(event LLMDeltaEvent)
type OnFunctionCall func(event FunctionCallEvent)
type OnMetrics func(event MetricsEvent)
type OnError func(event ErrorEvent)
type OnClose func(reason string)
type OnAddHistory func(event AddHistoryEvent)
type OnEou func(event EouEvent)
type OnInactivity func(event InactivityEvent)
type OnOther func(event OtherEvent)
type OnInterrupt func(event InterruptEvent)
type OnPing func(event PingEvent)
type OnBinary func(trackIdx int, data []byte)
type OnCustom func(event CustomEvent)
type OnRingbackState func(event RingbackStateEvent)

type Client struct {
	ctx                      context.Context
	cancel                   context.CancelFunc
	eventChan                chan []byte
	cmdChan                  chan any
	endpoint                 string
	conn                     *websocket.Conn
	logger                   *logrus.Logger
	id                       string
	dumpEvents               bool
	onAnswer                 OnAnswer
	OnClose                  OnClose
	OnEvent                  OnEvent
	OnIncoming               OnIncoming
	OnMediaReady             OnMediaReady
	OnReject                 OnReject
	OnHangup                 OnHangup
	OnRinging                OnRinging
	OnAnswerMachineDetection OnAnswerMachineDetection
	OnSpeaking               OnSpeaking
	OnSilence                OnSilence
	OnDTMF                   OnDTMF
	OnHold                   OnHold
	OnTransferRequest        OnTransferRequest
	OnMessage                OnMessage
	OnTrackStart             OnTrackStart
	OnTrackEnd               OnTrackEnd
	OnInterruption           OnInterruption
	OnAsrFinal               OnAsrFinal
	OnAsrDelta               OnAsrDelta
	OnLLMFinal               OnLLMFinal
	OnLLMDelta               OnLLMDelta
	OnFunctionCall           OnFunctionCall
	OnMetrics                OnMetrics
	OnError                  OnError
	OnAddHistory             OnAddHistory
	OnEou                    OnEou
	OnInactivity             OnInactivity
	OnOther                  OnOther
	OnInterrupt              OnInterrupt
	OnCustom                 OnCustom
	OnRingbackState          OnRingbackState
	OnPing                   OnPing
	OnBinary                 OnBinary
}

type event struct {
	Event string `json:"event"`
}

// --- Call lifecycle events ---

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
	Refer     *bool  `json:"refer,omitempty"`
}

type MediaReadyEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
}

type RejectEvent struct {
	TrackID   string  `json:"trackId"`
	Timestamp uint64  `json:"timestamp"`
	Reason    string  `json:"reason"`
	Code      *uint32 `json:"code,omitempty"`
	Refer     *bool   `json:"refer,omitempty"`
}

type RingingEvent struct {
	TrackID    string `json:"trackId"`
	Timestamp  uint64 `json:"timestamp"`
	EarlyMedia bool   `json:"earlyMedia"`
	Refer      *bool  `json:"refer,omitempty"`
}

type HangupEventAttendee struct {
	Username string `json:"username"`
	Realm    string `json:"realm"`
	Source   string `json:"source"`
}

type HangupEvent struct {
	TrackID     string               `json:"trackId"`
	Timestamp   uint64               `json:"timestamp"`
	Reason      *string              `json:"reason,omitempty"`
	Initiator   *string              `json:"initiator,omitempty"`
	StartTime   string               `json:"startTime,omitempty"`
	HangupTime  string               `json:"hangupTime,omitempty"`
	AnswerTime  *string              `json:"answerTime,omitempty"`
	RingingTime *string              `json:"ringingTime,omitempty"`
	From        *HangupEventAttendee `json:"from,omitempty"`
	To          *HangupEventAttendee `json:"to,omitempty"`
	Extra       map[string]any       `json:"extra,omitempty"`
	Refer       *bool                `json:"refer,omitempty"`
}

type AnswerMachineDetectionEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	StartTime uint64 `json:"startTime"`
	EndTime   uint64 `json:"endTime"`
	Text      string `json:"text"`
}

// --- Media / audio events ---

type SpeakingEvent struct {
	TrackID    string   `json:"trackId"`
	Timestamp  uint64   `json:"timestamp"`
	StartTime  uint64   `json:"startTime"`
	IsFiller   *bool    `json:"isFiller,omitempty"`
	Confidence *float32 `json:"confidence,omitempty"`
	Refer      *bool    `json:"refer,omitempty"`
}

type SilenceEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	StartTime uint64 `json:"startTime"`
	Duration  uint64 `json:"duration"`
	Refer     *bool  `json:"refer,omitempty"`
}

type DTMFEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Digit     string `json:"digit"`
	Refer     *bool  `json:"refer,omitempty"`
}

type HoldEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	OnHold    bool   `json:"onHold"`
	Refer     *bool  `json:"refer,omitempty"`
}

// --- Transfer events ---

type TransferRequestEvent struct {
	TrackID     string  `json:"trackId"`
	Timestamp   uint64  `json:"timestamp"`
	ReferTo     string  `json:"referTo"`
	ReferredBy  *string `json:"referredBy,omitempty"`
	Refer       *bool   `json:"refer,omitempty"`
}

type MessageEvent struct {
	TrackID     string  `json:"trackId"`
	Timestamp   uint64  `json:"timestamp"`
	Body        string  `json:"body"`
	ContentType *string `json:"contentType,omitempty"`
	Refer       *bool   `json:"refer,omitempty"`
}

// --- Track events ---

type TrackStartEvent struct {
	TrackID   string  `json:"trackId"`
	Timestamp uint64  `json:"timestamp"`
	PlayId    *string `json:"playId,omitempty"`
}

type TrackEndEvent struct {
	TrackID   string  `json:"trackId"`
	Timestamp uint64  `json:"timestamp"`
	Duration  uint64  `json:"duration"`
	Ssrc      uint32  `json:"ssrc"`
	PlayId    *string `json:"playId,omitempty"`
}

type InterruptionEvent struct {
	TrackID       string  `json:"trackId"`
	Timestamp     uint64  `json:"timestamp"`
	PlayID        *string `json:"playId,omitempty"`
	Subtitle      *string `json:"subtitle,omitempty"`
	Position      *uint32 `json:"position,omitempty"`
	TotalDuration uint32  `json:"totalDuration"`
	Current       uint32  `json:"current"`
}

// --- ASR events ---

type AsrFinalEvent struct {
	TrackID    string   `json:"trackId"`
	Timestamp  uint64   `json:"timestamp"`
	Index      uint32   `json:"index"`
	StartTime  *uint64  `json:"startTime,omitempty"`
	EndTime    *uint64  `json:"endTime,omitempty"`
	Text       string   `json:"text"`
	IsFiller   *bool    `json:"isFiller,omitempty"`
	Confidence *float32 `json:"confidence,omitempty"`
	TaskID     *string  `json:"taskId,omitempty"`
	Refer      *bool    `json:"refer,omitempty"`
}

type AsrDeltaEvent struct {
	TrackID    string   `json:"trackId"`
	Index      uint32   `json:"index"`
	Timestamp  uint64   `json:"timestamp"`
	StartTime  *uint64  `json:"startTime,omitempty"`
	EndTime    *uint64  `json:"endTime,omitempty"`
	Text       string   `json:"text"`
	IsFiller   *bool    `json:"isFiller,omitempty"`
	Confidence *float32 `json:"confidence,omitempty"`
	TaskID     *string  `json:"taskId,omitempty"`
	Refer      *bool    `json:"refer,omitempty"`
}

// --- LLM events ---

type LLMFinalEvent struct {
	Timestamp uint64 `json:"timestamp"`
	Text      string `json:"text"`
}

type LLMDeltaEvent struct {
	Timestamp uint64 `json:"timestamp"`
	Word      string `json:"word"`
}

type FunctionCallEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	CallID    string `json:"callId"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// --- System events ---

type MetricsEvent struct {
	Timestamp uint64         `json:"timestamp"`
	Key       string         `json:"key"`
	Duration  uint32         `json:"duration"`
	Data      map[string]any `json:"data"`
}

type ErrorEvent struct {
	TrackID   string  `json:"trackId"`
	Timestamp uint64  `json:"timestamp"`
	Sender    string  `json:"sender"`
	Error     string  `json:"error"`
	Code      *uint32 `json:"code,omitempty"`
}

type EouEvent struct {
	TrackID        string  `json:"trackId"`
	Timestamp      uint64  `json:"timestamp"`
	Complete       bool    `json:"complete"`
	InterruptPoint string  `json:"interruptPoint,omitempty"`
	Text           *string `json:"text,omitempty"`
	Refer          *bool   `json:"refer,omitempty"`
}

type InactivityEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
}

type AddHistoryEvent struct {
	Sender    *string `json:"sender,omitempty"`
	Timestamp uint64  `json:"timestamp"`
	Speaker   string  `json:"speaker"`
	Text      string  `json:"text"`
}

type OtherEvent struct {
	TrackID   string            `json:"trackId"`
	Timestamp uint64            `json:"timestamp"`
	Sender    string            `json:"sender"`
	Extra     map[string]string `json:"extra,omitempty"`
}

type BinaryEvent struct {
	TrackID   string `json:"trackId"`
	Timestamp uint64 `json:"timestamp"`
	Data      []byte `json:"data"`
}

type CustomEvent struct {
	Timestamp uint64          `json:"timestamp"`
	Sender    *string         `json:"sender,omitempty"`
	Data      json.RawMessage `json:"data"`
}

type InterruptEvent struct {
	Receiver *string `json:"receiver,omitempty"`
}

type PingEvent struct {
	Timestamp uint64  `json:"timestamp"`
	Payload   *string `json:"payload,omitempty"`
}

type RingbackStateEvent struct {
	TrackID        string   `json:"trackId"`
	Timestamp      uint64   `json:"timestamp"`
	State          string   `json:"state"`          // Current line state: "ringing", "human_voice", "busy_tone", "silence", etc.
	StateIndex     uint32   `json:"stateIndex"`     // Class index from the TelcoClassifier (3=ringing, 9=human_voice, ...)
	Confidence     float32  `json:"confidence"`     // Confidence of the prediction
	PrevState      *string  `json:"prevState,omitempty"`
	PrevConfidence *float32 `json:"prevConfidence,omitempty"`
	Refer          *bool    `json:"refer,omitempty"`
	IsFinal        bool     `json:"isFinal"`
}

// Command represents WebSocket commands to be sent to the server
type Command struct {
	Command string `json:"command"`
}

// InviteCommand initiates a call
type InviteCommand struct {
	Command string     `json:"command"`
	Option  CallOption `json:"option"`
}

// AcceptCommand accepts an incoming call
type AcceptCommand struct {
	Command string     `json:"command"`
	Option  CallOption `json:"option"`
}

// RingingCommand sends a ringing command to the server
type RingingCommand struct {
	Command  string          `json:"command"`
	Ringtone string          `json:"ringtone,omitempty"`
	Recorder *RecorderOption `json:"recorder,omitempty"`
}

// RejectCommand rejects an incoming call
type RejectCommand struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
	Code    uint32 `json:"code,omitempty"`
}

// TtsCommand sends text to be synthesized
type TtsCommand struct {
	Command          string     `json:"command"`
	Text             string     `json:"text"`
	Speaker          string     `json:"speaker,omitempty"`
	PlayID           string     `json:"playId,omitempty"`
	AutoHangup       bool       `json:"autoHangup,omitempty"`
	Streaming        bool       `json:"streaming,omitempty"`
	EndOfStream      bool       `json:"endOfStream,omitempty"`
	Option           *TTSOption `json:"option,omitempty"`
	WaitInputTimeout *uint32    `json:"waitInputTimeout,omitempty" comment:"tts wait input timeout, 5000"`
	Base64           bool       `json:"base64,omitempty"`
}

// PlayCommand plays audio from a URL
type PlayCommand struct {
	Command          string  `json:"command"`
	URL              string  `json:"url"`
	AutoHangup       bool    `json:"autoHangup,omitempty"`
	WaitInputTimeout *uint32 `json:"waitInputTimeout,omitempty" comment:"tts wait input timeout, 5000"`
}

// InterruptCommand interrupts current playback
type InterruptCommand struct {
	Command  string `json:"command"`
	Graceful bool   `json:"graceful,omitempty"`
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
	Reason  string `json:"reason,omitempty"`
}

// ReferCommand transfers the call
type ReferCommand struct {
	Command string       `json:"command"`
	Caller  string       `json:"caller"`
	Callee  string       `json:"callee"`
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
type HistoryCommand struct {
	Command string `json:"command"`
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type InterruptionCommand struct {
	Command string `json:"command"`
}

// RecorderFormat represents the audio file format for recording
type RecorderFormat string

const (
	RecorderFormatWav  RecorderFormat = "wav"
	RecorderFormatPcm  RecorderFormat = "pcm"
	RecorderFormatPcmu RecorderFormat = "pcmu"
	RecorderFormatPcma RecorderFormat = "pcma"
	RecorderFormatG722 RecorderFormat = "g722"
)

type RecorderOption struct {
	// RecorderFile is the path to save the recording file (supports local path or HTTP URL)
	RecorderFile string `json:"recorderFile,omitempty"`
	// Samplerate is the audio sample rate, default 16000
	Samplerate int `json:"samplerate,omitempty"`
	// Ptime is the packet time in milliseconds, default 200
	Ptime int `json:"ptime,omitempty"`
	// Format is the recording file format: wav, pcm, pcmu, pcma, g722
	Format RecorderFormat `json:"format,omitempty"`
}

// AGCOption represents Automatic Gain Control configuration
// AGC2 processes audio in fixed 10ms sub-frames to normalize audio levels
type AGCOption struct {
	// HeadroomDB is the target headroom below 0 dBFS in dB, default 5.0
	HeadroomDB *float32 `json:"headroomDb,omitempty"`
	// MaxGainDB is the maximum gain in dB, default 50.0
	MaxGainDB *float32 `json:"maxGainDb,omitempty"`
	// InitialGainDB is the initial gain applied before speech level estimator becomes confident, default 15.0
	InitialGainDB *float32 `json:"initialGainDb,omitempty"`
	// MaxGainChangeDBPerSecond is the max gain change in dB per second (attack/release rate), default 6.0
	MaxGainChangeDBPerSecond *float32 `json:"maxGainChangeDbPerSecond,omitempty"`
	// MaxOutputNoiseLevelDBFS is the noise floor cap above which AGC will not amplify, default -50.0
	MaxOutputNoiseLevelDBFS *float32 `json:"maxOutputNoiseLevelDbfs,omitempty"`
	// AdjacentSpeechFramesThreshold is the number of consecutive speech sub-frames required before allowing gain increase, default 12 (~120ms)
	AdjacentSpeechFramesThreshold *int32 `json:"adjacentSpeechFramesThreshold,omitempty"`
	// EnableLimiter runs the limiter after the adaptive gain, default true
	EnableLimiter *bool `json:"enableLimiter,omitempty"`
}

// AmbianceOption represents background ambiance (ambient sound) configuration
// Path supports local file path or HTTP URL (downloaded and cached automatically)
type AmbianceOption struct {
	// Path is the audio file path for ambiance (local path or HTTP/HTTPS URL)
	Path string `json:"path,omitempty"`
	// DuckLevel is the volume level when server-side is speaking (e.g. TTS), default 0.1
	DuckLevel *float32 `json:"duckLevel,omitempty"`
	// NormalLevel is the volume level when server-side is silent, default 0.3
	NormalLevel *float32 `json:"normalLevel,omitempty"`
	// TransitionSpeed is the speed of volume transition between duck/normal levels, default 0.01
	TransitionSpeed *float32 `json:"transitionSpeed,omitempty"`
	// Enabled controls whether ambiance mixing is active, default true
	Enabled *bool `json:"enabled,omitempty"`
}

// RealtimeOption represents configuration for realtime AI APIs (e.g. OpenAI Realtime API, Azure)
// When set, it creates a RealtimeProcessor and short-circuits VAD/ASR processing
type RealtimeOption struct {
	// Provider is the realtime provider: "openai", "azure", or custom
	Provider string `json:"provider,omitempty"`
	// Model is the model name (e.g. "gpt-4o-realtime-preview")
	Model string `json:"model,omitempty"`
	// SecretKey is the API key for authentication, falls back to OPENAI_API_KEY env var
	SecretKey string `json:"secretKey,omitempty"`
	// SecretID is the secret ID for providers that require it (e.g. Azure)
	SecretID string `json:"secretId,omitempty"`
	// Endpoint is the custom API endpoint URL
	Endpoint string `json:"endpoint,omitempty"`
	// TurnDetection is the turn detection configuration (JSON object, e.g. {"type":"server_vad"})
	TurnDetection json.RawMessage `json:"turnDetection,omitempty"`
	// Tools is the list of tool definitions (JSON array) for function calling
	Tools []json.RawMessage `json:"tools,omitempty"`
	// Extra is additional provider-specific options
	Extra map[string]string `json:"extra,omitempty"`
}

type VADOption struct {
	Type                  string  `json:"type,omitempty" comment:"vad type, silero|ten"`
	Samplerate            uint32  `json:"samplerate,omitempty" comment:"vad samplerate, 16000|48000"`
	SpeechPadding         uint64  `json:"speechPadding,omitempty" comment:"vad speech padding, 120"`
	SilencePadding        uint64  `json:"silencePadding,omitempty" comment:"vad silence padding, 200"`
	Ratio                 float32 `json:"ratio,omitempty" comment:"vad ratio, 0.5|0.7"`
	VoiceThreshold        float32 `json:"voiceThreshold,omitempty" comment:"vad voice threshold, 0.5"`
	MaxBufferDurationSecs uint64  `json:"maxBufferDurationSecs,omitempty"`
	Endpoint              string  `json:"endpoint,omitempty"`
	SecretKey             string  `json:"secretKey,omitempty"`
	SecretID              string  `json:"secretId,omitempty"`
	SilenceTimeout        uint    `json:"silenceTimeout,omitempty" comment:"vad silence timeout, 5000"`
}

type ASROption struct {
	Provider        string            `json:"provider,omitempty" comment:"asr provider, tencent|aliyun"`
	Model           string            `json:"model,omitempty"`
	Language        string            `json:"language,omitempty"`
	AppID           string            `json:"appId,omitempty"`
	SecretID        string            `json:"secretId,omitempty"`
	SecretKey       string            `json:"secretKey,omitempty"`
	ModelType       string            `json:"modelType,omitempty"`
	BufferSize      int               `json:"bufferSize,omitempty"`
	SampleRate      uint32            `json:"sampleRate,omitempty"`
	Endpoint        string            `json:"endpoint,omitempty"`
	Extra           map[string]string `json:"extra,omitempty"`
	StartWhenAnswer bool              `json:"startWhenAnswer,omitempty"` // start asr when call is answered
}

type TTSOption struct {
	Samplerate         int32             `json:"samplerate,omitempty" comment:"tts samplerate, 16000|48000"`
	Provider           string            `json:"provider,omitempty" comment:"tts provider, tencent|aliyun"`
	Speed              float32           `json:"speed,omitempty"`
	AppID              string            `json:"appId,omitempty"`
	SecretID           string            `json:"secretId,omitempty"`
	SecretKey          string            `json:"secretKey,omitempty"`
	Volume             int32             `json:"volume,omitempty"`
	Speaker            string            `json:"speaker,omitempty"`
	Codec              string            `json:"codec,omitempty"`
	Subtitle           bool              `json:"subtitle,omitempty"`
	Emotion            string            `json:"emotion,omitempty"`
	Endpoint           string            `json:"endpoint,omitempty"`
	Extra              map[string]string `json:"extra,omitempty"`
	WaitInputTimeout   uint32            `json:"waitInputTimeout,omitempty" comment:"tts wait input timeout, 5000"`
	MaxConcurrentTasks int32             `json:"maxConcurrentTasks,omitempty" comment:"tts max concurrent tasks, 1"`
}

// SipOption represents SIP protocol options for call setup
type SipOption struct {
	// Username is the SIP authentication username
	Username string `json:"username,omitempty"`
	// Password is the SIP authentication password
	Password string `json:"password,omitempty"`
	// Realm is the SIP authentication realm
	Realm string `json:"realm,omitempty"`
	// Contact is the SIP contact header URI
	Contact string `json:"contact,omitempty"`
	// Headers are custom SIP headers to include in INVITE
	Headers map[string]string `json:"headers,omitempty"`
	// HangupHeaders are custom SIP headers to include in BYE
	HangupHeaders map[string]string `json:"hangupHeaders,omitempty"`
	// ExtractHeaders is the list of SIP header names to extract and include in events
	ExtractHeaders []string `json:"extractHeaders,omitempty"`
	// EnableSRTP enables secure RTP (SRTP) for the call
	EnableSRTP bool `json:"enableSrtp,omitempty"`
}

// CallOption represents options for invite/answer commands
type CallOption struct {
	// Denoise enables noise reduction preprocessing
	Denoise bool `json:"denoise,omitempty"`
	// AGC configures Automatic Gain Control for audio level normalization
	AGC *AGCOption `json:"agc,omitempty"`
	// Offer is the SDP offer string (bypasses internal SDP generation)
	Offer string `json:"offer,omitempty"`
	// Callee is the callee URI (e.g. "sip:bob@example.com" or "1001")
	Callee string `json:"callee,omitempty"`
	// Caller is the caller URI (e.g. "sip:alice@example.com")
	Caller string `json:"caller,omitempty"`
	// Recorder configures call recording
	Recorder *RecorderOption `json:"recorder,omitempty"`
	// VAD configures Voice Activity Detection
	VAD *VADOption `json:"vad,omitempty"`
	// ASR configures Automatic Speech Recognition
	ASR *ASROption `json:"asr,omitempty"`
	// TTS configures Text-to-Speech synthesis
	TTS *TTSOption `json:"tts,omitempty"`
	// MediaPass configures media pass-through via WebSocket
	MediaPass *MediaPassOption `json:"mediaPass,omitempty"`
	// HandshakeTimeout is the SIP handshake timeout in seconds
	HandshakeTimeout int `json:"handshakeTimeout,omitempty"`
	// EnableIPv6 enables IPv6 for the call
	EnableIPv6 bool `json:"enableIpv6,omitempty"`
	// InactivityTimeout is the call inactivity timeout in seconds, default 50
	InactivityTimeout int `json:"inactivityTimeout,omitempty"`
	// Sip configures SIP options (credentials, headers, etc.)
	Sip *SipOption `json:"sip,omitempty"`
	// Extra is a map of additional custom options
	Extra map[string]string `json:"extra,omitempty"`
	// Codec specifies the audio codec (pcmu, pcma, g722, pcm) for websocket calls
	Codec string `json:"codec,omitempty"`
	// Ambiance configures background ambiance (ambient sound) mixing
	Ambiance *AmbianceOption `json:"ambiance,omitempty"`
	// Eou configures End-of-Utterance detection
	Eou *EouOption `json:"eou,omitempty"`
	// Realtime configures realtime AI API (OpenAI/Azure Realtime API), short-circuits VAD/ASR
	Realtime *RealtimeOption `json:"realtime,omitempty"`
	// Subscribe enables event subscription for realtime updates
	Subscribe bool `json:"subscribe,omitempty"`
	// EnableIceLite enables ICE lite mode (suitable for server-side only)
	EnableIceLite bool `json:"enableIceLite,omitempty"`
	// RingbackDetection configures ringback tone detection (busy tone, human voice, etc.)
	RingbackDetection *RingbackDetectionOption `json:"ringbackDetection,omitempty"`
}

// MediaPassOption configures media pass-through, routing audio via an external WebSocket server
type MediaPassOption struct {
	// URL is the WebSocket server URL (e.g. "ws://localhost:8080/")
	URL string `json:"url"`
	// InputSampleRate is the sample rate of audio received from the WebSocket
	InputSampleRate int `json:"inputSampleRate"`
	// OutputSampleRate is the sample rate of audio sent to the WebSocket server
	OutputSampleRate int `json:"outputSampleRate"`
	// PacketSize is the packet size in bytes sent to the WebSocket server, default 3200
	PacketSize int `json:"packetSize,omitempty"`
	// Ptime is the packet time in milliseconds; if set, buffers and emits audio at fixed intervals
	Ptime int `json:"ptime,omitempty"`
}

// EouOption configures End-of-Utterance detection
type EouOption struct {
	// Type is the EOU detection provider type
	Type string `json:"type,omitempty"`
	// Endpoint is the EOU service endpoint URL
	Endpoint string `json:"endpoint,omitempty"`
	// SecretKey is the API key for the EOU service
	SecretKey string `json:"secretKey,omitempty"`
	// SecretID is the secret ID for the EOU service
	SecretID string `json:"secretId,omitempty"`
	// Timeout is the max timeout in milliseconds for EOU detection
	Timeout uint32 `json:"timeout,omitempty"`
	// Extra is additional provider-specific options
	Extra map[string]string `json:"extra,omitempty"`
}

// ReferOption represents options for the refer (call transfer) command
type ReferOption struct {
	// Denoise enables noise reduction for the refer call
	Denoise bool `json:"denoise,omitempty"`
	// AGC configures Automatic Gain Control for the refer call
	AGC *AGCOption `json:"agc,omitempty"`
	// Timeout is the refer call timeout in seconds
	Timeout uint32 `json:"timeout,omitempty"`
	// MusicOnHold is the music-on-hold audio URL played to the caller during transfer
	MusicOnHold string `json:"moh,omitempty"`
	// VAD configures Voice Activity Detection for the refer call
	VAD *VADOption `json:"vad,omitempty"`
	// ASR configures Automatic Speech Recognition for the refer call
	ASR *ASROption `json:"asr,omitempty"`
	// AutoHangup hangs up the parent call after the refer call ends
	AutoHangup bool `json:"autoHangup,omitempty"`
	// Sip configures SIP options for the refer call (credentials, headers, etc.)
	Sip *SipOption `json:"sip,omitempty"`
	// CallID specifies the call ID to transfer (for attended transfer)
	CallID string `json:"callId,omitempty"`
	// PauseParentASR pauses the parent call's ASR during the refer call, resumes after refer ends (if autoHangup is false)
	PauseParentASR bool `json:"pauseParentAsr,omitempty"`
	// ForwardDTMF forwards DTMF tones between the main call and the refer call; default false
	ForwardDTMF bool `json:"forwardDtmf,omitempty"`
}

// RingbackDetectionOption configures ringback tone (TelcoClassifier) detection
// Detects call progress states: ringing, human_voice, busy_tone, silence, etc.
type RingbackDetectionOption struct {
	// Enabled enables ringback detection
	Enabled bool `json:"enabled,omitempty"`
	// ModelWeightsPath is the path to telcoclassifier_weights.bin, default "./telcoclassifier_weights.bin"
	ModelWeightsPath string `json:"modelWeightsPath,omitempty"`
	// MinBufferSecs is the minimum audio accumulation (seconds) before first inference, minimum 4.0, default 6.0
	MinBufferSecs float32 `json:"minBufferSecs,omitempty"`
	// DetectionIntervalSecs is the seconds between consecutive inferences
	DetectionIntervalSecs float32 `json:"detectionIntervalSecs,omitempty"`
	// ConfidenceThreshold is the confidence threshold for reporting a state
	ConfidenceThreshold float32 `json:"confidenceThreshold,omitempty"`
	// OnStateChangeOnly emits events only on state change (ringing->human_voice etc.)
	OnStateChangeOnly bool `json:"onStateChangeOnly,omitempty"`
	// SlidingWindowSize is the sliding window size for result accumulation, default 6
	SlidingWindowSize int `json:"slidingWindowSize,omitempty"`
	// FinalConfidenceThreshold is the confidence threshold for immediate finalization, default 0.9
	FinalConfidenceThreshold float32 `json:"finalConfidenceThreshold,omitempty"`
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
func WithDumpEvents(dump bool) ClientOption {
	return func(c *Client) {
		c.dumpEvents = dump
	}
}

func (c *Client) Connect(callType string) error {
	if c.conn != nil {
		return errors.New("client already connected")
	}
	if c.cancel == nil {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	}
	url := c.endpoint
	url += "/call/" + callType
	if c.id != "" {
		url = fmt.Sprintf("%s?id=%s", url, c.id)
	}
	if c.dumpEvents {
		if strings.Contains(url, "?") {
			url += "&"
		} else {
			url += "?"
		}
		url += "dump=true"
	}
	var err error
	c.conn, _, err = websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	chanSize := 128
	c.eventChan = make(chan []byte, chanSize)
	c.cmdChan = make(chan any, chanSize)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				var buf [4096]byte
				runtime.Stack(buf[:], false)
				c.logger.Errorf("Panic in WebSocket read loop:%v \n%s", r, string(buf[:]))
			}
		}()
		for {
			mt, message, err := c.conn.ReadMessage()
			if err != nil {
				c.logger.Errorf("Error reading message: %v", err)
				if c.OnClose != nil {
					c.OnClose(err.Error())
				}
				return
			}
			if mt == websocket.BinaryMessage {
				if c.OnBinary != nil {
					data := make([]byte, len(message))
					copy(data, message)
					go func() {
						if len(data) > 0 {
							c.OnBinary(int(data[0]), data[1:])
						}
					}()
				}
				continue
			}
			if mt != websocket.TextMessage {
				c.logger.Debugf("Received non-text message: %v", mt)
				continue
			}
			select {
			case c.eventChan <- message:
			case <-c.ctx.Done():
				return
			default:
				c.logger.Warnf("Event channel is full, dropping message: %s", string(message))
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				var buf [4096]byte
				runtime.Stack(buf[:], false)
				c.logger.Errorf("Panic in event processing loop: %v \n%s", r, string(buf[:]))
			}
			close(c.eventChan)
			c.conn.Close()
		}()

		for {
			select {
			case <-c.ctx.Done():
				c.logger.Info("Context cancelled, shutting down event processing")
				return
			case cmd, ok := <-c.cmdChan:
				if !ok {
					return
				}

				c.logger.WithFields(logrus.Fields{
					"command": cmd,
				}).Debug("Sending command")
				err = c.conn.WriteJSON(cmd)
				if err != nil {
					c.logger.Errorf("Error sending command: %v", err)
					return
				}
			case message, ok := <-c.eventChan:
				if !ok {
					return
				}
				go c.processEvent(message)
			}
		}
	}()
	return nil
}

func (c *Client) processEvent(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			var buf [4096]byte
			runtime.Stack(buf[:], false)
			c.logger.Errorf("Panic in processEvent: %v %s \n%s", r, string(message), string(buf[:]))
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
	case "mediaReady":
		var event MediaReadyEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling mediaReady event: %v", err)
			return
		}
		if c.OnMediaReady != nil {
			c.OnMediaReady(event)
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
	case "dtmf":
		var event DTMFEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling dtmf event: %v", err)
			return
		}
		if c.OnDTMF != nil {
			c.OnDTMF(event)
		}
	case "hold":
		var event HoldEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling hold event: %v", err)
			return
		}
		if c.OnHold != nil {
			c.OnHold(event)
		}
	case "transferRequest":
		var event TransferRequestEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling transferRequest event: %v", err)
			return
		}
		if c.OnTransferRequest != nil {
			c.OnTransferRequest(event)
		}
	case "message":
		var event MessageEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling message event: %v", err)
			return
		}
		if c.OnMessage != nil {
			c.OnMessage(event)
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
	case "functionCall":
		var event FunctionCallEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling functionCall event: %v", err)
			return
		}
		if c.OnFunctionCall != nil {
			c.OnFunctionCall(event)
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
	case "addHistory":
		var event AddHistoryEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling addHistory event: %v", err)
			return
		}
		if c.OnAddHistory != nil {
			c.OnAddHistory(event)
		}
	case "eou":
		var event EouEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling eou event: %v", err)
			return
		}
		if c.OnEou != nil {
			c.OnEou(event)
		}
	case "inactivity":
		var event InactivityEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling inactivity event: %v", err)
			return
		}
		if c.OnInactivity != nil {
			c.OnInactivity(event)
		}
	case "interrupt":
		var event InterruptEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling interrupt event: %v", err)
			return
		}
		if c.OnInterrupt != nil {
			c.OnInterrupt(event)
		}
	case "ping":
		var event PingEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling ping event: %v", err)
			return
		}
		if c.OnPing != nil {
			c.OnPing(event)
		}
	case "other":
		var event OtherEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling other event: %v", err)
			return
		}
		if c.OnOther != nil {
			c.OnOther(event)
		}
	case "custom":
		var event CustomEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling custom event: %v", err)
			return
		}
		if c.OnCustom != nil {
			c.OnCustom(event)
		}
	case "ringbackState":
		var event RingbackStateEvent
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("Error unmarshalling ringbackState event: %v", err)
			return
		}
		if c.OnRingbackState != nil {
			c.OnRingbackState(event)
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
func (c *Client) Invite(ctx context.Context, option CallOption) (*AnswerEvent, error) {
	onAnswer := c.onAnswer
	ch := make(chan any, 1)
	c.onAnswer = func(event AnswerEvent) {
		ch <- event
	}
	onError := c.OnError
	c.OnError = func(event ErrorEvent) {
		select {
		case ch <- event:
		default:
			c.logger.Errorf("Error event channel is full, dropping event: %s", event.Error)
			onError(event)
		}
	}
	onReject := c.OnReject
	c.OnReject = func(event RejectEvent) {
		select {
		case ch <- event:
		default:
			c.logger.Errorf("Reject event channel is full, dropping event: %s", event.Reason)
			onReject(event)
		}
	}
	onHangup := c.OnHangup
	c.OnHangup = func(event HangupEvent) {
		select {
		case ch <- event:
		default:
			reason := ""
			if event.Reason != nil {
				reason = *event.Reason
			}
			c.logger.Errorf("Hangup event channel is full, dropping event: %s", reason)
			onHangup(event)
		}
	}
	defer func() {
		c.onAnswer = onAnswer
		c.OnError = onError
		c.OnReject = onReject
		c.OnHangup = onHangup
	}()
	cmd := InviteCommand{
		Command: "invite",
		Option:  option,
	}
	err := c.sendCommand(cmd)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case event := <-ch:
		if event, ok := event.(AnswerEvent); ok {
			return &event, nil
		}
		if event, ok := event.(ErrorEvent); ok {
			return nil, errors.New(event.Error)
		}
		if event, ok := event.(RejectEvent); ok {
			return nil, errors.New(event.Reason)
		}
		if event, ok := event.(HangupEvent); ok {
			reason := ""
			if event.Reason != nil {
				reason = *event.Reason
			}
			return nil, errors.New(reason)
		}
		return nil, errors.New("invalid event type")
	}
}

// Ringing sends a ringing command to the server
func (c *Client) Ringing(ringtone string, recorder *RecorderOption) error {
	cmd := RingingCommand{
		Command:  "ringing",
		Ringtone: ringtone,
		Recorder: recorder,
	}
	return c.sendCommand(cmd)
}

// Accept sends an accept command to accept an incoming call
func (c *Client) Accept(option CallOption) error {
	cmd := AcceptCommand{
		Command: "accept",
		Option:  option,
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

// TTS sends a text-to-speech command
func (c *Client) TTS(text string, speaker string, playID string, endOfStream, autoHangup bool, option *TTSOption, waitInputTimeout *uint32, base64 bool) error {
	cmd := TtsCommand{
		Command:          "tts",
		Text:             text,
		Speaker:          speaker,
		PlayID:           playID,
		AutoHangup:       autoHangup,
		Streaming:        false,
		EndOfStream:      endOfStream,
		Option:           option,
		WaitInputTimeout: waitInputTimeout,
		Base64:           base64,
	}
	return c.sendCommand(cmd)
}

// TTS sends a text-to-speech command
func (c *Client) StreamTTS(text string, speaker string, playID string, endOfStream, autoHangup bool, option *TTSOption, waitInputTimeout *uint32, base64 bool) error {
	cmd := TtsCommand{
		Command:          "tts",
		Text:             text,
		Speaker:          speaker,
		PlayID:           playID,
		AutoHangup:       autoHangup,
		Streaming:        true,
		EndOfStream:      endOfStream,
		Option:           option,
		WaitInputTimeout: waitInputTimeout,
		Base64:           base64,
	}
	return c.sendCommand(cmd)
}

// Play sends a command to play audio from a URL
func (c *Client) Play(url string, autoHangup bool, waitInputTimeout *uint32) error {
	cmd := PlayCommand{
		Command:          "play",
		URL:              url,
		AutoHangup:       autoHangup,
		WaitInputTimeout: waitInputTimeout,
	}
	return c.sendCommand(cmd)
}

// Interrupt sends a command to interrupt current playback
func (c *Client) Interrupt(graceful bool) error {
	cmd := InterruptCommand{
		Command:  "interrupt",
		Graceful: graceful,
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
func (c *Client) Hangup(reason string) error {
	cmd := HangupCommand{
		Command: "hangup",
		Reason:  reason,
	}
	return c.sendCommand(cmd)
}

// Refer sends a command to transfer the call.
//
// Parameters:
//   - caller: the caller to transfer the call from, eg. "sip:alice@restsend.com"
//   - callee: the callee to transfer the call to, eg. "sip:bob@restsend.com"
//   - options: optional parameters for the transfer
func (c *Client) Refer(caller, callee string, options *ReferOption) error {
	cmd := ReferCommand{
		Command: "refer",
		Caller:  caller,
		Callee:  callee,
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

func (c *Client) History(speaker string, text string) error {
	cmd := HistoryCommand{
		Command: "history",
		Speaker: speaker,
		Text:    text,
	}
	return c.sendCommand(cmd)
}

// sendCommand sends a command to the server
func (c *Client) sendCommand(cmd any) error {
	// Implementation will be added when the WebSocket connection is implemented
	if c.conn == nil {
		return errors.New("client not initialized")
	}
	c.cmdChan <- cmd
	return nil
}
