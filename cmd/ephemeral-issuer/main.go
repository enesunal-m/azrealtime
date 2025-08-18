// Minimal server that mints ephemeral keys for browser WebRTC clients.
// Features: optional OIDC (Entra ID) verification for callers and simple CORS.
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    oidc "github.com/coreos/go-oidc/v3/oidc"
    "github.com/golang-jwt/jwt/v5"
    "github.com/MicahParks/keyfunc/v2"

    "github.com/enesunal-m/azrealtime/webrtc"
)

type TokenResponse struct {
    SessionID  string `json:"session_id"`
    Ephemeral  string `json:"ephemeral"`
    RegionURL  string `json:"region_url"`
    Deployment string `json:"deployment"`
}

type server struct {
    endpoint   string
    apiKey     string
    deployment string
    region     string
    apiVersion string
    voice      string

    // OIDC config
    tokenType string // "id" (ID token) or "access" (JWT access token)
    issuer    string
    audience  string
    verifier  *oidc.IDTokenVerifier
    jwks      *keyfunc.JWKS

    // CORS
    allowedOrigins []string
}

func main() {
    s := &server{
        endpoint:   must("AZURE_OPENAI_ENDPOINT"),
        apiKey:     must("AZURE_OPENAI_API_KEY"),
        deployment: must("AZURE_OPENAI_REALTIME_DEPLOYMENT"),
        region:     must("AZURE_OPENAI_REGION"),
        apiVersion: env("AZURE_OPENAI_API_VERSION", "2025-04-01-preview"),
        voice:      env("AZURE_OPENAI_VOICE", "verse"),
    }

    // OIDC setup
    if iss := os.Getenv("OIDC_ISSUER"); iss != "" {
        aud := must("OIDC_AUDIENCE")
        s.issuer = iss
        s.audience = aud
        s.tokenType = env("OIDC_TOKEN_TYPE", "access") // "id" or "access"

        prov, err := oidc.NewProvider(context.Background(), iss)
        if err != nil { log.Fatalf("oidc provider: %v", err) }

        if s.tokenType == "id" {
            s.verifier = prov.Verifier(&oidc.Config{ClientID: aud})
            log.Println("OIDC (ID token) enabled", iss, "aud", aud)
        } else {
            // Access token: load JWKS
            var disc struct{ JWKSURI string `json:"jwks_uri"` }
            if err := prov.Claims(&disc); err != nil || disc.JWKSURI == "" {
                log.Fatalf("failed to discover jwks_uri: %v", err)
            }
            jwks, err := keyfunc.Get(disc.JWKSURI, keyfunc.Options{
                RefreshInterval: time.Hour,
                RefreshTimeout:  10 * time.Second,
            })
            if err != nil { log.Fatalf("jwks: %v", err) }
            s.jwks = jwks
            log.Println("OIDC (access token) enabled", iss, "aud", aud)
        }
    } else {
        log.Println("OIDC disabled")
    }

    if ao := os.Getenv("CORS_ALLOWED_ORIGINS"); ao != "" {
        s.allowedOrigins = splitCSV(ao)
        log.Println("CORS allowed origins:", s.allowedOrigins)
    }

    mux := http.NewServeMux()
    mux.Handle("/token", s.cors(s.auth(http.HandlerFunc(s.handleToken))))
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { 
        w.WriteHeader(200)
        if _, err := w.Write([]byte("ok")); err != nil {
            log.Printf("Failed to write health check response: %v", err)
        }
    })

    addr := env("ADDR", ":8080")
    log.Println("ephemeral-issuer on", addr)
    log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) handleToken(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second); defer cancel()
    sessionID, eph, err := webrtc.MintEphemeralKey(ctx, s.endpoint, s.apiVersion, s.deployment, s.apiKey, s.voice)
    if err != nil {
        log.Println("mint error:", err)
        http.Error(w, "mint failed", http.StatusBadGateway)
        return
    }
    if err := json.NewEncoder(w).Encode(TokenResponse{
        SessionID:  sessionID,
        Ephemeral:  eph,
        RegionURL:  webrtc.RegionWebRTCURL(s.region),
        Deployment: s.deployment,
    }); err != nil {
        log.Printf("Failed to encode token response: %v", err)
    }
}

// Middleware: OIDC auth
func (s *server) auth(next http.Handler) http.Handler {
    if s.issuer == "" { return next }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
            http.Error(w, "missing bearer", http.StatusUnauthorized); return
        }
        raw := strings.TrimSpace(auth[len("Bearer "):])
        if s.tokenType == "id" {
            if s.verifier == nil {
                http.Error(w, "verifier not initialized", http.StatusInternalServerError); return
            }
            if _, err := s.verifier.Verify(r.Context(), raw); err != nil {
                http.Error(w, "invalid token", http.StatusUnauthorized); return
            }
        } else {
            if s.jwks == nil {
                http.Error(w, "jwks not initialized", http.StatusInternalServerError); return
            }
            tok, err := jwt.Parse(raw, s.jwks.Keyfunc, jwt.WithAudience(s.audience), jwt.WithIssuer(s.issuer))
            if err != nil || !tok.Valid {
                http.Error(w, "invalid token", http.StatusUnauthorized); return
            }
        }
        next.ServeHTTP(w, r)
    })
}

// Middleware: CORS
func (s *server) cors(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if origin != "" && (len(s.allowedOrigins) == 0 || contains(s.allowedOrigins, origin) || contains(s.allowedOrigins, "*")) {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Vary", "Origin")
            w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        }
        if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
        next.ServeHTTP(w, r)
    })
}

// helpers
func must(k string) string { v := os.Getenv(k); if v == "" { log.Fatalf("missing env %s", k) }; return v }
func env(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }
func splitCSV(s string) []string { parts := strings.Split(s, ","); out := make([]string,0,len(parts)); for _, p := range parts { if t := strings.TrimSpace(p); t != "" { out = append(out, t) } }; return out }
func contains(a []string, v string) bool { for _, x := range a { if x==v {return true} }; return false }
