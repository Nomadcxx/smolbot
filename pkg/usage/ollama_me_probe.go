package usage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const defaultOllamaBaseURL = "https://ollama.com"

type OllamaMeSigner interface {
	Sign(ctx context.Context, challenge []byte) (string, error)
}

type OllamaMeIdentity struct {
	IdentityState            QuotaIdentityState
	Source                   QuotaSource
	AccountName              string
	AccountEmail             string
	PlanName                 string
	NotifyUsageLimits        bool
	AccountMetadataPopulated bool
}

type ollamaKeySigner struct {
	keyPath string
}

type ollamaMeResponse struct {
	Name              string `json:"Name"`
	Email             string `json:"Email"`
	Plan              string `json:"Plan"`
	NotifyUsageLimits bool   `json:"NotifyUsageLimits"`
}

func NewOllamaKeySigner(keyPath string) OllamaMeSigner {
	if strings.TrimSpace(keyPath) == "" {
		home, _ := os.UserHomeDir()
		keyPath = filepath.Join(home, ".ollama", "id_ed25519")
	}
	return &ollamaKeySigner{keyPath: keyPath}
}

func (s *ollamaKeySigner) Sign(ctx context.Context, challenge []byte) (string, error) {
	privateKeyFile, err := os.ReadFile(s.keyPath)
	if err != nil {
		return "", err
	}
	privateKey, err := ssh.ParsePrivateKey(privateKeyFile)
	if err != nil {
		return "", err
	}

	publicKey := ssh.MarshalAuthorizedKey(privateKey.PublicKey())
	parts := bytes.Split(publicKey, []byte(" "))
	if len(parts) < 2 {
		return "", fmt.Errorf("malformed public key")
	}
	signedData, err := privateKey.Sign(rand.Reader, challenge)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", bytes.TrimSpace(parts[1]), base64.StdEncoding.EncodeToString(signedData.Blob)), nil
}

func ProbeOllamaMe(ctx context.Context, baseURL string, client *http.Client, signer OllamaMeSigner, now func() time.Time) (OllamaMeIdentity, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = http.DefaultClient
	}
	if signer == nil {
		return OllamaMeIdentity{IdentityState: QuotaIdentityStateError, Source: QuotaSourceOllamaAPIMe}, fmt.Errorf("ollama me signer is required")
	}
	if now == nil {
		now = time.Now
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}

	ts := now().UTC().Unix()
	challenge := fmt.Sprintf("POST,/api/me?ts=%d", ts)
	authHeader, err := signer.Sign(ctx, []byte(challenge))
	if err != nil {
		return OllamaMeIdentity{IdentityState: QuotaIdentityStateError, Source: QuotaSourceOllamaAPIMe}, fmt.Errorf("sign ollama me challenge: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/me?ts="+fmt.Sprint(ts), nil)
	if err != nil {
		return OllamaMeIdentity{IdentityState: QuotaIdentityStateError, Source: QuotaSourceOllamaAPIMe}, fmt.Errorf("build ollama me request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := client.Do(req)
	if err != nil {
		return OllamaMeIdentity{IdentityState: QuotaIdentityStateError, Source: QuotaSourceOllamaAPIMe}, fmt.Errorf("call ollama me: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		_, _ = io.Copy(io.Discard, resp.Body)
		return OllamaMeIdentity{IdentityState: QuotaIdentityStateUnauthenticated, Source: QuotaSourceOllamaAPIMe}, nil
	case http.StatusOK:
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return OllamaMeIdentity{IdentityState: QuotaIdentityStateError, Source: QuotaSourceOllamaAPIMe}, fmt.Errorf("ollama me returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload ollamaMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return OllamaMeIdentity{IdentityState: QuotaIdentityStateError, Source: QuotaSourceOllamaAPIMe}, fmt.Errorf("decode ollama me response: %w", err)
	}

	populated := strings.TrimSpace(payload.Name) != "" || strings.TrimSpace(payload.Email) != "" || strings.TrimSpace(payload.Plan) != ""
	state := QuotaIdentityStateAuthenticatedEmpty
	if populated {
		state = QuotaIdentityStateAuthenticated
	}

	return OllamaMeIdentity{
		IdentityState:            state,
		Source:                   QuotaSourceOllamaAPIMe,
		AccountName:              strings.TrimSpace(payload.Name),
		AccountEmail:             strings.TrimSpace(payload.Email),
		PlanName:                 strings.TrimSpace(payload.Plan),
		NotifyUsageLimits:        payload.NotifyUsageLimits,
		AccountMetadataPopulated: populated,
	}, nil
}
