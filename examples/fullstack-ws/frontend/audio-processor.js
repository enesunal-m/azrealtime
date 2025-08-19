// Audio Worklet Processor for real-time PCM16 audio processing
class AudioProcessor extends AudioWorkletProcessor {
    constructor() {
        super();
        this.isRecording = false;
        this.targetSampleRate = 24000; // Azure OpenAI expects 24kHz
        this.inputSampleRate = 48000;  // Browser typically gives us 48kHz
        this.resampleBuffer = [];
        this.resampleIndex = 0;
        
        // Listen for messages from main thread
        this.port.onmessage = (event) => {
            if (event.data.type === 'setRecording') {
                this.isRecording = event.data.recording;
            }
        };
    }
    
    process(inputs, outputs, parameters) {
        const input = inputs[0];
        
        if (input && input.length > 0 && this.isRecording) {
            const inputData = input[0]; // Get first channel (mono)
            
            if (inputData && inputData.length > 0) {
                // Log buffer size occasionally for debugging
                if (Math.random() < 0.001) { // Log ~0.1% of the time
                    console.log(`AudioWorklet buffer size: ${inputData.length}, sampleRate: ${sampleRate}`);
                }
                
                // Resample from 48kHz to 24kHz (simple decimation)
                const resampledData = this.resampleAudio(inputData);
                
                if (resampledData && resampledData.length > 0) {
                    // Convert Float32 to PCM16
                    const pcm16Data = this.convertToPCM16(resampledData);
                    
                    // Send PCM16 data to main thread
                    this.port.postMessage({
                        type: 'audioData',
                        data: pcm16Data
                    });
                }
            }
        }
        
        // Keep the processor alive
        return true;
    }
    
    resampleAudio(inputData) {
        // Simple decimation: take every other sample to go from 48kHz to 24kHz
        const ratio = this.inputSampleRate / this.targetSampleRate;
        const outputLength = Math.floor(inputData.length / ratio);
        const output = new Float32Array(outputLength);
        
        for (let i = 0; i < outputLength; i++) {
            const sourceIndex = Math.floor(i * ratio);
            output[i] = inputData[sourceIndex];
        }
        
        return output;
    }
    
    convertToPCM16(audioData) {
        const pcm16 = new Int16Array(audioData.length);
        for (let i = 0; i < audioData.length; i++) {
            const sample = Math.max(-1, Math.min(1, audioData[i]));
            pcm16[i] = sample < 0 ? sample * 0x8000 : sample * 0x7FFF;
        }
        return pcm16;
    }
}

registerProcessor('audio-processor', AudioProcessor);