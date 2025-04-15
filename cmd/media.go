package main

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/shenjinti/go722"
	"github.com/sirupsen/logrus"
)

// MediaHandler handles WebRTC and audio encoding
type MediaHandler struct {
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *logrus.Logger
	encoder        *go722.G722Encoder
	decoder        *go722.G722Decoder
	peerConnection *webrtc.PeerConnection
	audioTrack     *webrtc.TrackLocalStaticSample
	buffer         []byte
	bufferMutex    sync.Mutex
	connected      bool
	mu             sync.Mutex
	sequenceNumber uint16
	timestamp      uint32
	playbackBuffer []byte
	playbackMutex  *sync.Mutex
	playbackDevice *malgo.Device
	playbackCtx    *malgo.AllocatedContext
	captureDevice  *malgo.Device
}

// NewMediaHandler creates a new media handler
func NewMediaHandler(ctx context.Context, logger *logrus.Logger) (*MediaHandler, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Create G.722 encoder and decoder
	encoder := go722.NewG722Encoder(go722.Rate48000, 0)
	decoder := go722.NewG722Decoder(go722.Rate48000, 0)

	// Initialize playback context
	playbackCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize playback context: %w", err)
	}

	return &MediaHandler{
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		encoder:        encoder,
		decoder:        decoder,
		buffer:         make([]byte, 0, 16000), // Buffer for 1 second of audio at 16kHz
		sequenceNumber: 0,
		timestamp:      0,
		playbackCtx:    playbackCtx,
	}, nil
}

func (mh *MediaHandler) Setup() (string, error) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create peer connection: %w", err)
	}
	mh.peerConnection = peerConnection

	// Create audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeG722,
		ClockRate: 16000,
		Channels:  1,
	}, "audio", "rustpbxgo")
	if err != nil {
		return "", fmt.Errorf("failed to create audio track: %w", err)
	}
	// Add track to peer connection
	rtpSender, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		return "", fmt.Errorf("failed to add track to peer connection: %w", err)
	}
	mh.audioTrack = audioTrack

	if rtpSender != nil {
		go func() {
			for {
				if _, _, rtcpErr := rtpSender.ReadRTCP(); rtcpErr != nil {
					if rtcpErr == io.EOF {
						return
					}
					mh.logger.Errorf("Failed to read RTCP packet: %v", rtcpErr)
					continue
				}
			}
		}()
	}
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		mh.logger.Infof("Track remote added %v", track)
		go func() {
			for mh.connected {
				if mh.ctx.Err() != nil {
					return
				}
				rtpPacket, _, err := track.ReadRTP()
				if err != nil {
					mh.logger.Errorf("Failed to read RTP packet: %v", err)
					break
				}

				// Decode G.722 data to PCM
				audioData := mh.decoder.Decode(rtpPacket.Payload)
				// Add to playback buffer
				mh.playbackMutex.Lock()
				mh.playbackBuffer = append(mh.playbackBuffer, audioData...)
				mh.playbackMutex.Unlock()
			}
		}()
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		mh.logger.Infof("Peer connection state: %v", state)
		if state == webrtc.PeerConnectionStateConnected {
			mh.connected = true
			mh.initPlaybackDevice()
			mh.startAudioCapture()
			go mh.encodeAndSendAudio()
		}
	})
	peerConnection.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		mh.logger.Infof("ICE gathering state: %v", state)
	})
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		mh.logger.Infof("ICE candidate: %v", candidate)
	})
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create offer: %w", err)
	}
	peerConnection.SetLocalDescription(offer)

	select {
	case <-webrtc.GatheringCompletePromise(peerConnection):
		mh.logger.Info("Gathering complete")
	case <-time.After(20 * time.Second):
		mh.logger.Warn("Gathering timeout")
		return "", fmt.Errorf("gathering timeout")
	}
	offerSdp := peerConnection.LocalDescription().SDP
	return offerSdp, nil
}

func (mh *MediaHandler) SetupAnswer(answer string) error {
	remoteOffer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer,
	}
	err := mh.peerConnection.SetRemoteDescription(remoteOffer)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	select {
	case <-webrtc.GatheringCompletePromise(mh.peerConnection):
		mh.logger.Info("Gathering complete")
	case <-time.After(10 * time.Second):
		mh.logger.Warn("Gathering timeout")
		return fmt.Errorf("gathering timeout")
	}

	return nil
}

// encodeAndSendAudio encodes audio to G.722 and sends it via WebRTC
func (mh *MediaHandler) encodeAndSendAudio() {
	ticker := time.NewTicker(20 * time.Millisecond)
	for mh.connected {
		select {
		case <-ticker.C:
		case <-mh.ctx.Done():
			return
		}
		mh.bufferMutex.Lock()
		if len(mh.buffer) < 640 { // 20ms at 48kHz
			mh.bufferMutex.Unlock()
			continue
		}

		// Get audio data from buffer
		audioData := mh.buffer[:640]
		mh.buffer = mh.buffer[640:]
		mh.bufferMutex.Unlock()

		audioData = mh.encoder.Encode(audioData)

		// Create media sample
		sample := media.Sample{
			Data:     audioData,
			Duration: 20 * time.Millisecond,
		}
		// Send via WebRTC
		err := mh.audioTrack.WriteSample(sample)
		if err != nil {
			mh.logger.Errorf("Failed to send audio sample: %v", err)
			continue
		}
	}
}

// Stop stops the media handler
func (mh *MediaHandler) Stop() error {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	if !mh.connected {
		return nil
	}
	mh.cancel()
	if mh.playbackDevice != nil {
		mh.playbackDevice.Stop()
		mh.playbackDevice.Uninit()
	}

	if mh.captureDevice != nil {
		mh.captureDevice.Stop()
		mh.captureDevice.Uninit()
	}
	if mh.playbackCtx != nil {
		mh.playbackCtx.Uninit()
	}

	if mh.peerConnection != nil {
		mh.peerConnection.Close()
	}

	mh.connected = false
	mh.logger.Info("Media handler stopped")
	return nil
}

func (mh *MediaHandler) initPlaybackDevice() error {
	// Set up playback device
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 1
	deviceConfig.SampleRate = 16000
	deviceConfig.Alsa.NoMMap = 1
	// Create a buffer for decoded audio
	mh.playbackBuffer = make([]byte, 0, 16000)
	mh.playbackMutex = &sync.Mutex{}
	// Create playback device
	playbackDevice, err := malgo.InitDevice(mh.playbackCtx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			if !mh.connected {
				return
			}
			mh.playbackMutex.Lock()
			if len(mh.playbackBuffer) >= 640 {
				n := copy(outputSamples, mh.playbackBuffer[:640])
				mh.playbackBuffer = mh.playbackBuffer[n:]
			}
			mh.playbackMutex.Unlock()
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize playback device: %w", err)
	}
	mh.playbackDevice = playbackDevice
	mh.logger.Info("Playback device initialized")
	return playbackDevice.Start()
}

func (mh *MediaHandler) startAudioCapture() error {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = 48000
	deviceConfig.Alsa.NoMMap = 1

	// Create capture device
	captureDevice, err := malgo.InitDevice(mh.playbackCtx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			if !mh.connected {
				return
			}
			mh.bufferMutex.Lock()
			mh.buffer = append(mh.buffer, inputSamples...)
			mh.bufferMutex.Unlock()
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize capture device: %v", err)
	}
	mh.captureDevice = captureDevice

	// Start capture device
	err = captureDevice.Start()
	if err != nil {
		return fmt.Errorf("failed to start capture device: %v", err)
	}
	mh.logger.Info("Capture device initialized")
	return nil
}
