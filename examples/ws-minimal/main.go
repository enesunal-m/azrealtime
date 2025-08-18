package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/enesunal-m/azrealtime"
)

func main() {
    ctx := context.Background()
    cfg := azrealtime.Config{
        ResourceEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
        Deployment:       os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT"),
        APIVersion:       "2025-04-01-preview",
        Credential:       azrealtime.APIKey(os.Getenv("AZURE_OPENAI_API_KEY")), // or Bearer(token)
        DialTimeout:      15 * time.Second,
        Logger: func(event string, fields map[string]any) { log.Printf("%s: %+v", event, fields) },
    }
    client, err := azrealtime.Dial(ctx, cfg); if err != nil { log.Fatal(err) }
    defer client.Close()

    audio := azrealtime.NewAudioAssembler()
    text := azrealtime.NewTextAssembler()

    client.OnResponseAudioDelta(func(e azrealtime.ResponseAudioDelta) { _ = audio.OnDelta(e) })
    client.OnResponseAudioDone(func(e azrealtime.ResponseAudioDone) {
        pcm := audio.OnDone(e.ResponseID)
        wav := azrealtime.WAVFromPCM16Mono(pcm, azrealtime.DefaultSampleRate)
        _ = os.WriteFile("out.wav", wav, 0644)
        fmt.Println("Wrote out.wav")
    })
    client.OnResponseTextDelta(func(e azrealtime.ResponseTextDelta) { text.OnDelta(e) })
    client.OnResponseTextDone(func(e azrealtime.ResponseTextDone) {
        fmt.Println("Assistant:", text.OnDone(e))
    })

    _ = client.SessionUpdate(ctx, azrealtime.Session{
        Voice:             azrealtime.Ptr("verse"),
        InputAudioFormat:  azrealtime.Ptr("pcm16"),
        OutputAudioFormat: azrealtime.Ptr("pcm16"),
        TurnDetection: &azrealtime.TurnDetection{
            Type: "server_vad", Threshold: 0.5, PrefixPaddingMS: 300, SilenceDurationMS: 200, CreateResponse: true,
        },
    })

    _, err = client.CreateResponse(ctx, azrealtime.CreateResponseOptions{
        Modalities: []string{"audio", "text"},
        Prompt:     "Please introduce yourself briefly.",
    })
    if err != nil { log.Fatal(err) }

    time.Sleep(5 * time.Second)
}
