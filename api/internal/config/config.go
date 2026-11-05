package config

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Config holds the entire system configuration
type Config struct {
	App              AppConfig              `mapstructure:"app"`
	Server           ServerConfig           `mapstructure:"server"`
	Database         DatabaseConfig         `mapstructure:"database"`
	Auth             AuthConfig             `mapstructure:"auth"`
	CORS             CORSConfig             `mapstructure:"cors"`
	Logger           LoggerConfig           `mapstructure:"logger"`
	Tracer           TracerConfig           `mapstructure:"tracer"`
	AI               AIConfig               `mapstructure:"ai"`
	LLM              LLMConfig              `mapstructure:"llm"`
	TTS              TTSConfig              `mapstructure:"tts"`
	Email            EmailConfig            `mapstructure:"email"`
	Storage          StorageConfig          `mapstructure:"storage"`
	PayOS            PayOSConfig            `mapstructure:"payos"`
	QuestionBank     QuestionBankConfig     `mapstructure:"question_bank"`
	Interview        InterviewConfig        `mapstructure:"interview"`
	Evaluation       EvaluationConfig       `mapstructure:"evaluation"`
	AvatarConversion AvatarConversionConfig `mapstructure:"avatar_conversion"`
	Jobs             JobsConfig             `mapstructure:"jobs"`
	RateLimit        RateLimitConfig        `mapstructure:"rate_limit"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"` // development, staging, production
	Version     string `mapstructure:"version"`
	Debug       bool   `mapstructure:"debug"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	// TrustedProxies lists the reverse proxies (IPs or CIDRs) whose X-Forwarded-For is believed.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

type DatabaseConfig struct {
	URL         string `mapstructure:"url"`
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	Name        string `mapstructure:"name"`
	SSLMode     string `mapstructure:"ssl_mode"`
	SSLRootCert string `mapstructure:"ssl_root_cert"`
	// RequireTLS makes production reject any sslmode but verify-full with a root certificate.
	RequireTLS      bool          `mapstructure:"require_tls"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	DebugLevel      string        `mapstructure:"debug_level"`
}

// DSN returns the PostgreSQL connection string.
// If URL is set, it is returned directly.
// Otherwise, a postgres:// connection URL is constructed with all
// credentials and parameters properly escaped.
func (d *DatabaseConfig) DSN() string {
	if d.URL != "" {
		return d.URL
	}

	u := &url.URL{
		Scheme: "postgres",
	}

	host := d.Host
	if host == "" {
		host = "localhost"
	}
	port := d.Port
	if port <= 0 {
		port = 5432
	}
	u.Host = net.JoinHostPort(host, strconv.Itoa(port))

	if d.Name != "" {
		u.Path = "/" + strings.TrimPrefix(d.Name, "/")
	}

	if d.User != "" || d.Password != "" {
		u.User = url.UserPassword(d.User, d.Password)
	}

	q := make(url.Values)
	if d.SSLMode != "" {
		q.Set("sslmode", d.SSLMode)
	}
	if d.SSLRootCert != "" {
		q.Set("sslrootcert", d.SSLRootCert)
	}
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}

	return u.String()
}

type AuthConfig struct {
	PublicURL     string        `mapstructure:"public_url"`
	AdminSecret   string        `mapstructure:"admin_secret"`
	URL           string        `mapstructure:"url"`
	Timeout       time.Duration `mapstructure:"timeout"`
	CookiePrefix  string        `mapstructure:"cookie_prefix"`
	CookieDomain  string        `mapstructure:"cookie_domain"`
	SecureCookies bool          `mapstructure:"secure_cookies"`
	// AdminRequireMFA rejects Admin sessions without two-factor authentication; AdminMFAGrace is an RFC3339
	// instant (empty = none) before which Admins without it are still let in.
	AdminRequireMFA bool   `mapstructure:"admin_require_mfa"`
	AdminMFAGrace   string `mapstructure:"admin_mfa_grace"`
}

type CORSConfig struct {
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

type LoggerConfig struct {
	Level      string `mapstructure:"level"`  // debug, info, warn, error
	Format     string `mapstructure:"format"` // json or console
	Output     string `mapstructure:"output"` // stdout or file path
	TimeFormat string `mapstructure:"time_format"`
}

type TracerConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	ServiceName string  `mapstructure:"service_name"`
	Endpoint    string  `mapstructure:"endpoint"`
	Insecure    bool    `mapstructure:"insecure"`
	SampleRate  float64 `mapstructure:"sample_rate"`
}

type LLMConfig struct {
	Provider   string        `mapstructure:"provider"`
	APIKey     string        `mapstructure:"api_key"`
	Model      string        `mapstructure:"model"`
	Timeout    time.Duration `mapstructure:"timeout"`
	MaxRetries int           `mapstructure:"max_retries"`
	// MaxOutputTokens caps one completion, which bounds the cost of a single call. Zero leaves the
	// provider default.
	MaxOutputTokens int `mapstructure:"max_output_tokens"`
	// UserDailyCallBudget is the most LLM-backed operations one user may start per UTC day (KAN-278); 0 is
	// unlimited. See internal/pkg/llmbudget.
	UserDailyCallBudget int `mapstructure:"user_daily_call_budget"`
}

type PayOSConfig struct {
	ClientID    string        `mapstructure:"client_id"`
	APIKey      string        `mapstructure:"api_key"`
	ChecksumKey string        `mapstructure:"checksum_key"`
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxRetries  int           `mapstructure:"max_retries"`
}

// TTSConfig holds the Azure AI Speech settings, shared by interview text-to-speech and the Admin voice
// sync. Key and Region may both be empty: the API still starts, interviews fall back to text and the voice
// sync reports the provider as not configured. Timeout bounds one provider call, including its retries.
// A malformed value does not stop the API: the synthesizer is wired as unavailable and the reason is
// logged. SyncLocales limits the voice sync to those locales (the default en-US because the interview
// speaks English only; an entry also matches the regions under it, so "en" would match en-GB too;
// empty means all).
type TTSConfig struct {
	Key         string        `mapstructure:"key"`
	Region      string        `mapstructure:"region"`
	Timeout     time.Duration `mapstructure:"timeout"`
	SyncLocales []string      `mapstructure:"sync_locales"`
}

// EmailConfig configures the Go API's transactional email (KAN-213). Provider "log" sends nothing and
// only logs, which is the default for local development and tests; "resend" sends through Resend.
type EmailConfig struct {
	ModerationApprovedTemplateID string `mapstructure:"approved_template_id"`
	ModerationRejectedTemplateID string `mapstructure:"rejected_template_id"`

	// Application notice templates (KAN-219). A blank ID disables that notice.
	ApplicationSubmittedTemplateID        string `mapstructure:"application_submitted_template_id"`
	ApplicationNewApplicationTemplateID   string `mapstructure:"application_new_application_template_id"`
	ApplicationCVRejectedTemplateID       string `mapstructure:"application_cv_rejected_template_id"`
	ApplicationCVApprovedTemplateID       string `mapstructure:"application_cv_approved_template_id"`
	ApplicationDecisionApprovedTemplateID string `mapstructure:"application_decision_approved_template_id"`
	ApplicationDecisionRejectedTemplateID string `mapstructure:"application_decision_rejected_template_id"`
	ApplicationAutoRejectedTemplateID     string `mapstructure:"application_auto_rejected_template_id"`
	ApplicationIntakeClosedTemplateID     string `mapstructure:"application_intake_closed_template_id"`

	Provider    string        `mapstructure:"provider"`
	From        string        `mapstructure:"from"`
	APIKey      string        `mapstructure:"api_key"`
	Timeout     time.Duration `mapstructure:"timeout"`
	Workers     int           `mapstructure:"workers"`
	QueueSize   int           `mapstructure:"queue_size"`
	MaxAttempts int           `mapstructure:"max_attempts"`
	Backoff     time.Duration `mapstructure:"backoff"`
}

// QuestionBankConfig holds the core-question bank settings. Size is the number of questions
// generated per bank. The team chose a large bank (50) so any interview duration is covered,
// because one interview only asks fewer than ten core questions. 0 disables generation.
type QuestionBankConfig struct {
	Size int `mapstructure:"size"`
}

// MaxQuestionBankSize bounds QUESTION_BANK_SIZE to a sane technical range.
const MaxQuestionBankSize = 100

// InterviewConfig holds the interview session settings. QuestionLimit is the most core questions one
// session asks (0 uses the default of 8). The practice price is a fixed team decision, not configuration
// (interview/domain.PracticeInterviewCoins).
//
// The live interview engine (docs/interview-ws-protocol.md §10) adds: STTProvider, the speech-to-text
// adapter ("fake" until KAN-55 adds a real one; empty means fake); STTTimeout, the bound of one
// transcription (0 uses 30s); DefaultProviderVoiceID, the provider voice of a session without its own
// voice (default en-US-JennyNeural, team decision 2026-10-30; set it empty for text-only turns); and TTSCacheEntries, the size of the in-memory synthesis cache (0
// disables it).
//
// Bounded follow-ups (KAN-56): FollowUpLimit is the most follow-ups the live interviewer asks after one
// core question (default 1, coordinator proposal 2026-11-03; 0 turns follow-ups off and the interview is
// core questions only); FollowUpTimeout bounds one follow-up decision (0 uses 5s), after which the
// interview moves on to the next core question.
type InterviewConfig struct {
	QuestionLimit          int           `mapstructure:"question_limit"`
	STTProvider            string        `mapstructure:"stt_provider"`
	STTTimeout             time.Duration `mapstructure:"stt_timeout"`
	DefaultProviderVoiceID string        `mapstructure:"default_provider_voice_id"`
	TTSCacheEntries        int           `mapstructure:"tts_cache_entries"`
	FollowUpLimit          int           `mapstructure:"follow_up_limit"`
	FollowUpTimeout        time.Duration `mapstructure:"follow_up_timeout"`
	// LiveRecheckInterval is how often an open live socket's login is re-verified (0 uses 60s);
	// LiveMessageRate and LiveMessageBurst size the per-socket incoming-message token bucket (0 uses 20/s
	// and 60). A socket whose login is gone is closed; a socket that keeps flooding is closed too.
	LiveRecheckInterval time.Duration `mapstructure:"live_recheck_interval"`
	LiveMessageRate     float64       `mapstructure:"live_message_rate"`
	LiveMessageBurst    int           `mapstructure:"live_message_burst"`
}

// MaxInterviewQuestionLimit bounds INTERVIEW_QUESTION_LIMIT to a sane technical range.
const MaxInterviewQuestionLimit = 20

// STTProviderFake is the deterministic development transcriber, the only one until KAN-55.
const STTProviderFake = "fake"

// Bounds of the live interview settings.
const (
	MinInterviewSTTTimeout      = time.Second
	MaxInterviewSTTTimeout      = 2 * time.Minute
	MaxInterviewTTSCacheEntries = 4096
	maxProviderVoiceIDLength    = 128
	MaxInterviewFollowUpLimit   = 3
	MinInterviewFollowUpTimeout = time.Second
	MaxInterviewFollowUpTimeout = 15 * time.Second
	MinLiveRecheckInterval      = 5 * time.Second
	MaxLiveRecheckInterval      = 10 * time.Minute
	MaxLiveMessageRate          = 1000
	MaxLiveMessageBurst         = 10000
)

// Validate checks the live interview settings.
func (c InterviewConfig) Validate() error {
	if c.QuestionLimit < 0 || c.QuestionLimit > MaxInterviewQuestionLimit {
		return fmt.Errorf("interview.question_limit must be between 0 and %d", MaxInterviewQuestionLimit)
	}
	if c.STTProvider != "" && c.STTProvider != STTProviderFake {
		return fmt.Errorf("interview.stt_provider must be %q", STTProviderFake)
	}
	if c.STTTimeout != 0 && (c.STTTimeout < MinInterviewSTTTimeout || c.STTTimeout > MaxInterviewSTTTimeout) {
		return fmt.Errorf("interview.stt_timeout must be between %s and %s", MinInterviewSTTTimeout, MaxInterviewSTTTimeout)
	}
	if v := c.DefaultProviderVoiceID; len(v) > maxProviderVoiceIDLength || strings.ContainsAny(v, " \t\r\n") {
		return errors.New("interview.default_provider_voice_id must be one provider voice name")
	}
	if c.TTSCacheEntries < 0 || c.TTSCacheEntries > MaxInterviewTTSCacheEntries {
		return fmt.Errorf("interview.tts_cache_entries must be between 0 and %d", MaxInterviewTTSCacheEntries)
	}
	if c.FollowUpLimit < 0 || c.FollowUpLimit > MaxInterviewFollowUpLimit {
		return fmt.Errorf("interview.follow_up_limit must be between 0 and %d", MaxInterviewFollowUpLimit)
	}
	if c.LiveRecheckInterval != 0 && (c.LiveRecheckInterval < MinLiveRecheckInterval || c.LiveRecheckInterval > MaxLiveRecheckInterval) {
		return fmt.Errorf("interview.live_recheck_interval must be between %s and %s", MinLiveRecheckInterval, MaxLiveRecheckInterval)
	}
	if math.IsNaN(c.LiveMessageRate) || c.LiveMessageRate < 0 || c.LiveMessageRate > MaxLiveMessageRate {
		return fmt.Errorf("interview.live_message_rate must be between 0 and %d", MaxLiveMessageRate)
	}
	if c.LiveMessageBurst < 0 || c.LiveMessageBurst > MaxLiveMessageBurst {
		return fmt.Errorf("interview.live_message_burst must be between 0 and %d", MaxLiveMessageBurst)
	}
	if c.FollowUpTimeout != 0 && (c.FollowUpTimeout < MinInterviewFollowUpTimeout || c.FollowUpTimeout > MaxInterviewFollowUpTimeout) {
		return fmt.Errorf("interview.follow_up_timeout must be between %s and %s", MinInterviewFollowUpTimeout, MaxInterviewFollowUpTimeout)
	}
	return nil
}

// EvaluationConfig holds the posting evaluation-weight settings. WeightTotal is the sum a posting's
// weights must reach (100 = percentages); 0 turns the sum check off. It lives in configuration
// because the scoring formula is still an open product decision.
type EvaluationConfig struct {
	WeightTotal float64 `mapstructure:"weight_total"`
}

// MaxEvaluationWeightTotal bounds EVALUATION_WEIGHT_TOTAL to a sane technical range.
const MaxEvaluationWeightTotal = 1000

// Load loads configuration from file and overrides with environment variables
func Load(configPath string) (*Config, error) {
	_ = gotenv.Load("../.env")
	_ = gotenv.Load(".env")

	v := viper.New()

	setDefaults(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/rolecue/")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.AllowEmptyEnv(true)

	// Explicitly bind environment variables
	for key, env := range map[string]string{
		"storage.endpoint":                        "S3_ENDPOINT",
		"storage.region":                          "S3_REGION",
		"storage.bucket":                          "S3_BUCKET",
		"storage.access_key_id":                   "S3_ACCESS_KEY_ID",
		"storage.secret_access_key":               "S3_SECRET_ACCESS_KEY",
		"storage.session_token":                   "S3_SESSION_TOKEN",
		"database.url":                            "DATABASE_URL",
		"database.name":                           "DB_DATABASE",
		"database.user":                           "DB_USERNAME",
		"database.password":                       "DB_PASSWORD",
		"database.require_tls":                    "DATABASE_REQUIRE_TLS",
		"database.ssl_root_cert":                  "DATABASE_SSL_ROOT_CERT",
		"server.trusted_proxies":                  "SERVER_TRUSTED_PROXIES",
		"database.port":                           "DB_PORT",
		"auth.public_url":                         "AUTH_BASE_URL",
		"auth.admin_secret":                       "ADMIN_AUTH_INTERNAL_SECRET",
		"auth.timeout":                            "AUTH_SESSION_TIMEOUT",
		"auth.cookie_prefix":                      "BETTER_AUTH_COOKIE_PREFIX",
		"auth.cookie_domain":                      "BETTER_AUTH_COOKIE_DOMAIN",
		"auth.secure_cookies":                     "AUTH_SECURE_COOKIES",
		"auth.admin_require_mfa":                  "ADMIN_REQUIRE_MFA",
		"auth.admin_mfa_grace":                    "ADMIN_MFA_GRACE_UNTIL",
		"llm.provider":                            "LLM_PROVIDER",
		"llm.api_key":                             "LLM_API_KEY",
		"llm.model":                               "LLM_MODEL",
		"llm.timeout":                             "LLM_TIMEOUT",
		"llm.max_retries":                         "LLM_MAX_RETRIES",
		"llm.max_output_tokens":                   "LLM_MAX_OUTPUT_TOKENS",
		"tts.key":                                 "AZURE_SPEECH_KEY",
		"tts.region":                              "AZURE_SPEECH_REGION",
		"tts.timeout":                             "TTS_TIMEOUT",
		"tts.sync_locales":                        "TTS_SYNC_LOCALES",
		"email.provider":                          "EMAIL_PROVIDER",
		"email.from":                              "EMAIL_FROM",
		"email.api_key":                           "RESEND_API_KEY",
		"email.timeout":                           "EMAIL_TIMEOUT",
		"email.workers":                           "EMAIL_WORKERS",
		"email.queue_size":                        "EMAIL_QUEUE_SIZE",
		"email.max_attempts":                      "EMAIL_MAX_ATTEMPTS",
		"email.backoff":                           "EMAIL_BACKOFF",
		"email.approved_template_id":              "EMAIL_MODERATION_APPROVED_TEMPLATE_ID",
		"email.rejected_template_id":              "EMAIL_MODERATION_REJECTED_TEMPLATE_ID",
		"email.application_submitted_template_id": "EMAIL_APPLICATION_SUBMITTED_TEMPLATE_ID",
		"email.application_new_application_template_id":   "EMAIL_APPLICATION_NEW_APPLICATION_TEMPLATE_ID",
		"email.application_cv_rejected_template_id":       "EMAIL_APPLICATION_CV_REJECTED_TEMPLATE_ID",
		"email.application_cv_approved_template_id":       "EMAIL_APPLICATION_CV_APPROVED_TEMPLATE_ID",
		"email.application_decision_approved_template_id": "EMAIL_APPLICATION_DECISION_APPROVED_TEMPLATE_ID",
		"email.application_decision_rejected_template_id": "EMAIL_APPLICATION_DECISION_REJECTED_TEMPLATE_ID",
		"email.application_auto_rejected_template_id":     "EMAIL_APPLICATION_AUTO_REJECTED_TEMPLATE_ID",
		"email.application_intake_closed_template_id":     "EMAIL_APPLICATION_INTAKE_CLOSED_TEMPLATE_ID",
		"payos.client_id":                     "PAYOS_CLIENT_ID",
		"payos.api_key":                       "PAYOS_API_KEY",
		"payos.checksum_key":                  "PAYOS_CHECKSUM_KEY",
		"payos.timeout":                       "PAYOS_TIMEOUT",
		"payos.max_retries":                   "PAYOS_MAX_RETRIES",
		"question_bank.size":                  "QUESTION_BANK_SIZE",
		"evaluation.weight_total":             "EVALUATION_WEIGHT_TOTAL",
		"interview.question_limit":            "INTERVIEW_QUESTION_LIMIT",
		"interview.stt_provider":              "INTERVIEW_STT_PROVIDER",
		"interview.stt_timeout":               "INTERVIEW_STT_TIMEOUT",
		"interview.default_provider_voice_id": "INTERVIEW_DEFAULT_PROVIDER_VOICE_ID",
		"interview.tts_cache_entries":         "INTERVIEW_TTS_CACHE_ENTRIES",
		"interview.follow_up_limit":           "INTERVIEW_FOLLOW_UP_LIMIT",
		"interview.follow_up_timeout":         "INTERVIEW_FOLLOW_UP_TIMEOUT",
		"interview.live_recheck_interval":     "INTERVIEW_LIVE_RECHECK_INTERVAL",
		"interview.live_message_rate":         "INTERVIEW_LIVE_MESSAGE_RATE",
		"interview.live_message_burst":        "INTERVIEW_LIVE_MESSAGE_BURST",
		"avatar_conversion.enabled":           "AVATAR_CONVERSION_ENABLED",
		"avatar_conversion.blender_binary":    "AVATAR_BLENDER_BINARY",
		"avatar_conversion.timeout":           "AVATAR_CONVERSION_TIMEOUT",
		"avatar_conversion.max_input_bytes":   "AVATAR_MAX_INPUT_BYTES",
		"avatar_conversion.max_output_bytes":  "AVATAR_MAX_OUTPUT_BYTES",
		"avatar_conversion.commercial_usage":  "AVATAR_VRM_COMMERCIAL_USAGE",
		"avatar_conversion.license_url":       "AVATAR_VRM_LICENSE_URL",
		"avatar_conversion.user_daily_quota":  "AVATAR_CONVERSION_USER_DAILY_QUOTA",
		"avatar_conversion.user_max_inflight": "AVATAR_CONVERSION_USER_MAX_INFLIGHT",
	} {
		if err := v.BindEnv(key, env); err != nil {
			return nil, fmt.Errorf("bind %s: %w", key, err)
		}
	}

	if err := bindRateLimitEnv(v); err != nil {
		return nil, err
	}
	if err := bindJobEnv(v); err != nil {
		return nil, err
	}

	// AUTH_UPSTREAM_URL takes precedence; legacy BETTER_AUTH_URL keeps one-origin configs working.
	if err := v.BindEnv("auth.url", "AUTH_UPSTREAM_URL", "BETTER_AUTH_URL"); err != nil {
		return nil, fmt.Errorf("bind auth.url: %w", err)
	}

	if err := bindAI(v); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.AI.GroqAPIKey = os.Getenv("GROQ_API_KEY")

	if cfg.Auth.PublicURL == "" {
		if os.Getenv("AUTH_UPSTREAM_URL") != "" {
			return nil, errors.New("AUTH_BASE_URL is required with AUTH_UPSTREAM_URL")
		}
		cfg.Auth.PublicURL = cfg.Auth.URL
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("storage.endpoint", "")
	v.SetDefault("storage.region", "")
	v.SetDefault("storage.bucket", "")
	// App defaults
	v.SetDefault("app.name", "rolecue")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("app.debug", true)

	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "5s")
	v.SetDefault("server.trusted_proxies", DefaultTrustedProxies)

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5434)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.name", "rolecue")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.ssl_root_cert", "")
	v.SetDefault("database.require_tls", false)
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "5m")

	v.SetDefault("auth.url", "http://localhost:3000")
	v.SetDefault("auth.timeout", "5s")
	v.SetDefault("auth.cookie_prefix", "rolecue")
	v.SetDefault("auth.cookie_domain", "")
	v.SetDefault("auth.secure_cookies", false)
	v.SetDefault("auth.admin_require_mfa", false)
	v.SetDefault("auth.admin_mfa_grace", "")
	// CORS defaults
	v.SetDefault("cors.allow_origins", []string{"http://localhost:3000"})
	v.SetDefault("cors.allow_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allow_headers", []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "Idempotency-Key"})
	v.SetDefault("cors.expose_headers", []string{"X-Request-ID", "X-Total-Count", "X-Export-Truncated", "Content-Disposition"})
	v.SetDefault("cors.allow_credentials", true)
	v.SetDefault("cors.max_age", 86400)

	// Logger defaults
	v.SetDefault("logger.level", "debug")
	v.SetDefault("logger.format", "console")
	v.SetDefault("logger.output", "stdout")
	v.SetDefault("logger.time_format", "2006-01-02T15:04:05.000Z07:00")

	// Tracer defaults
	v.SetDefault("tracer.enabled", false)
	v.SetDefault("tracer.service_name", "rolecue-api")
	v.SetDefault("tracer.endpoint", "localhost:4317")
	v.SetDefault("tracer.insecure", true)
	v.SetDefault("tracer.sample_rate", 1.0)

	// LLM defaults
	v.SetDefault("llm.provider", "gemini")
	v.SetDefault("llm.model", "gemini-3.1-flash-lite")
	v.SetDefault("llm.timeout", "30s")
	v.SetDefault("llm.max_retries", 1)
	v.SetDefault("llm.max_output_tokens", 4096)

	// TTS defaults: one provider call gets 10s; the interview speaks English only, so the voice sync keeps English locales.
	v.SetDefault("tts.timeout", "10s")
	v.SetDefault("tts.sync_locales", []string{"en-US"})

	// Email defaults: log only (nothing is sent), two workers, three attempts with a doubling backoff.
	v.SetDefault("email.provider", "log")
	v.SetDefault("email.timeout", "10s")
	v.SetDefault("email.workers", 2)
	v.SetDefault("email.queue_size", 256)
	v.SetDefault("email.max_attempts", 3)
	v.SetDefault("email.backoff", "1s")

	// PayOS defaults
	v.SetDefault("payos.timeout", "30s")
	v.SetDefault("payos.max_retries", 2)

	// Question bank defaults: 50 questions per bank, as agreed by the team.
	v.SetDefault("question_bank.size", 50)
	v.SetDefault("evaluation.weight_total", 100)
	v.SetDefault("interview.question_limit", 8)
	v.SetDefault("interview.stt_provider", STTProviderFake)
	v.SetDefault("interview.stt_timeout", "30s")
	v.SetDefault("interview.default_provider_voice_id", "en-US-JennyNeural")
	v.SetDefault("interview.tts_cache_entries", 128)
	v.SetDefault("interview.follow_up_limit", 1)
	v.SetDefault("interview.follow_up_timeout", "5s")
	v.SetDefault("interview.live_recheck_interval", "60s")
	v.SetDefault("interview.live_message_rate", 20)
	v.SetDefault("interview.live_message_burst", 60)
	setJobDefaults(v)
	setRateLimitDefaults(v)
	v.SetDefault("avatar_conversion.enabled", false)
	v.SetDefault("avatar_conversion.blender_binary", "blender")
	v.SetDefault("avatar_conversion.timeout", "120s")
	v.SetDefault("avatar_conversion.max_input_bytes", 50<<20)
	v.SetDefault("avatar_conversion.max_output_bytes", 100<<20)
	v.SetDefault("avatar_conversion.user_daily_quota", 20)
	v.SetDefault("avatar_conversion.user_max_inflight", 1)
}

// Validate checks configuration integrity and security constraints.
func (c *Config) Validate() error {
	if err := c.validateAI(); err != nil {
		return err
	}
	if err := c.AvatarConversion.Validate(); err != nil {
		return err
	}
	if err := validateTrustedProxies(c.Server.TrustedProxies); err != nil {
		return err
	}
	if c.IsProduction() {
		if err := c.Database.validateProduction(); err != nil {
			return err
		}
	}
	if c.CORS.AllowCredentials {
		if len(c.CORS.AllowOrigins) == 0 {
			return errors.New("credentialed CORS requires explicit allowed origins")
		}
		for _, origin := range c.CORS.AllowOrigins {
			if origin == "*" || strings.TrimSpace(origin) == "" {
				return errors.New("credentialed CORS requires explicit allowed origins, without wildcard")
			}
		}
	}
	u, err := url.Parse(c.Auth.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("auth.url must be a fixed HTTP(S) origin without path, query or credentials")
	}
	u, err = url.Parse(c.Auth.PublicURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("auth.public_url must be a fixed HTTP(S) origin")
	}

	if c.Auth.Timeout <= 0 || c.Auth.Timeout > 30*time.Second {
		return errors.New("auth.timeout must be positive and at most 30s")
	}
	if c.Auth.CookiePrefix == "" || strings.ContainsAny(c.Auth.CookiePrefix, " ;=\r\n") {
		return errors.New("auth.cookie_prefix is invalid")
	}
	if g := c.Auth.AdminMFAGrace; g != "" {
		if _, err := time.Parse(time.RFC3339, g); err != nil {
			return errors.New("auth.admin_mfa_grace (ADMIN_MFA_GRACE_UNTIL) must be an RFC3339 timestamp")
		}
	}
	if c.Auth.SecureCookies != (u.Scheme == "https") {
		return errors.New("auth.secure_cookies must match the public auth URL scheme")
	}
	if domain := c.Auth.CookieDomain; domain != "" && (!c.Auth.SecureCookies || (u.Hostname() != domain && !strings.HasSuffix(u.Hostname(), "."+domain))) {
		return errors.New("shared auth cookies require HTTPS under the cookie domain")
	}
	if c.IsProduction() && (!c.Auth.SecureCookies || c.Auth.CookieDomain == "") {
		return errors.New("production requires HTTPS and a shared cookie domain")
	}

	if c.QuestionBank.Size < 0 || c.QuestionBank.Size > MaxQuestionBankSize {
		return fmt.Errorf("question_bank.size must be between 0 and %d", MaxQuestionBankSize)
	}
	if c.TTS.Timeout < 0 {
		return errors.New("tts.timeout must not be negative")
	}
	// Load always sets a provider (default "log"); a hand-built Config without one has no email section to check.
	if c.Email.Provider != "" {
		switch c.Email.Provider {
		case "log":
		case "resend":
			if c.Email.APIKey == "" || c.Email.From == "" {
				return errors.New("email.provider resend requires RESEND_API_KEY and EMAIL_FROM")
			}
		default:
			return errors.New("email.provider must be log or resend")
		}
		if c.Email.Timeout < 0 || c.Email.Backoff < 0 || c.Email.Workers < 1 || c.Email.Workers > 32 ||
			c.Email.QueueSize < 1 || c.Email.QueueSize > 10000 || c.Email.MaxAttempts < 1 || c.Email.MaxAttempts > 10 {
			return errors.New("email workers (1-32), queue_size (1-10000) and max_attempts (1-10) must be in range, timeout and backoff not negative")
		}
	}
	if err := c.Interview.Validate(); err != nil {
		return err
	}
	if err := c.RateLimit.Validate(); err != nil {
		return err
	}
	if c.LLM.UserDailyCallBudget < 0 || c.LLM.UserDailyCallBudget > MaxLLMUserDailyCallBudget {
		return fmt.Errorf("llm.user_daily_call_budget must be between 0 and %d", MaxLLMUserDailyCallBudget)
	}
	if err := c.Jobs.Validate(); err != nil {
		return err
	}
	if math.IsNaN(c.Evaluation.WeightTotal) || c.Evaluation.WeightTotal < 0 || c.Evaluation.WeightTotal > MaxEvaluationWeightTotal {
		return fmt.Errorf("evaluation.weight_total must be between 0 and %d", MaxEvaluationWeightTotal)
	}

	if c.Tracer.Enabled {
		if c.Tracer.SampleRate < 0.0 || c.Tracer.SampleRate > 1.0 {
			return fmt.Errorf("tracer.sample_rate must be between 0.0 and 1.0, got %f", c.Tracer.SampleRate)
		}
	}

	if strings.TrimSpace(c.PayOS.ClientID) == "" {
		return errors.New("payos.client_id is required")
	}
	if strings.TrimSpace(c.PayOS.APIKey) == "" {
		return errors.New("payos.api_key is required")
	}
	if strings.TrimSpace(c.PayOS.ChecksumKey) == "" {
		return errors.New("payos.checksum_key is required")
	}
	if c.PayOS.Timeout < 0 {
		return errors.New("payos.timeout must not be negative")
	}
	if c.PayOS.MaxRetries < 0 {
		return errors.New("payos.max_retries must not be negative")
	}

	return nil
}

func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}
