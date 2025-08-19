package webrtc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func SessionsURL(resourceEndpoint, apiVersion string) string {
	if apiVersion == "" {
		apiVersion = "2025-04-01-preview"
	}
	return fmt.Sprintf("%s/openai/realtimeapi/sessions?api-version=%s", resourceEndpoint, apiVersion)
}

type EphemeralResponse struct {
	ID           string `json:"id"`
	ClientSecret struct {
		Value string `json:"value"`
	} `json:"client_secret"`
}

func MintEphemeralKey(ctx context.Context, resourceEndpoint, apiVersion, deployment, apiKey, voice string) (sessionID, ephemeralKey string, err error) {
	url := SessionsURL(resourceEndpoint, apiVersion)
	payload := map[string]any{"model": deployment}
	if voice != "" {
		payload["voice"] = voice
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("mint ephemeral: status %d", resp.StatusCode)
	}
	var er EphemeralResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return "", "", err
	}
	return er.ID, er.ClientSecret.Value, nil
}

func RegionWebRTCURL(region string) string {
	return fmt.Sprintf("https://%s.realtimeapi-preview.ai.azure.com/v1/realtimertc", region)
}
