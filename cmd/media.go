package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/shenjinti/go711"
	"github.com/sirupsen/logrus"
)

// MediaHandler handles WebRTC and audio encoding
type MediaHandler struct {
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *logrus.Logger
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
		buffer:         make([]byte, 0, 16000), // Buffer for 1 second of audio at 16kHz
		sequenceNumber: 0,
		timestamp:      0,
		playbackCtx:    playbackCtx,
	}, nil
}

func (mh *MediaHandler) Setup() (string, error) {
	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterCodec(
		webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:  webrtc.MimeTypePCMU,
				ClockRate: 8000,
			},
			PayloadType: 0,
		},
		webrtc.RTPCodecTypeAudio,
	)
	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
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
		MimeType:  webrtc.MimeTypePCMU,
		ClockRate: 8000,
		Channels:  1,
	}, "rustpbxgo-audio", "rustpbxgo-audio")
	if err != nil {
		return "", fmt.Errorf("failed to create audio track: %w", err)
	}
	// Add track to peer connection
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		return "", fmt.Errorf("failed to add track to peer connection: %w", err)
	}
	mh.audioTrack = audioTrack
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		mh.logger.Infof("Track remote added %v %s", track.ID(), track.Codec().MimeType)
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

				audioData, _ := go711.DecodePCMU(rtpPacket.Payload)
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
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		mh.logger.Infof("ICE connection state: %v", state)
	})
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create offer: %w", err)
	}
	peerConnection.SetLocalDescription(offer)

	select {
	case <-webrtc.GatheringCompletePromise(peerConnection):
		mh.logger.Info("ICE Gathering complete")
	case <-time.After(20 * time.Second):
		mh.logger.Warn("ICE Gathering timeout")
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
	return mh.peerConnection.SetRemoteDescription(remoteOffer)
}

// encodeAndSendAudio encodes audio to G.722 and sends it via WebRTC
func (mh *MediaHandler) encodeAndSendAudio() {
	ticker := time.NewTicker(20 * time.Millisecond)
	framesize := int(20 * int(mh.captureDevice.SampleRate()) / 1000 * 2)
	for mh.connected {
		select {
		case <-ticker.C:
		case <-mh.ctx.Done():
			return
		}
		mh.bufferMutex.Lock()
		if len(mh.buffer) < framesize { // 20ms at 8khz
			mh.bufferMutex.Unlock()
			continue
		}

		// Get audio data from buffer
		audioData := mh.buffer[:framesize]
		mh.buffer = mh.buffer[framesize:]
		mh.bufferMutex.Unlock()

		payload, _ := go711.EncodePCMU(audioData)
		// Create media sample
		sample := media.Sample{
			Data:      payload,
			Duration:  20 * time.Millisecond,
			Timestamp: time.Now(),
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
	deviceConfig.SampleRate = 8000
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
			n := copy(outputSamples, mh.playbackBuffer)
			mh.playbackBuffer = mh.playbackBuffer[n:]
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
	deviceConfig.SampleRate = 8000
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
