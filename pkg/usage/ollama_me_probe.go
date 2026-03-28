package usage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const ollamaMePath = "/api/me"

type OllamaMeSigner interface {
	Sign(ctx context.Context, challenge []byte) (string, error)
}

type OllamaMeResult struct {
	IdentityState            QuotaIdentityState
	AccountName              string
	AccountEmail             string
	PlanName                 string
	NotifyUsageLimits        bool
	AccountMetadataPopulated bool
}

type OllamaMeFileSigner struct {
	KeyPath string
}

func NewOllamaMeFileSigner() (*OllamaMeFileSigner, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &OllamaMeFileSigner{KeyPath: filepath.Join(home, ".ollama", "id_ed25519")}, nil
}

func NewOllamaMeFileSignerAt(keyPath string) *OllamaMeFileSigner {
	return &OllamaMeFileSigner{KeyPath: keyPath}
}

func (s *OllamaMeFileSigner) Sign(ctx context.Context, challenge []byte) (string, error) {
	if s == nil {
		return "", errors.New("signer unavailable")
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
	}

	keyPath := strings.TrimSpace(s.KeyPath)
	if keyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		keyPath = filepath.Join(home, ".ollama", "id_ed25519")
	}

	privateKeyFile, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("read ollama private key: %w", err)
	}

	privateKey, err := ssh.ParsePrivateKey(privateKeyFile)
	if err != nil {
		return "", fmt.Errorf("parse ollama private key: %w", err)
	}

	publicKey := ssh.MarshalAuthorizedKey(privateKey.PublicKey())
	parts := bytes.Split(publicKey, []byte(" "))
	if len(parts) < 2 {
		return "", errors.New("malformed public key")
	}

	signedData, err := privateKey.Sign(rand.Reader, challenge)
	if err != nil {
		return "", fmt.Errorf("sign challenge: %w", err)
	}

	return fmt.Sprintf("%s:%s", bytes.TrimSpace(parts[1]), base64.StdEncoding.EncodeToString(signedData.Blob)), nil
}

func ProbeOllamaMe(ctx context.Context, baseURL string, client *http.Client, signer OllamaMeSigner, now func() time.Time) (OllamaMeResult, error) {
	var result OllamaMeResult
	if ctx == nil {
		ctx = context.Background()
	}

	if signer == nil {
		result.IdentityState = QuotaIdentityStateError
		return result, errors.New("ollama me signer is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	if now == nil {
		now = time.Now
	}

	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		result.IdentityState = QuotaIdentityStateError
		return result, fmt.Errorf("parse base url: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		result.IdentityState = QuotaIdentityStateError
		return result, fmt.Errorf("invalid base url %q", baseURL)
	}

	ts := strconvFormatInt(now().UTC().Unix())
	challenge := []byte(fmt.Sprintf("%s,%s?ts=%s", http.MethodPost, ollamaMePath, ts))
	token, err := signer.Sign(ctx, challenge)
	if err != nil {
		result.IdentityState = QuotaIdentityStateError
		return result, fmt.Errorf("sign ollama me challenge: %w", err)
	}

	requestURL := *base
	requestURL.Path = strings.TrimRight(base.Path, "/") + ollamaMePath
	requestURL.RawQuery = url.Values{"ts": []string{ts}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), nil)
	if err != nil {
		result.IdentityState = QuotaIdentityStateError
		return result, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "smolbot-ollama-me-probe")
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.IdentityState = QuotaIdentityStateError
		return result, fmt.Errorf("probe ollama me: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.IdentityState = QuotaIdentityStateError
		return result, fmt.Errorf("read ollama me response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		result.IdentityState = QuotaIdentityStateUnauthenticated
		return result, nil
	case http.StatusOK:
		// handled below
	default:
		result.IdentityState = QuotaIdentityStateError
		if len(body) > 0 {
			return result, fmt.Errorf("ollama me probe failed: %s", strings.TrimSpace(string(body)))
		}
		return result, fmt.Errorf("ollama me probe failed: %s", resp.Status)
	}

	if len(bytes.TrimSpace(body)) == 0 {
		result.IdentityState = QuotaIdentityStateAuthenticatedEmpty
		return result, nil
	}

	var raw ollamaMeResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		result.IdentityState = QuotaIdentityStateError
		return result, fmt.Errorf("decode ollama me response: %w", err)
	}

	result.AccountName = strings.TrimSpace(raw.Name)
	result.AccountEmail = strings.TrimSpace(raw.Email)
	result.PlanName = strings.TrimSpace(raw.Plan)
	result.NotifyUsageLimits = raw.NotifyUsageLimits
	result.AccountMetadataPopulated = raw.hasAccountMetadata()
	if result.AccountMetadataPopulated {
		result.IdentityState = QuotaIdentityStateAuthenticated
	} else {
		result.IdentityState = QuotaIdentityStateAuthenticatedEmpty
	}
	return result, nil
}

type ollamaMeResponse struct {
	ID                      string             `json:"ID,omitempty"`
	Email                   string             `json:"Email,omitempty"`
	Name                    string             `json:"Name,omitempty"`
	Bio                     string             `json:"Bio,omitempty"`
	AvatarURL               string             `json:"AvatarURL,omitempty"`
	FirstName               string             `json:"FirstName,omitempty"`
	LastName                string             `json:"LastName,omitempty"`
	Links                   []string           `json:"Links,omitempty"`
	CustomerID              nullableIdentifier `json:"CustomerID,omitempty"`
	SubscriptionID          nullableIdentifier `json:"SubscriptionID,omitempty"`
	WorkOSUserID            nullableIdentifier `json:"WorkOSUserID,omitempty"`
	Plan                    string             `json:"Plan,omitempty"`
	NotifyUsageLimits       bool               `json:"NotifyUsageLimits,omitempty"`
	SubscriptionPeriodStart nullableDateTime   `json:"SubscriptionPeriodStart,omitempty"`
	SubscriptionPeriodEnd   nullableDateTime   `json:"SubscriptionPeriodEnd,omitempty"`
	SuspendedAt             nullableDateTime   `json:"SuspendedAt,omitempty"`
}

func (r ollamaMeResponse) hasAccountMetadata() bool {
	if strings.TrimSpace(r.ID) != "" ||
		strings.TrimSpace(r.Email) != "" ||
		strings.TrimSpace(r.Name) != "" ||
		strings.TrimSpace(r.Bio) != "" ||
		strings.TrimSpace(r.AvatarURL) != "" ||
		strings.TrimSpace(r.FirstName) != "" ||
		strings.TrimSpace(r.LastName) != "" ||
		strings.TrimSpace(r.Plan) != "" ||
		r.NotifyUsageLimits {
		return true
	}
	if r.CustomerID.Valid || strings.TrimSpace(r.CustomerID.String) != "" {
		return true
	}
	if r.SubscriptionID.Valid || strings.TrimSpace(r.SubscriptionID.String) != "" {
		return true
	}
	if r.WorkOSUserID.Valid || strings.TrimSpace(r.WorkOSUserID.String) != "" {
		return true
	}
	if r.SubscriptionPeriodStart.Valid || r.SubscriptionPeriodEnd.Valid || r.SuspendedAt.Valid {
		return true
	}
	return false
}

type nullableIdentifier struct {
	String string `json:"String,omitempty"`
	Valid  bool   `json:"Valid,omitempty"`
}

type nullableDateTime struct {
	Time  time.Time `json:"Time,omitempty"`
	Valid bool      `json:"Valid,omitempty"`
}

func strconvFormatInt(v int64) string {
	return fmt.Sprintf("%d", v)
}
