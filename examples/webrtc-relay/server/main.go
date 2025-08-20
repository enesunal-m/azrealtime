package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/enesunal-m/azrealtime/webrtc"
	"github.com/pion/rtp"
	pion "github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

// Message types for saving conversation data
type ConversationMessage struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Direction string                 `json:"direction"` // "browser_to_azure" or "azure_to_browser"
	Data      map[string]interface{} `json:"data"`
}

// Audio recording session
type AudioRecording struct {
	SessionID string
	StartTime time.Time
	oggWriter *oggwriter.OggWriter
	mutex     sync.Mutex
}

// Global variables for the single peer connection (browser side)
var (
	browserPeerConnection *pion.PeerConnection
	azurePeerConnection   *pion.PeerConnection
	pcMutex               sync.Mutex
	browserToAzureTrack   *pion.TrackLocalStaticSample
	azureToBrowserTrack   *pion.TrackLocalStaticSample
	browserDataChannel    *pion.DataChannel
	azureDataChannel      *pion.DataChannel
	messageBuffer         [][]byte // Buffer for messages while Azure not ready
	bufferMutex           sync.Mutex
	conversationLog       []ConversationMessage
	conversationMutex     sync.Mutex
	currentRecording      *AudioRecording
	recordingMutex        sync.Mutex
)

// saveMessage saves a message to the conversation log
func saveMessage(msgType, direction string, data map[string]interface{}) {
	conversationMutex.Lock()
	defer conversationMutex.Unlock()

	msg := ConversationMessage{
		Timestamp: time.Now(),
		Type:      msgType,
		Direction: direction,
		Data:      data,
	}

	conversationLog = append(conversationLog, msg)

	// Save to file every 10 messages
	if len(conversationLog)%10 == 0 {
		go saveConversationToFile()
	}
}

// saveConversationToFile saves the conversation log to a JSON file
func saveConversationToFile() {
	conversationMutex.Lock()
	defer conversationMutex.Unlock()

	if len(conversationLog) == 0 {
		return
	}

	filename := fmt.Sprintf("transcripts/conversation_%s.json", time.Now().Format("2006-01-02_15-04-05"))
	data, err := json.MarshalIndent(conversationLog, "", "  ")
	if err != nil {
		log.Printf("❌ Failed to marshal conversation: %v", err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("❌ Failed to save conversation: %v", err)
		return
	}

	log.Printf("💾 Saved conversation to %s (%d messages)", filename, len(conversationLog))
}

// startAudioRecording starts recording audio from the browser
func startAudioRecording() error {
	recordingMutex.Lock()
	defer recordingMutex.Unlock()

	// Close any existing recording
	if currentRecording != nil {
		currentRecording.mutex.Lock()
		if currentRecording.oggWriter != nil {
			currentRecording.oggWriter.Close()
		}
		currentRecording.mutex.Unlock()
	}

	// Create new recording
	sessionID := fmt.Sprintf("session_%s", time.Now().Format("20060102_150405"))
	filename := fmt.Sprintf("audio/audio_%s.ogg", sessionID)

	oggFile, err := oggwriter.New(filename, 48000, 2)
	if err != nil {
		return fmt.Errorf("failed to create OGG file: %v", err)
	}

	currentRecording = &AudioRecording{
		SessionID: sessionID,
		StartTime: time.Now(),
		oggWriter: oggFile,
	}

	log.Printf("🎙️ Started audio recording: %s", filename)
	return nil
}

// stopAudioRecording stops the current audio recording
func stopAudioRecording() {
	recordingMutex.Lock()
	defer recordingMutex.Unlock()

	if currentRecording != nil {
		currentRecording.mutex.Lock()
		if currentRecording.oggWriter != nil {
			currentRecording.oggWriter.Close()
			duration := time.Since(currentRecording.StartTime)
			log.Printf("🛑 Stopped audio recording: %s (duration: %v)",
				currentRecording.SessionID, duration)
		}
		currentRecording.mutex.Unlock()
		currentRecording = nil
	}
}

// writeAudioSample writes an audio sample to the current recording
func writeAudioSample(rtpPacket *rtp.Packet) {
	recordingMutex.Lock()
	recording := currentRecording
	recordingMutex.Unlock()

	if recording == nil || recording.oggWriter == nil {
		return
	}

	recording.mutex.Lock()
	defer recording.mutex.Unlock()

	if err := recording.oggWriter.WriteRTP(rtpPacket); err != nil {
		log.Printf("❌ Failed to write audio sample: %v", err)
	}
}

func main() {
	// Check required environment variables
	required := []string{
		"AZURE_OPENAI_ENDPOINT",
		"AZURE_OPENAI_API_KEY",
		"AZURE_OPENAI_REALTIME_DEPLOYMENT",
		"AZURE_OPENAI_REGION",
	}

	for _, env := range required {
		if os.Getenv(env) == "" {
			log.Fatalf("Environment variable %s is required", env)
		}
	}

	log.Printf("🎤 WebRTC Azure Relay Server")
	log.Printf("📡 Starting on port 8085")

	http.HandleFunc("/offer", handleOffer)
	http.HandleFunc("/ice-candidate", handleICECandidate)
	http.HandleFunc("/conversation", handleConversation)
	http.HandleFunc("/audio-files", handleAudioFiles)
	http.HandleFunc("/audio/", handleAudioDownload)
	http.HandleFunc("/", serveFiles)

	log.Printf("✅ Server ready at http://localhost:8085")
	log.Fatal(http.ListenAndServe(":8085", nil))
}

func handleConversation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conversationMutex.Lock()
	defer conversationMutex.Unlock()

	data, err := json.Marshal(conversationLog)
	if err != nil {
		http.Error(w, "Failed to encode conversation", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func handleICECandidate(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pcMutex.Lock()
	defer pcMutex.Unlock()

	if browserPeerConnection == nil {
		log.Printf("⚠️ ICE candidate received but no peer connection")
		http.Error(w, "Peer connection not established", http.StatusBadRequest)
		return
	}

	var candidate pion.ICECandidateInit
	if err := json.NewDecoder(r.Body).Decode(&candidate); err != nil {
		log.Printf("❌ Failed to decode ICE candidate: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := browserPeerConnection.AddICECandidate(candidate); err != nil {
		log.Printf("❌ Failed to add ICE candidate: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("📥 Added ICE candidate from browser: %s", candidate.Candidate)
	w.WriteHeader(http.StatusOK)
}

func handleOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	offerBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Failed to read offer: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	offerSDP := string(offerBody)
	log.Printf("📥 Received browser offer (%d chars)", len(offerSDP))

	// Create browser peer connection
	pc, err := pion.NewPeerConnection(pion.Configuration{
		ICEServers: []pion.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		log.Printf("❌ Failed to create peer connection: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Store the peer connection
	pcMutex.Lock()
	browserPeerConnection = pc
	pcMutex.Unlock()

	// Create tracks for audio relay
	var createTrackErr error
	azureToBrowserTrack, createTrackErr = pion.NewTrackLocalStaticSample(
		pion.RTPCodecCapability{MimeType: pion.MimeTypeOpus},
		"azure-audio",
		"azure-stream",
	)
	if createTrackErr != nil {
		log.Printf("❌ Failed to create Azure→Browser track: %v", createTrackErr)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	browserToAzureTrack, createTrackErr = pion.NewTrackLocalStaticSample(
		pion.RTPCodecCapability{MimeType: pion.MimeTypeOpus},
		"browser-audio",
		"browser-stream",
	)
	if createTrackErr != nil {
		log.Printf("❌ Failed to create Browser→Azure track: %v", createTrackErr)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Add Azure→Browser track to browser connection
	if _, err = pc.AddTrack(azureToBrowserTrack); err != nil {
		log.Printf("❌ Failed to add Azure→Browser track: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Added Azure→Browser track")

	// Wait for ICE gathering to complete
	gatheringComplete := make(chan struct{})
	pc.OnICEGatheringStateChange(func(state pion.ICEGathererState) {
		if state == pion.ICEGathererStateComplete {
			close(gatheringComplete)
		}
	})

	// Handle incoming audio from browser
	pc.OnTrack(func(track *pion.TrackRemote, receiver *pion.RTPReceiver) {
		log.Printf("🎉 SUCCESS! RECEIVED BROWSER AUDIO TRACK!")
		log.Printf("🎤 Track ID: %s, Codec: %s", track.ID(), track.Codec().MimeType)

		// Forward browser audio to Azure
		go forwardBrowserToAzure(track)
	})

	// Handle data channels from browser
	pc.OnDataChannel(func(dc *pion.DataChannel) {
		log.Printf("📡 Browser data channel: %s", dc.Label())
		browserDataChannel = dc
		setupBrowserDataChannel(dc)
	})

	// Connection state monitoring
	pc.OnConnectionStateChange(func(state pion.PeerConnectionState) {
		log.Printf("🔗 Browser connection state: %s", state.String())
		if state == pion.PeerConnectionStateConnected {
			log.Printf("✅ Browser connected - starting Azure connection")
			// Start audio recording when browser connects
			if err := startAudioRecording(); err != nil {
				log.Printf("❌ Failed to start audio recording: %v", err)
			}
			go setupAzureConnection()
		} else if state == pion.PeerConnectionStateFailed ||
			state == pion.PeerConnectionStateDisconnected ||
			state == pion.PeerConnectionStateClosed {
			pcMutex.Lock()
			browserPeerConnection = nil
			pcMutex.Unlock()
			// Stop audio recording when browser disconnects
			stopAudioRecording()
			log.Printf("🔌 Browser connection cleaned up")
		}
	})

	// Set remote description, create answer, set local description
	offer := pion.SessionDescription{Type: pion.SDPTypeOffer, SDP: offerSDP}
	if err := pc.SetRemoteDescription(offer); err != nil {
		log.Printf("❌ Failed to set remote description: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Set browser remote description")

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Printf("❌ Failed to create answer: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		log.Printf("❌ Failed to set local description: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Wait for gathering to complete before sending answer
	<-gatheringComplete

	log.Printf("📤 Sending answer to browser (%d chars)", len(pc.LocalDescription().SDP))

	// Send answer back to browser
	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(pc.LocalDescription().SDP))
}

func setupBrowserDataChannel(dc *pion.DataChannel) {
	dc.OnOpen(func() {
		log.Printf("📡 Browser data channel opened")

		// Flush any buffered messages when Azure becomes ready
		go func() {
			for {
				time.Sleep(500 * time.Millisecond)
				if azureDataChannel != nil && azureDataChannel.ReadyState() == pion.DataChannelStateOpen {
					bufferMutex.Lock()
					if len(messageBuffer) > 0 {
						for _, msg := range messageBuffer {
							if err := azureDataChannel.Send(msg); err != nil {
								log.Printf("❌ Failed to send buffered message: %v", err)
							}
						}
						messageBuffer = nil
					}
					bufferMutex.Unlock()
					break
				}
			}
		}()
	})

	dc.OnMessage(func(msg pion.DataChannelMessage) {
		// Parse and save the message
		var parsed map[string]any
		if err := json.Unmarshal(msg.Data, &parsed); err == nil {
			// Save message from browser
			if msgType, ok := parsed["type"].(string); ok {
				saveMessage(msgType, "browser_to_azure", parsed)
			}
		}

		// Forward to Azure data channel
		if azureDataChannel != nil && azureDataChannel.ReadyState() == pion.DataChannelStateOpen {
			if err := azureDataChannel.Send(msg.Data); err != nil {
				log.Printf("❌ Failed to forward to Azure: %v", err)
			}
		} else {
			// Buffer the message
			bufferMutex.Lock()
			messageBuffer = append(messageBuffer, msg.Data)
			bufferMutex.Unlock()
		}
	})

	dc.OnClose(func() {
		log.Printf("📡 Browser data channel closed")
		// Save conversation when connection closes
		saveConversationToFile()
	})
}

func setupAzureConnection() {
	log.Printf("🤖 Setting up Azure connection...")

	ctx := context.Background()

	// Get ephemeral token
	ephemeralSessionID, ephemeralToken, err := webrtc.MintEphemeralKey(
		ctx,
		os.Getenv("AZURE_OPENAI_ENDPOINT"),
		getEnvDefault("AZURE_OPENAI_API_VERSION", "2025-04-01-preview"),
		os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT"),
		os.Getenv("AZURE_OPENAI_API_KEY"),
		"alloy",
	)
	if err != nil {
		log.Printf("❌ Failed to mint ephemeral key: %v", err)
		return
	}

	log.Printf("🔑 Got ephemeral token for Azure session: %s", ephemeralSessionID)

	// Setup Azure connection
	azureOpts := webrtc.EnhancedHeadlessOptions{
		Region:          os.Getenv("AZURE_OPENAI_REGION"),
		Deployment:      os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT"),
		Ephemeral:       ephemeralToken,
		AudioInputTrack: browserToAzureTrack,
		OnReady: func(pc *pion.PeerConnection, dc *pion.DataChannel) {
			log.Printf("🎯 Azure connection ready")
			azurePeerConnection = pc
			azureDataChannel = dc

			// Handle Azure messages
			dc.OnMessage(func(msg pion.DataChannelMessage) {
				// Parse and save the message
				var parsed map[string]any
				if err := json.Unmarshal(msg.Data, &parsed); err == nil {
					msgType := parsed["type"].(string)

					// Save message from Azure
					saveMessage(msgType, "azure_to_browser", parsed)

					// Log only important messages
					if msgType == "error" {
						if errInfo, ok := parsed["error"].(map[string]any); ok {
							log.Printf("❌ Azure error: %v - %v", errInfo["type"], errInfo["message"])
						}
					} else if msgType == "conversation.item.created" {
						// Extract and log conversation content
						if item, ok := parsed["item"].(map[string]any); ok {
							if role, ok := item["role"].(string); ok && role == "assistant" {
								if formatted, ok := item["formatted"].(map[string]any); ok {
									if transcript, ok := formatted["transcript"].(string); ok {
										log.Printf("🤖 Assistant: %s", transcript)
									}
								}
							}
						}
					}
				}

				// Forward to browser
				if browserDataChannel != nil && browserDataChannel.ReadyState() == pion.DataChannelStateOpen {
					if err := browserDataChannel.Send(msg.Data); err != nil {
						log.Printf("❌ Failed to forward to browser: %v", err)
					}
				}
			})

			// Send initial session config
			go func() {
				time.Sleep(2 * time.Second)
				sessionConfig := map[string]any{
					"type": "session.update",
					"session": map[string]any{
						"instructions":        "You are a helpful AI assistant. Please respond briefly and conversationally.",
						"voice":               "alloy",
						"input_audio_format":  "pcm16", // Azure expects pcm16 format specification
						"output_audio_format": "pcm16", // Azure expects pcm16 format specification
						"input_audio_transcription": map[string]any{
							"model": "whisper-1",
						},
						"turn_detection": map[string]any{
							"type":                "server_vad",
							"threshold":           0.5,
							"prefix_padding_ms":   300,
							"silence_duration_ms": 700,
							"create_response":     true,
						},
					},
				}
				configJSON, _ := json.Marshal(sessionConfig)
				if err := dc.Send(configJSON); err != nil {
					log.Printf("❌ Failed to send session config: %v", err)
				} else {
					log.Printf("✅ Sent initial session configuration to Azure")
				}
			}()
		},
		OnTrack: func(track *pion.TrackRemote, receiver *pion.RTPReceiver) {
			log.Printf("🎵 Azure audio track received: %s", track.Codec().MimeType)
			// Forward Azure audio to browser
			go forwardAzureToBrowser(track)
		},
	}

	// Connect to Azure
	if err := webrtc.EnhancedHeadlessConnect(ctx, azureOpts); err != nil {
		log.Printf("❌ Azure connection error: %v", err)
	}

	log.Printf("🤖 Azure connection closed")
}

func forwardBrowserToAzure(track *pion.TrackRemote) {
	log.Printf("🎤 Started forwarding browser audio to Azure")

	for {
		// ReadRTP gives us the full RTP packet
		rtpPacket, _, readErr := track.ReadRTP()
		if readErr != nil {
			if readErr != io.EOF {
				log.Printf("❌ Error reading browser audio: %v", readErr)
			}
			return
		}

		// Save the audio packet to file
		writeAudioSample(rtpPacket)

		// Forward the audio payload to Azure track
		if browserToAzureTrack != nil {
			// Opus uses 20ms packets typically
			sample := media.Sample{
				Data:     rtpPacket.Payload,
				Duration: time.Millisecond * 20,
			}

			if err := browserToAzureTrack.WriteSample(sample); err != nil {
				if err != io.ErrClosedPipe {
					log.Printf("❌ Error forwarding to Azure: %v", err)
				}
			}
		}
	}
}

func forwardAzureToBrowser(track *pion.TrackRemote) {
	log.Printf("🎵 Started forwarding Azure audio to browser")

	for {
		// ReadRTP gives us the full RTP packet
		rtpPacket, _, readErr := track.ReadRTP()
		if readErr != nil {
			if readErr != io.EOF {
				log.Printf("❌ Error reading Azure audio: %v", readErr)
			}
			return
		}

		// Forward the audio payload to browser track
		if azureToBrowserTrack != nil {
			// Opus uses 20ms packets typically
			sample := media.Sample{
				Data:     rtpPacket.Payload,
				Duration: time.Millisecond * 20,
			}

			if err := azureToBrowserTrack.WriteSample(sample); err != nil {
				if err != io.ErrClosedPipe {
					log.Printf("❌ Error forwarding to browser: %v", err)
				}
			}
		}
	}
}

func serveFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Serve static files from frontend directory
	http.FileServer(http.Dir("../frontend/")).ServeHTTP(w, r)
}

func handleAudioFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// List all OGG files in the current directory
	files, err := os.ReadDir(".")
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	var audioFiles []map[string]interface{}
	for _, file := range files {
		if !file.IsDir() && len(file.Name()) > 4 && file.Name()[len(file.Name())-4:] == ".ogg" {
			info, _ := file.Info()
			audioFiles = append(audioFiles, map[string]interface{}{
				"name": file.Name(),
				"size": info.Size(),
				"time": info.ModTime().Format(time.RFC3339),
			})
		}
	}

	data, err := json.Marshal(audioFiles)
	if err != nil {
		http.Error(w, "Failed to encode file list", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func handleAudioDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL path
	filename := r.URL.Path[len("/audio/"):]

	// Security check - prevent directory traversal
	if filename == "" || filename[0] == '.' || filename[0] == '/' {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Serve the OGG file
	http.ServeFile(w, r, filename)
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
