package usage

import "time"

type CompletionRecord struct {
	SessionKey       string
	ProviderID       string
	ModelName        string
	RequestType      string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	DurationMS       int
	Status           string
	UsageSource      string
	RecordedAt       time.Time
}

type UsageRecord struct {
	ID               int64
	SessionKey       string
	ProviderID       string
	ModelName        string
	RequestType      string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	DurationMS       int
	Status           string
	UsageSource      string
	CreatedAt        time.Time
}

type Summary struct {
	TotalRequests         int
	TotalPromptTokens     int
	TotalCompletionTokens int
	TotalTokens           int
}

type ProviderSummary struct {
	ProviderID      string
	ModelName       string
	SessionKey      string
	SessionTokens   int
	TodayTokens     int
	WeeklyTokens    int
	SessionRequests int
	TodayRequests   int
	WeeklyRequests  int
	BudgetStatus    string
	WarningLevel    string
	Quota           *QuotaSummary
}

type QuotaState string

const (
	QuotaStateLive        QuotaState = "live"
	QuotaStateStale       QuotaState = "stale"
	QuotaStateExpired     QuotaState = "expired"
	QuotaStateUnavailable QuotaState = "unavailable"
)

type QuotaSource string

const (
	QuotaSourceUnknown            QuotaSource = ""
	QuotaSourceOllamaAPIMe        QuotaSource = "ollama_api_me"
	QuotaSourceOllamaSettingsHTML QuotaSource = "ollama_settings_html"
)

type QuotaIdentityState string

const (
	QuotaIdentityStateUnknown            QuotaIdentityState = ""
	QuotaIdentityStateAuthenticated      QuotaIdentityState = "authenticated"
	QuotaIdentityStateAuthenticatedEmpty QuotaIdentityState = "authenticated_empty"
	QuotaIdentityStateUnauthenticated    QuotaIdentityState = "unauthenticated"
	QuotaIdentityStateError              QuotaIdentityState = "error"
)

type QuotaSummary struct {
	ProviderID           string
	AccountName          string
	AccountEmail         string
	PlanName             string
	SessionUsedPercent   float64
	SessionResetsAt      *time.Time
	WeeklyUsedPercent    float64
	WeeklyResetsAt       *time.Time
	NotifyUsageLimits    bool
	State                QuotaState
	Source               QuotaSource
	FetchedAt            time.Time
	ExpiresAt            time.Time
	IdentityState        QuotaIdentityState
	IdentitySource       QuotaSource
	IdentityAccountName  string
	IdentityAccountEmail string
}

type Budget struct {
	ID              string
	Name            string
	BudgetType      string
	LimitAmount     float64
	LimitUnit       string
	ScopeType       string
	ScopeTarget     string
	AlertThresholds []int
	IsActive        bool
	WindowStart     *time.Time
	WindowEnd       *time.Time
	ResetsAt        *time.Time
}

type BudgetAlert struct {
	ID               int64
	BudgetID         string
	AlertType        string
	ThresholdPercent int
	TokensAtAlert    int
	SentAt           time.Time
	Channel          string
}

type HistoricalUsageSample struct {
	ID            int64
	ProviderID    string
	ModelName     string
	WindowType    string
	Source        string
	SampledAt     time.Time
	UsedPercent   float64
	ResetsAt      time.Time
	WindowMinutes int
	TotalTokens   int
}
