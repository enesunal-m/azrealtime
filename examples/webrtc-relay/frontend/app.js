let peerConnection = null;
let mediaStream = null;
let dataChannel = null;
let audioElement = null;  // Store audio element reference

const connectBtn = document.getElementById('connectBtn');
const disconnectBtn = document.getElementById('disconnectBtn');
const status = document.getElementById('status');
const log = document.getElementById('log');
const conversation = document.getElementById('conversation');
const updateSessionBtn = document.getElementById('updateSessionBtn');
const voiceSelect = document.getElementById('voiceSelect');
const instructionsInput = document.getElementById('instructionsInput');

function addLog(message, type = 'info') {
    const timestamp = new Date().toLocaleTimeString();
    const logEntry = document.createElement('div');
    logEntry.className = type;
    logEntry.textContent = `[${timestamp}] ${message}`;
    log.appendChild(logEntry);
    log.scrollTop = log.scrollHeight;
}

function addConversationMessage(role, message) {
    const timestamp = new Date().toLocaleTimeString();
    const messageDiv = document.createElement('div');
    messageDiv.style.marginBottom = '10px';
    messageDiv.style.padding = '8px';
    messageDiv.style.borderRadius = '5px';

    if (role === 'user') {
        messageDiv.style.backgroundColor = '#e3f2fd';
        messageDiv.style.borderLeft = '4px solid #2196f3';
        messageDiv.innerHTML = `<strong>🗣️ You (${timestamp}):</strong><br>${message}`;
    } else if (role === 'assistant') {
        messageDiv.style.backgroundColor = '#f3e5f5';
        messageDiv.style.borderLeft = '4px solid #9c27b0';
        messageDiv.innerHTML = `<strong>🤖 Assistant (${timestamp}):</strong><br>${message}`;
    } else {
        messageDiv.style.backgroundColor = '#f5f5f5';
        messageDiv.style.borderLeft = '4px solid #607d8b';
        messageDiv.innerHTML = `<strong>ℹ️ System (${timestamp}):</strong><br>${message}`;
    }

    conversation.appendChild(messageDiv);
    conversation.scrollTop = conversation.scrollHeight;
}

connectBtn.addEventListener('click', async () => {
    try {
        addLog('🎤 Requesting microphone access...', 'info');
        mediaStream = await navigator.mediaDevices.getUserMedia({
            audio: true  // Simplified audio configuration like the working example
        });
        addLog('✅ Microphone access granted', 'success');

        peerConnection = new RTCPeerConnection({
            iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
        });

        // Handle ICE candidates with trickle ICE
        peerConnection.onicecandidate = (event) => {
            if (event.candidate) {
                // Delay sending ICE candidates to ensure peer connection exists on server
                setTimeout(() => {
                    addLog('📤 Sending ICE candidate to server...', 'info');
                    fetch('/ice-candidate', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(event.candidate.toJSON())
                    }).catch(error => {
                        addLog(`❌ ICE candidate error: ${error.message}`, 'error');
                    });
                }, 100);
            } else {
                addLog('🧊 ICE Gathering Complete', 'success');
            }
        };

        // Connection state monitoring
        peerConnection.onconnectionstatechange = () => {
            const state = peerConnection.connectionState;
            addLog(`🔗 Connection state: ${state}`, 'info');
            if (state === 'connected') {
                status.textContent = 'Status: Connected & Sending Audio';
                connectBtn.disabled = true;
                disconnectBtn.disabled = false;
                updateSessionBtn.disabled = false;
                addLog('🎉 SUCCESS! Audio relay is working!', 'success');
            }
        };

        // Handle incoming audio from Azure
        peerConnection.ontrack = (event) => {
            addLog('🎵 Received audio track from Azure relay', 'success');

            // Create or reuse audio element
            if (!audioElement) {
                audioElement = new Audio();
                audioElement.id = 'azureAudio';
                document.body.appendChild(audioElement);
            }

            audioElement.srcObject = event.streams[0];
            audioElement.autoplay = true;
            audioElement.volume = 1.0;

            // Force play in case autoplay is blocked
            audioElement.play().catch(e => {
                addLog('⚠️ Autoplay blocked, click anywhere to enable audio', 'error');
                document.addEventListener('click', () => {
                    audioElement.play();
                }, { once: true });
            });

            // Add audio quality monitoring
            audioElement.onplay = () => {
                addLog('🔊 Audio playback started', 'success');
                addConversationMessage('system', 'Audio playback active');
            };

            audioElement.onerror = (error) => {
                addLog(`❌ Audio playback error: ${error}`, 'error');
            };

            // Monitor audio stats
            setInterval(() => {
                if (peerConnection && peerConnection.connectionState === 'connected') {
                    peerConnection.getStats().then(stats => {
                        stats.forEach(report => {
                            if (report.type === 'inbound-rtp' && report.kind === 'audio') {
                                const packetsLost = report.packetsLost || 0;
                                const packetsReceived = report.packetsReceived || 0;
                                if (packetsReceived > 0 && packetsLost > packetsReceived * 0.05) {
                                    addLog(`⚠️ High packet loss: ${packetsLost}/${packetsReceived}`, 'error');
                                }
                            }
                        });
                    });
                }
            }, 5000);
        };

        // Handle data channel from server
        peerConnection.ondatachannel = (event) => {
            const channel = event.channel;
            addLog(`📡 Received data channel: ${channel.label}`, 'info');
            setupDataChannel(channel);
        };

        // Create data channel
        dataChannel = peerConnection.createDataChannel('realtime-channel', { ordered: true });
        setupDataChannel(dataChannel);

        // Add microphone track
        mediaStream.getTracks().forEach(track => {
            peerConnection.addTrack(track, mediaStream);
        });
        addLog('✅ Audio track added to peer connection', 'success');

        // Create and send offer
        const offer = await peerConnection.createOffer({
            offerToReceiveAudio: true,
            offerToReceiveVideo: false
        });
        await peerConnection.setLocalDescription(offer);
        addLog('📤 Sending offer to server...', 'info');

        const response = await fetch('/offer', {
            method: 'POST',
            headers: { 'Content-Type': 'application/sdp' },
            body: peerConnection.localDescription.sdp
        });

        if (!response.ok) {
            throw new Error('HTTP ' + response.status);
        }

        const answerSdp = await response.text();
        addLog('📥 Received answer from server', 'success');
        await peerConnection.setRemoteDescription({ type: 'answer', sdp: answerSdp });
        addLog('✅ WebRTC connection established', 'success');

    } catch (error) {
        addLog('❌ Error: ' + error.message, 'error');
        console.error(error);
    }
});

function setupDataChannel(channel) {
    channel.onopen = () => {
        addLog(`📡 Data channel opened: ${channel.label}`, 'success');
    };

    channel.onmessage = async (event) => {
        try {
            let data = event.data;

            // Handle different data types (string vs Blob vs ArrayBuffer)
            if (data instanceof ArrayBuffer) {
                // Convert ArrayBuffer to string
                const decoder = new TextDecoder();
                data = decoder.decode(data);
            } else if (data instanceof Blob) {
                data = await data.text();
            }

            const message = JSON.parse(data);
            addLog(`📨 Azure: ${message.type}`, 'info');

            // Handle different message types
            if (message.type === 'session.created' || message.type === 'session.updated') {
                addLog('⚙️ Azure session configured', 'success');
                addConversationMessage('system', 'Session configured and ready');
            } else if (message.type === 'conversation.item.created') {
                if (message.item && message.item.role === 'user' && message.item.formatted && message.item.formatted.transcript) {
                    addConversationMessage('user', message.item.formatted.transcript);
                }
            } else if (message.type === 'response.created') {
                addConversationMessage('assistant', 'Thinking...');
            } else if (message.type === 'response.audio_transcript.done') {
                if (message.transcript) {
                    addConversationMessage('assistant', message.transcript);
                }
            } else if (message.type === 'conversation.item.input_audio_transcription.completed') {
                if (message.transcript) {
                    addConversationMessage('user', message.transcript);
                }
            } else if (message.type === 'response.done') {
                addLog('✅ Azure response complete', 'success');
            } else if (message.type === 'error') {
                addLog(`❌ Azure error: ${message.error?.message || 'Unknown error'}`, 'error');
                addConversationMessage('system', `Error: ${message.error?.message || 'Unknown error'}`);
            } else {
                console.log(message);
            }
        } catch (e) {
            addLog(`⚠️ Error parsing message: ${e.message}`, 'error');
            console.error('Message parsing error:', e);
        }
    };

    channel.onerror = (error) => {
        addLog(`❌ Data channel error: ${error}`, 'error');
    };
}

updateSessionBtn.addEventListener('click', () => {
    if (dataChannel && dataChannel.readyState === 'open') {
        const sessionConfig = {
            type: 'session.update',
            session: {
                voice: voiceSelect.value,
                instructions: instructionsInput.value,
                input_audio_format: 'pcm16',   // Azure expects pcm16 format specification
                output_audio_format: 'pcm16',  // Azure expects pcm16 format specification
                turn_detection: {
                    type: 'server_vad',
                    threshold: 0.5,
                    prefix_padding_ms: 300,
                    silence_duration_ms: 700,
                    create_response: true
                }
            }
        };

        dataChannel.send(JSON.stringify(sessionConfig));
        addLog('⚙️ Session configuration sent', 'success');
    } else {
        addLog('❌ Data channel not ready', 'error');
    }
});

disconnectBtn.addEventListener('click', () => {
    if (peerConnection) {
        peerConnection.close();
        peerConnection = null;
    }
    if (mediaStream) {
        mediaStream.getTracks().forEach(track => track.stop());
        mediaStream = null;
    }
    if (audioElement) {
        audioElement.pause();
        audioElement.srcObject = null;
    }
    dataChannel = null;
    status.textContent = 'Status: Disconnected';
    connectBtn.disabled = false;
    disconnectBtn.disabled = true;
    updateSessionBtn.disabled = true;
    addLog('🔌 Disconnected', 'info');
});