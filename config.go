package azrealtime

import (
    "net/http"
    "time"
)

// Credential represents an authentication method for Azure OpenAI.
// Implementations must apply the appropriate authentication headers to HTTP requests.
type Credential interface{ apply(h http.Header) }

// APIKey implements Credential using Azure OpenAI API key authentication.
// This is the most common authentication method for Azure OpenAI resources.
type APIKey string

// apply adds the API key to the request headers using the "api-key" header.
func (k APIKey) apply(h http.Header) { if k != "" { h.Set("api-key", string(k)) } }

// Bearer implements Credential using OAuth2 Bearer token authentication.
// Use this when authenticating with Azure AD tokens or other Bearer tokens.
type Bearer string

// apply adds the Bearer token to the Authorization header.
func (b Bearer) apply(h http.Header) { if b != "" { h.Set("Authorization", "Bearer " + string(b)) } }

// Config holds all configuration options for creating an Azure OpenAI Realtime client.
// All fields marked as required must be provided for successful connection.
type Config struct {
    // ResourceEndpoint is the base URL of your Azure OpenAI resource.
    // Format: https://{resource-name}.openai.azure.com
    // Required: Yes
    ResourceEndpoint string
    
    // Deployment is the name of your GPT-4o Realtime deployment.
    // This should match the deployment name configured in Azure OpenAI Studio.
    // Required: Yes
    Deployment       string
    
    // APIVersion specifies the Azure OpenAI API version to use.
    // Recommended: "2025-04-01-preview" (latest as of implementation)
    // Required: Yes
    APIVersion       string
    
    // Credential provides authentication for API requests.
    // Use APIKey for key-based auth or Bearer for token-based auth.
    // Required: Yes
    Credential       Credential
    
    // DialTimeout sets the maximum time to wait for WebSocket connection establishment.
    // If zero, no timeout is applied (not recommended for production).
    // Recommended: 15-30 seconds
    // Required: No
    DialTimeout      time.Duration
    
    // HandshakeHeaders allows adding custom headers to the WebSocket handshake request.
    // Useful for proxy authentication, tracing headers, etc.
    // Required: No
    HandshakeHeaders http.Header
    
    // Logger is called for significant events and can be used for debugging and monitoring.
    // Events include: ws_connected, bad_event_json, and other operational events.
    // The fields parameter contains structured data relevant to each event.
    // Required: No (if nil, no logging occurs)
    Logger           func(event string, fields map[string]any)
    
    // StructuredLogger provides advanced structured logging with configurable levels.
    // If both Logger and StructuredLogger are provided, StructuredLogger takes precedence.
    // Use NewLogger() or NewLoggerFromEnv() to create a structured logger.
    // Required: No (if nil, falls back to Logger or no logging)
    StructuredLogger *Logger
}
