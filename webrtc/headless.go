package webrtc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	pion "github.com/pion/webrtc/v3"
)

// Original HeadlessOptions for backward compatibility
type HeadlessOptions struct {
	Region     string
	Deployment string
	Ephemeral  string
	IceServers []pion.ICEServer
	OnMessage  func(msg []byte)
	OnAudioRTP func(pkts uint64)
}

// Enhanced options that support bidirectional audio
type EnhancedHeadlessOptions struct {
	Region     string
	Deployment string
	Ephemeral  string
	IceServers []pion.ICEServer
	OnMessage  func(msg []byte)
	OnAudioRTP func(pkts uint64)

	// NEW: Support for sending audio to Azure
	AudioInputTrack *pion.TrackLocalStaticSample
	OnReady         func(pc *pion.PeerConnection, dc *pion.DataChannel)
	OnTrack         func(track *pion.TrackRemote, receiver *pion.RTPReceiver)
}

// Enhanced HeadlessConnect that supports bidirectional audio
func EnhancedHeadlessConnect(ctx context.Context, opt EnhancedHeadlessOptions) error {
	if opt.Region == "" || opt.Deployment == "" || opt.Ephemeral == "" {
		return errors.New("region, deployment and ephemeral are required")
	}

	cfg := pion.Configuration{}
	if len(opt.IceServers) > 0 {
		cfg.ICEServers = opt.IceServers
	}

	pc, err := pion.NewPeerConnection(cfg)
	if err != nil {
		return err
	}
	defer pc.Close()

	// Create data channel
	dc, err := pc.CreateDataChannel("realtime-channel", nil)
	if err != nil {
		return err
	}

	if opt.OnMessage != nil {
		dc.OnMessage(func(m pion.DataChannelMessage) { opt.OnMessage(m.Data) })
	}

	// NEW: Add audio input track if provided (for sending audio TO Azure)
	if opt.AudioInputTrack != nil {
		if _, err := pc.AddTrack(opt.AudioInputTrack); err != nil {
			return fmt.Errorf("failed to add audio input track: %w", err)
		}
	}

	// Add receive transceiver for audio FROM Azure
	_, err = pc.AddTransceiverFromKind(pion.RTPCodecTypeAudio, pion.RTPTransceiverInit{
		Direction: pion.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		return err
	}

	// NEW: Enhanced track handling
	if opt.OnTrack != nil {
		pc.OnTrack(opt.OnTrack)
	} else if opt.OnAudioRTP != nil {
		pc.OnTrack(func(track *pion.TrackRemote, receiver *pion.RTPReceiver) {
			var pkts uint64
			buf := make([]byte, 1500)
			for {
				_, _, e := track.Read(buf)
				if e != nil {
					return
				}
				pkts++
				if pkts%200 == 0 {
					opt.OnAudioRTP(pkts)
				}
			}
		})
	}

	// Call OnReady callback if provided
	if opt.OnReady != nil {
		opt.OnReady(pc, dc)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		return err
	}

	url := fmt.Sprintf("%s?model=%s", RegionWebRTCURL(opt.Region), opt.Deployment)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(offer.SDP))
	req.Header.Set("Authorization", "Bearer "+opt.Ephemeral)
	req.Header.Set("Content-Type", "application/sdp")

	httpClient := &http.Client{Timeout: 20 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("SDP exchange failed: %d: %s", resp.StatusCode, string(b))
	}

	answer := pion.SessionDescription{Type: pion.SDPTypeAnswer, SDP: string(b)}
	if err := pc.SetRemoteDescription(answer); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

// Original HeadlessConnect for backward compatibility
func HeadlessConnect(ctx context.Context, opt HeadlessOptions) error {
	enhancedOpt := EnhancedHeadlessOptions{
		Region:     opt.Region,
		Deployment: opt.Deployment,
		Ephemeral:  opt.Ephemeral,
		IceServers: opt.IceServers,
		OnMessage:  opt.OnMessage,
		OnAudioRTP: opt.OnAudioRTP,
	}
	return EnhancedHeadlessConnect(ctx, enhancedOpt)
}

// Helper function to create a relay-friendly audio track
func CreateRelayAudioTrack() (*pion.TrackLocalStaticSample, error) {
	return pion.NewTrackLocalStaticSample(
		pion.RTPCodecCapability{MimeType: pion.MimeTypeOpus},
		"relay-audio",
		"relay-stream",
	)
}
