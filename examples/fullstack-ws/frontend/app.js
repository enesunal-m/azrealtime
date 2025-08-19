class RealtimeClient {
    constructor() {
        this.ws = null;
        this.mediaRecorder = null;
        this.audioContext = null;
        this.analyser = null;
        this.isRecording = false;
        this.isConnected = false;
        this.recordingStartTime = null;
        this.recordingTimer = null;
        this.responseCount = 0;
        
        // Audio settings
        this.sampleRate = 24000;
        this.channels = 1;
        
        // Current conversation turn
        this.currentUserTurn = null;
        this.currentAssistantTurn = null;
        
        this.initializeUI();
        this.initializeAudio();
    }
    
    initializeUI() {
        // Connection controls
        this.connectBtn = document.getElementById('connectBtn');
        this.disconnectBtn = document.getElementById('disconnectBtn');
        this.statusDot = document.getElementById('statusDot');
        this.statusText = document.getElementById('statusText');
        this.loadingSpinner = document.getElementById('loadingSpinner');
        
        // Session config
        this.voiceSelect = document.getElementById('voiceSelect');
        this.languageSelect = document.getElementById('languageSelect');
        this.vadTypeSelect = document.getElementById('vadTypeSelect');
        this.thresholdSlider = document.getElementById('thresholdSlider');
        this.thresholdValue = document.getElementById('thresholdValue');
        this.instructionsInput = document.getElementById('instructionsInput');
        this.updateSessionBtn = document.getElementById('updateSessionBtn');
        
        // Voice controls
        this.startRecordingBtn = document.getElementById('startRecordingBtn');
        this.stopRecordingBtn = document.getElementById('stopRecordingBtn');
        this.createResponseBtn = document.getElementById('createResponseBtn');
        
        // Display areas
        this.conversationArea = document.getElementById('conversationArea');
        this.debugMessages = document.getElementById('debugMessages');
        
        // Metrics
        this.recordingTimeEl = document.getElementById('recordingTime');
        this.audioLevelEl = document.getElementById('audioLevel');
        this.responseCountEl = document.getElementById('responseCount');
        
        // Event listeners
        this.connectBtn.addEventListener('click', () => this.connect());
        this.disconnectBtn.addEventListener('click', () => this.disconnect());
        this.updateSessionBtn.addEventListener('click', () => this.updateSession());
        this.startRecordingBtn.addEventListener('click', () => this.startRecording());
        this.stopRecordingBtn.addEventListener('click', () => this.stopRecording());
        this.createResponseBtn.addEventListener('click', () => this.createResponse());
        
        this.thresholdSlider.addEventListener('input', (e) => {
            this.thresholdValue.textContent = e.target.value;
        });
        
        document.getElementById('clearConversationBtn').addEventListener('click', () => {
            this.conversationArea.innerHTML = '<p style="text-align: center; color: #7f8c8d; font-style: italic;">Conversation cleared...</p>';
        });
        
        document.getElementById('clearDebugBtn').addEventListener('click', () => {
            this.debugMessages.innerHTML = '';
        });
        
        document.getElementById('toggleDebugBtn').addEventListener('click', (e) => {
            const debugSection = this.debugMessages.parentElement;
            if (debugSection.style.display === 'none') {
                debugSection.style.display = 'block';
                e.target.textContent = 'Hide Debug';
            } else {
                debugSection.style.display = 'none';
                e.target.textContent = 'Show Debug';
            }
        });
    }
    
    async initializeAudio() {
        try {
            const stream = await navigator.mediaDevices.getUserMedia({
                audio: {
                    sampleRate: this.sampleRate,
                    channelCount: this.channels,
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true
                }
            });
            
            // Verify the actual sample rate we got
            const track = stream.getAudioTracks()[0];
            const settings = track.getSettings();
            console.log('Audio track settings:', settings);
            if (settings.sampleRate !== this.sampleRate) {
                console.warn(`Requested ${this.sampleRate}Hz but got ${settings.sampleRate}Hz`);
            }
            
            this.audioContext = new (window.AudioContext || window.webkitAudioContext)({
                sampleRate: this.sampleRate
            });
            
            // Set up audio analysis
            this.analyser = this.audioContext.createAnalyser();
            this.analyser.fftSize = 256;
            
            // Load and set up AudioWorklet (modern replacement for ScriptProcessorNode)
            try {
                await this.audioContext.audioWorklet.addModule('audio-processor.js');
                
                this.audioWorkletNode = new AudioWorkletNode(this.audioContext, 'audio-processor');
                
                // Handle messages from audio worklet
                this.audioWorkletNode.port.onmessage = (event) => {
                    if (event.data.type === 'audioData' && this.isConnected && this.isRecording) {
                        this.sendAudioData(event.data.data);
                    }
                };
                
                const source = this.audioContext.createMediaStreamSource(stream);
                source.connect(this.analyser);
                source.connect(this.audioWorkletNode);
                
                this.addDebugMessage('🎤 Modern audio worklet initialized', 'success');
            } catch (workletError) {
                // Fallback to ScriptProcessorNode for older browsers
                this.addDebugMessage('⚠️ AudioWorklet not supported, using fallback', 'info');
                
                this.scriptProcessor = this.audioContext.createScriptProcessor(2048, 1, 1);
                
                const source = this.audioContext.createMediaStreamSource(stream);
                source.connect(this.analyser);
                source.connect(this.scriptProcessor);
                this.scriptProcessor.connect(this.audioContext.destination);
                
                this.scriptProcessor.onaudioprocess = (event) => {
                    if (this.isRecording && this.isConnected) {
                        const inputData = event.inputBuffer.getChannelData(0);
                        this.processRealTimeAudio(inputData);
                    }
                };
                
                this.addDebugMessage('🎤 Fallback audio processor initialized', 'success');
            }
            
        } catch (error) {
            this.addDebugMessage(`❌ Failed to initialize audio: ${error.message}`, 'error');
        }
    }
    
    processRealTimeAudio(inputData) {
        try {
            // Convert Float32 audio data to PCM16
            const pcm16Data = this.convertToPCM16(inputData);
            this.sendAudioData(pcm16Data);
        } catch (error) {
            console.warn('Error processing real-time audio:', error);
        }
    }
    
    sendAudioData(pcm16Data) {
        try {
            // Ensure we have the right data type - pcm16Data should be Int16Array from AudioWorklet
            let buffer;
            let int16Array;
            if (pcm16Data instanceof Int16Array) {
                int16Array = pcm16Data;
                buffer = pcm16Data.buffer;
            } else if (pcm16Data instanceof ArrayBuffer) {
                int16Array = new Int16Array(pcm16Data);
                buffer = pcm16Data;
            } else {
                console.warn('Unexpected audio data type:', typeof pcm16Data);
                return;
            }
            
            // Check if audio contains actual sound (not just silence)
            let hasSound = false;
            const threshold = 100; // Minimum amplitude threshold
            for (let i = 0; i < int16Array.length; i++) {
                if (Math.abs(int16Array[i]) > threshold) {
                    hasSound = true;
                    break;
                }
            }
            
            // Log audio statistics periodically (every 100 chunks)
            this.audioChunkCount = (this.audioChunkCount || 0) + 1;
            if (this.audioChunkCount % 100 === 0) {
                const maxAmplitude = Math.max(...Array.from(int16Array).map(Math.abs));
                console.log(`Audio chunk #${this.audioChunkCount}: size=${int16Array.length}, max_amplitude=${maxAmplitude}, has_sound=${hasSound}`);
            }
            
            // Encode as base64 and send immediately
            const base64Data = this.arrayBufferToBase64(buffer);
            this.sendMessage('audio_data', {
                data: base64Data,
                format: 'pcm16'
            });
        } catch (error) {
            console.warn('Error sending audio data:', error);
        }
    }
    
    convertToPCM16(audioData) {
        // audioData is already Float32Array from ScriptProcessorNode
        // No resampling needed as we requested 24kHz from getUserMedia
        
        // Convert to 16-bit PCM
        const pcm16 = new Int16Array(audioData.length);
        for (let i = 0; i < audioData.length; i++) {
            const sample = Math.max(-1, Math.min(1, audioData[i]));
            pcm16[i] = sample < 0 ? sample * 0x8000 : sample * 0x7FFF;
        }
        
        return pcm16.buffer;
    }
    
    resampleAudio(audioData, fromSampleRate, toSampleRate) {
        const ratio = fromSampleRate / toSampleRate;
        const newLength = Math.round(audioData.length / ratio);
        const result = new Float32Array(newLength);
        
        for (let i = 0; i < newLength; i++) {
            const index = i * ratio;
            const indexInt = Math.floor(index);
            const indexFrac = index - indexInt;
            
            if (indexInt + 1 < audioData.length) {
                result[i] = audioData[indexInt] * (1 - indexFrac) + audioData[indexInt + 1] * indexFrac;
            } else {
                result[i] = audioData[indexInt];
            }
        }
        
        return result;
    }
    
    arrayBufferToBase64(buffer) {
        const bytes = new Uint8Array(buffer);
        let binary = '';
        for (let i = 0; i < bytes.byteLength; i++) {
            binary += String.fromCharCode(bytes[i]);
        }
        return btoa(binary);
    }
    
    base64ToArrayBuffer(base64) {
        const binary = atob(base64);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return bytes.buffer;
    }
    
    connect() {
        if (this.isConnected) return;
        
        this.showLoading(true);
        this.updateStatus('connecting', 'Connecting...');
        
        const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`;
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onopen = () => {
            this.isConnected = true;
            this.showLoading(false);
            this.updateStatus('connected', 'Connected');
            this.updateUI();
            this.startSession();
            this.addDebugMessage('✅ WebSocket connected', 'success');
        };
        
        this.ws.onclose = (event) => {
            this.isConnected = false;
            this.showLoading(false);
            this.updateStatus('disconnected', 'Disconnected');
            this.updateUI();
            
            if (this.isRecording) {
                this.stopRecording();
            }
            
            this.addDebugMessage(`❌ WebSocket disconnected (code: ${event.code}, reason: ${event.reason || 'Unknown'})`, 'error');
            
            // Auto-reconnect after a short delay if not manually disconnected
            if (event.code !== 1000) { // 1000 = normal closure
                setTimeout(() => {
                    if (!this.isConnected) {
                        this.addDebugMessage('🔄 Attempting to reconnect...', 'info');
                        this.connect();
                    }
                }, 3000);
            }
        };
        
        this.ws.onerror = (error) => {
            this.showLoading(false);
            this.updateStatus('error', 'Connection Error');
            this.addDebugMessage(`❌ WebSocket error: ${error.message || 'Unknown error'}`, 'error');
        };
        
        this.ws.onmessage = (event) => {
            this.handleMessage(JSON.parse(event.data));
        };
    }
    
    disconnect() {
        if (this.isRecording) {
            this.stopRecording();
        }
        
        if (this.ws) {
            this.sendMessage('end_session');
            this.ws.close();
            this.ws = null;
        }
        
        this.isConnected = false;
        this.updateStatus('disconnected', 'Disconnected');
        this.updateUI();
    }
    
    startSession() {
        const config = {
            voice: this.voiceSelect.value,
            instructions: this.instructionsInput.value || "You are a helpful AI assistant. If the user speaks Turkish, respond in Turkish. If the user speaks English, respond in English. Be natural and conversational.",
            input_audio_format: "pcm16",
            output_audio_format: "pcm16",
            turn_detection: {
                type: "server_vad",  // Match working example exactly
                threshold: 0.5,      // Match working example exactly  
                create_response: true,     // Let server auto-create responses
                interrupt_response: true,  // Allow interrupting - like working example
                prefix_padding_ms: 300,
                silence_duration_ms: 500   // Match working example exactly
            },
            transcription: {
                model: "whisper-1",
                language: this.languageSelect.value
            }
        };
        
        this.sendMessage('start_session', config);
        this.addDebugMessage(`🚀 Starting real-time session with language: ${config.transcription.language}`, 'info');
    }
    
    updateSession() {
        const config = {
            voice: this.voiceSelect.value,
            instructions: this.instructionsInput.value || "You are a helpful AI assistant.",
            turn_detection: {
                type: "server_vad",  // Match working example exactly
                threshold: 0.5,      // Match working example exactly
                create_response: true,     // Let server auto-create responses
                interrupt_response: true,  // Allow interrupting - like working example
                prefix_padding_ms: 300,
                silence_duration_ms: 500   // Match working example exactly
            },
            transcription: {
                model: "whisper-1",
                language: this.languageSelect.value
            }
        };
        
        this.sendMessage('update_session', config);
        this.addDebugMessage('⚙️ Updating session configuration...', 'info');
    }
    
    startRecording() {
        if (!this.isConnected || (!this.audioWorkletNode && !this.scriptProcessor)) return;
        
        this.isRecording = true;
        this.recordingStartTime = Date.now();
        
        // Tell AudioWorklet to start recording (if using modern approach)
        if (this.audioWorkletNode) {
            this.audioWorkletNode.port.postMessage({
                type: 'setRecording',
                recording: true
            });
        }
        
        this.startRecordingBtn.classList.add('recording');
        this.updateUI();
        this.startRecordingTimer();
        this.startAudioLevelMonitoring();
        
        // Start new user turn
        this.currentUserTurn = this.addConversationTurn('user');
        
        this.addDebugMessage('🎤 Real-time streaming started', 'info');
    }
    
    stopRecording() {
        if (!this.isRecording) return;
        
        this.isRecording = false;
        
        // Tell AudioWorklet to stop recording (if using modern approach)
        if (this.audioWorkletNode) {
            this.audioWorkletNode.port.postMessage({
                type: 'setRecording',
                recording: false
            });
        }
        
        this.startRecordingBtn.classList.remove('recording');
        this.updateUI();
        this.stopRecordingTimer();
        this.stopAudioLevelMonitoring();
        
        this.addDebugMessage('⏹️ Real-time streaming stopped', 'info');
    }
    
    createResponse() {
        this.sendMessage('create_response', {
            modalities: ['text', 'audio']
        });
        
        this.addDebugMessage('🤖 Requesting AI response...', 'info');
    }
    
    startRecordingTimer() {
        this.recordingTimer = setInterval(() => {
            if (this.recordingStartTime) {
                const elapsed = Math.floor((Date.now() - this.recordingStartTime) / 1000);
                this.recordingTimeEl.textContent = `${elapsed}s`;
            }
        }, 1000);
    }
    
    stopRecordingTimer() {
        if (this.recordingTimer) {
            clearInterval(this.recordingTimer);
            this.recordingTimer = null;
        }
        this.recordingTimeEl.textContent = '0s';
    }
    
    startAudioLevelMonitoring() {
        if (!this.analyser) return;
        
        const dataArray = new Uint8Array(this.analyser.frequencyBinCount);
        
        const updateLevel = () => {
            if (!this.isRecording) return;
            
            this.analyser.getByteFrequencyData(dataArray);
            
            // Calculate RMS
            let sum = 0;
            for (let i = 0; i < dataArray.length; i++) {
                sum += dataArray[i] * dataArray[i];
            }
            const rms = Math.sqrt(sum / dataArray.length);
            const level = Math.min(100, Math.floor((rms / 255) * 100));
            
            this.audioLevelEl.textContent = `${level}%`;
            
            requestAnimationFrame(updateLevel);
        };
        
        updateLevel();
    }
    
    stopAudioLevelMonitoring() {
        this.audioLevelEl.textContent = '0%';
    }
    
    sendMessage(type, data = null) {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            if (type !== 'audio_data') { // Don't spam logs with audio data errors
                this.addDebugMessage(`⚠️ Cannot send ${type}: WebSocket not connected`, 'error');
            }
            return false;
        }
        
        try {
            const message = { type, data };
            this.ws.send(JSON.stringify(message));
            return true;
        } catch (error) {
            this.addDebugMessage(`❌ Failed to send ${type}: ${error.message}`, 'error');
            return false;
        }
    }
    
    handleMessage(message) {
        switch (message.type) {
            case 'session_started':
                this.addDebugMessage('✅ Session started successfully', 'success');
                break;
                
            case 'session_error':
                this.addDebugMessage(`❌ Session error: ${message.data.message}`, 'error');
                if (message.data.details) {
                    this.addDebugMessage(`Details: ${message.data.details}`, 'error');
                }
                break;
                
            case 'connection_lost':
                this.handleConnectionLost(message.data);
                break;
                
            case 'reconnect_success':
                this.handleReconnectSuccess(message.data);
                break;
                
            case 'reconnect_failed':
                this.handleReconnectFailed(message.data);
                break;
                
            case 'vad_event':
                this.handleVADEvent(message.data);
                break;
                
            case 'transcript':
                this.handleTranscript(message.data);
                break;
                
            case 'text_delta':
                this.handleTextDelta(message.data);
                break;
                
            case 'text_done':
                this.handleTextDone(message.data);
                break;
                
            case 'audio_delta':
                this.handleAudioDelta(message.data);
                break;
                
            case 'audio_done':
                this.handleAudioDone(message.data);
                break;
                
            case 'response_created':
                this.addDebugMessage(`🤖 Response created: ${message.data.response_id}`, 'success');
                break;
                
            case 'response_done':
                this.addDebugMessage(`✅ Response completed: ${message.data.response_id}`, 'success');
                break;
                
            case 'error':
                this.addDebugMessage(`❌ API Error: ${message.data.message}`, 'error');
                break;
                
            default:
                this.addDebugMessage(`📨 Unknown message: ${message.type}`, 'info');
        }
    }
    
    handleConnectionLost(data) {
        // Stop streaming immediately
        if (this.isRecording) {
            this.stopRecording();
        }
        
        // Update UI to show connection lost
        this.updateStatus('error', 'Reconnecting...');
        this.addDebugMessage(`🔌 ${data.message}`, 'error');
        
        // Show user-friendly message in current turn
        if (this.currentUserTurn) {
            const messageEl = document.createElement('div');
            messageEl.style.color = '#e74c3c';
            messageEl.style.fontStyle = 'italic';
            messageEl.style.marginTop = '10px';
            messageEl.textContent = '⚠️ Connection to Azure lost. Reconnecting...';
            this.currentUserTurn.appendChild(messageEl);
        }
        
        // Attempt automatic reconnection after a short delay
        this.addDebugMessage('🔄 Attempting automatic reconnection in 2 seconds...', 'info');
        setTimeout(() => {
            if (this.isConnected) {
                this.reconnectAzure();
            }
        }, 2000);
    }
    
    reconnectAzure() {
        this.addDebugMessage('🔄 Reconnecting to Azure...', 'info');
        this.updateStatus('connecting', 'Reconnecting...');
        
        // Send reconnect message to server
        this.sendMessage('reconnect_azure', {
            voice: this.voiceSelect.value,
            instructions: this.instructionsInput.value || "You are a helpful AI assistant. Respond naturally and conversationally.",
            turn_detection: {
                type: this.vadTypeSelect.value,
                threshold: parseFloat(this.thresholdSlider.value),
                create_response: true,
                interrupt_response: true,
                prefix_padding_ms: 300,
                silence_duration_ms: 700
            },
            transcription: {
                model: "whisper-1",
                language: this.languageSelect.value
            }
        });
    }
    
    handleReconnectSuccess(data) {
        this.updateStatus('connected', 'Connected');
        this.addDebugMessage('✅ Azure reconnection successful', 'success');
        
        // Update the user turn message if it exists
        if (this.currentUserTurn) {
            const errorMsg = this.currentUserTurn.querySelector('div[style*="color: #e74c3c"]');
            if (errorMsg) {
                errorMsg.style.color = '#27ae60';
                errorMsg.textContent = '✅ Connection restored successfully!';
            }
        }
    }
    
    handleReconnectFailed(data) {
        this.updateStatus('error', 'Reconnection Failed');
        this.addDebugMessage(`❌ Azure reconnection failed: ${data.message}`, 'error');
        
        // Update the user turn message if it exists
        if (this.currentUserTurn) {
            const errorMsg = this.currentUserTurn.querySelector('div[style*="color: #e74c3c"]');
            if (errorMsg) {
                errorMsg.textContent = '❌ Reconnection failed. Please manually reconnect.';
            }
        }
    }

    handleVADEvent(data) {
        switch (data.event) {
            case 'speech_started':
                this.addDebugMessage(`🎤 Speech detected at ${data.audio_start_ms}ms`, 'info');
                break;
            case 'speech_stopped':
                this.addDebugMessage(`⏹️ Speech ended at ${data.audio_end_ms}ms`, 'info');
                break;
            case 'committed':
                this.addDebugMessage(`✅ Audio committed (item: ${data.item_id})`, 'success');
                break;
        }
    }
    
    handleTranscript(data) {
        if (this.currentUserTurn) {
            const transcriptEl = this.currentUserTurn.querySelector('.transcript');
            transcriptEl.textContent = `"${data.transcript}"`;
            transcriptEl.style.display = 'block';
        }
        this.addDebugMessage(`🎯 Transcript: "${data.transcript}"`, 'info');
        
        // Azure should automatically generate response due to create_response: true
        // No manual triggering needed - this was causing conflicts
    }
    
    handleTextDelta(data) {
        if (!this.currentAssistantTurn) {
            this.currentAssistantTurn = this.addConversationTurn('assistant');
            this.responseCount++;
            this.responseCountEl.textContent = this.responseCount;
        }
        
        const textEl = this.currentAssistantTurn.querySelector('.response-text');
        textEl.textContent += data.delta;
        
        // Auto-scroll to bottom
        this.conversationArea.scrollTop = this.conversationArea.scrollHeight;
    }
    
    handleTextDone(data) {
        this.addDebugMessage(`📝 Text response complete (${data.text.length} chars)`, 'success');
    }
    
    handleAudioDelta(data) {
        // Audio is being streamed - we'll wait for audio_done to play it
    }
    
    handleAudioDone(data) {
        if (!this.currentAssistantTurn) return;
        
        try {
            // Decode base64 audio data
            const audioBuffer = this.base64ToArrayBuffer(data.audio_data);
            
            // Convert PCM16 to playable audio
            this.createAudioElement(audioBuffer, data.sample_rate);
            
            this.addDebugMessage(`🔊 Audio response complete (${audioBuffer.byteLength} bytes)`, 'success');
        } catch (error) {
            this.addDebugMessage(`❌ Error processing audio: ${error.message}`, 'error');
        }
    }
    
    async createAudioElement(pcm16Buffer, sampleRate) {
        try {
            // Create AudioBuffer from PCM16 data
            const int16Array = new Int16Array(pcm16Buffer);
            const float32Array = new Float32Array(int16Array.length);
            
            // Convert int16 to float32
            for (let i = 0; i < int16Array.length; i++) {
                float32Array[i] = int16Array[i] / 32768.0;
            }
            
            // Create AudioBuffer
            const audioBuffer = this.audioContext.createBuffer(1, float32Array.length, sampleRate);
            audioBuffer.copyToChannel(float32Array, 0);
            
            // Convert to WAV blob
            const wavBlob = this.audioBufferToWav(audioBuffer);
            const audioUrl = URL.createObjectURL(wavBlob);
            
            // Add audio element to conversation
            const audioEl = document.createElement('audio');
            audioEl.controls = true;
            audioEl.src = audioUrl;
            audioEl.style.width = '100%';
            
            if (this.currentAssistantTurn) {
                this.currentAssistantTurn.appendChild(audioEl);
                
                // Auto-play the response
                audioEl.play().catch(e => {
                    console.warn('Auto-play prevented:', e);
                });
            }
            
            // Reset current assistant turn for next response
            this.currentAssistantTurn = null;
            
        } catch (error) {
            this.addDebugMessage(`❌ Error creating audio: ${error.message}`, 'error');
        }
    }
    
    audioBufferToWav(audioBuffer) {
        const length = audioBuffer.length;
        const sampleRate = audioBuffer.sampleRate;
        const buffer = new ArrayBuffer(44 + length * 2);
        const view = new DataView(buffer);
        const channels = audioBuffer.numberOfChannels;
        const samples = audioBuffer.getChannelData(0);
        
        // WAV header
        const writeString = (offset, string) => {
            for (let i = 0; i < string.length; i++) {
                view.setUint8(offset + i, string.charCodeAt(i));
            }
        };
        
        writeString(0, 'RIFF');
        view.setUint32(4, 36 + length * 2, true);
        writeString(8, 'WAVE');
        writeString(12, 'fmt ');
        view.setUint32(16, 16, true);
        view.setUint16(20, 1, true);
        view.setUint16(22, channels, true);
        view.setUint32(24, sampleRate, true);
        view.setUint32(28, sampleRate * 2, true);
        view.setUint16(32, 2, true);
        view.setUint16(34, 16, true);
        writeString(36, 'data');
        view.setUint32(40, length * 2, true);
        
        // Convert samples to 16-bit PCM
        let offset = 44;
        for (let i = 0; i < length; i++) {
            const sample = Math.max(-1, Math.min(1, samples[i]));
            view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7FFF, true);
            offset += 2;
        }
        
        return new Blob([buffer], { type: 'audio/wav' });
    }
    
    addConversationTurn(role) {
        // Clear the placeholder text if it exists
        if (this.conversationArea.querySelector('p')) {
            this.conversationArea.innerHTML = '';
        }
        
        const turnEl = document.createElement('div');
        turnEl.className = `turn ${role}`;
        
        const headerEl = document.createElement('div');
        headerEl.className = 'turn-header';
        
        const roleEl = document.createElement('span');
        roleEl.textContent = role === 'user' ? '👤 You' : '🤖 Assistant';
        
        const timeEl = document.createElement('span');
        timeEl.textContent = new Date().toLocaleTimeString();
        timeEl.style.fontSize = '12px';
        timeEl.style.color = '#999';
        
        headerEl.appendChild(roleEl);
        headerEl.appendChild(timeEl);
        
        const transcriptEl = document.createElement('div');
        transcriptEl.className = 'transcript';
        transcriptEl.style.display = 'none';
        
        const textEl = document.createElement('div');
        textEl.className = 'response-text';
        
        turnEl.appendChild(headerEl);
        turnEl.appendChild(transcriptEl);
        turnEl.appendChild(textEl);
        
        this.conversationArea.appendChild(turnEl);
        this.conversationArea.scrollTop = this.conversationArea.scrollHeight;
        
        return turnEl;
    }
    
    addDebugMessage(message, type = 'info') {
        const messageEl = document.createElement('div');
        messageEl.className = `message ${type}`;
        messageEl.innerHTML = `<span style="color: #666;">[${new Date().toLocaleTimeString()}]</span> ${message}`;
        
        this.debugMessages.appendChild(messageEl);
        this.debugMessages.scrollTop = this.debugMessages.scrollHeight;
        
        // Keep only last 100 messages
        while (this.debugMessages.children.length > 100) {
            this.debugMessages.removeChild(this.debugMessages.firstChild);
        }
    }
    
    updateStatus(status, text) {
        this.statusDot.className = `status-dot ${status}`;
        this.statusText.textContent = text;
    }
    
    showLoading(show) {
        this.loadingSpinner.classList.toggle('hidden', !show);
    }
    
    updateUI() {
        const connected = this.isConnected;
        const recording = this.isRecording;
        
        this.connectBtn.disabled = connected;
        this.disconnectBtn.disabled = !connected;
        this.updateSessionBtn.disabled = !connected;
        this.startRecordingBtn.disabled = !connected || recording;
        this.stopRecordingBtn.disabled = !connected || !recording;
        this.createResponseBtn.disabled = !connected;
        
        this.startRecordingBtn.textContent = recording ? '🎤 Streaming...' : '🎤 Start Streaming';
    }
}

// Initialize the application
document.addEventListener('DOMContentLoaded', () => {
    new RealtimeClient();
});