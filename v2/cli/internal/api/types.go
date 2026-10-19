// Package api is the CLI's side of v2/contracts/cli-api.yaml: the request
// and response types, and a client. Field names and JSON tags follow the
// contract exactly; change the contract first, then this file.
package api

import (
	"encoding/json"
	"time"
)

// Problem is an RFC 9457 problem with the contract's stable code.
type Problem struct {
	Type           string     `json:"type,omitempty"`
	Code           string     `json:"code"`
	Title          string     `json:"title"`
	Status         int        `json:"status"`
	Detail         string     `json:"detail,omitempty"`
	MinimumVersion string     `json:"minimum_version,omitempty"`
	SessionStatus  string     `json:"session_status,omitempty"`
	Mismatches     []Mismatch `json:"mismatches,omitempty"`
	// RetryAfter is filled from the Retry-After header, not the body.
	RetryAfter time.Duration `json:"-"`
}

func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Title + ": " + p.Detail
	}
	return p.Title
}

// Problem codes from the contract.
const (
	CodeMalformed        = "malformed"
	CodeNotFound         = "not_found"
	CodeRateLimited      = "rate_limited"
	CodeTooLarge         = "too_large"
	CodeCLITooOld        = "cli_too_old"
	CodeSetupCodeUsed    = "setup_code_used"
	CodeSetupCodeExpired = "setup_code_expired"
	CodeSetupCodeInvalid = "setup_code_invalid"
	// Browser sign-in (POST /sessions/device/token).
	CodeAuthorizationPending = "authorization_pending"
	CodeSlowDown             = "slow_down"
	CodeAccessDenied         = "access_denied"
	CodeDeviceCodeExpired    = "device_code_expired"
	CodeDeviceCodeUsed       = "device_code_used"
	CodeDeviceCodeInvalid    = "device_code_invalid"
	CodeTokenMissing         = "token_missing"
	CodeTokenInvalid         = "token_invalid"
	CodeTokenExpired         = "token_expired"
	CodeTokenRevoked         = "token_revoked"
	CodeSessionNotActive     = "session_not_active"
	CodeAssistantOff         = "assistant_off"
	CodeBudgetExhausted      = "budget_exhausted"
	CodeTurnNotWaiting       = "turn_not_waiting"
	CodeAlreadySubmitted     = "already_submitted"
	CodeBundleMismatch       = "bundle_mismatch"
	CodeInternal             = "internal"
)

type Mismatch struct {
	What     string `json:"what"`
	Path     string `json:"path,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

type VersionInfo struct {
	MinimumVersion  string `json:"minimum_version"`
	LatestVersion   string `json:"latest_version"`
	ReleaseNotesURL string `json:"release_notes_url,omitempty"`
}

type Device struct {
	OS        string `json:"os"`
	OSVersion string `json:"os_version,omitempty"`
	Arch      string `json:"arch"`
}

type ExchangeRequest struct {
	SetupCode string `json:"setup_code"`
	Device    Device `json:"device"`
}

// DeviceRequest starts a browser sign-in.
type DeviceRequest struct {
	Device Device `json:"device"`
}

// DeviceAuthorization is what the candidate confirms in the browser. The
// device code is the CLI's secret for polling; the user code is shown in
// both the terminal and the browser so the candidate can check they match.
type DeviceAuthorization struct {
	DeviceCode              string    `json:"device_code"`
	UserCode                string    `json:"user_code"`
	VerificationURI         string    `json:"verification_uri"`
	VerificationURIComplete string    `json:"verification_uri_complete"`
	ExpiresAt               time.Time `json:"expires_at"`
	IntervalSeconds         int       `json:"interval_seconds"`
}

// DeviceTokenRequest polls for the session token.
type DeviceTokenRequest struct {
	DeviceCode string `json:"device_code"`
}

type ExchangeResponse struct {
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Session      Session   `json:"session"`
}

// Session statuses.
const (
	StatusActive    = "active"
	StatusSubmitted = "submitted"
	StatusExpired   = "expired"
	StatusRevoked   = "revoked"
)

// Assistant modes.
const (
	ModeOff         = "off"
	ModeExplainOnly = "explain_only"
	ModeCodeAllowed = "code_allowed"
)

type Session struct {
	ID              string    `json:"id"`
	AssessmentID    string    `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title,omitempty"`
	Status          string    `json:"status"`
	StartedAt       time.Time `json:"started_at"`
	EndsAt          time.Time `json:"ends_at"`
	Policy          Policy    `json:"policy"`
	Intervals       Intervals `json:"intervals"`
	Limits          Limits    `json:"limits"`
	LastAckedSeq    int64     `json:"last_acked_seq"`
	SubmissionID    string    `json:"submission_id,omitempty"`
	// GitHub is set when the work goes to a repository in the candidate's
	// own GitHub account.
	GitHub *GitHub `json:"github,omitempty"`
}

// GitHub says how the session uses the candidate's GitHub: the CLI signs
// them in with GitHub's own device login (the Mindfries OAuth app,
// ClientID), creates a private repository, invites the reviewers and
// pushes the work there, so deployment can be assessed too.
type GitHub struct {
	Required bool     `json:"required"`
	ClientID string   `json:"client_id"`
	Scopes   []string `json:"scopes,omitempty"`
	// RepoName is a suggested repository name; the CLI adds a suffix if
	// the candidate already has one by that name.
	RepoName  string   `json:"repo_name,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
	// Deployment means the task asks for a deployed URL at submit.
	Deployment bool `json:"deployment,omitempty"`
}

// GitHubLogin is GitHub's device code, sent so the Mindfries sign-in page
// can show it next to an "Open GitHub" button.
type GitHubLogin struct {
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// GitHubRepo is the repository the work goes to.
type GitHubRepo struct {
	Login            string   `json:"login"`
	FullName         string   `json:"full_name"`
	HTMLURL          string   `json:"html_url"`
	DefaultBranch    string   `json:"default_branch"`
	Private          bool     `json:"private"`
	ReviewersInvited []string `json:"reviewers_invited,omitempty"`
}

type Policy struct {
	AssistantMode        string   `json:"assistant_mode"`
	AssistantBudget      *Budget  `json:"assistant_budget,omitempty"`
	OutsideAgentsAllowed bool     `json:"outside_agents_allowed"`
	ResearchDomains      []string `json:"research_domains,omitempty"`
}

type Intervals struct {
	HeartbeatSeconds int `json:"heartbeat_seconds"`
	SnapshotSeconds  int `json:"snapshot_seconds"`
	AgentScanSeconds int `json:"agent_scan_seconds"`
}

type Limits struct {
	MaxEventsPerBatch int   `json:"max_events_per_batch"`
	MaxBatchBytes     int   `json:"max_batch_bytes"`
	MaxSnapshotBytes  int   `json:"max_snapshot_bytes"`
	MaxBundleBytes    int64 `json:"max_bundle_bytes"`
}

type Budget struct {
	TokensTotal     int `json:"tokens_total"`
	TokensRemaining int `json:"tokens_remaining"`
}

type RevokeRequest struct {
	Reason string `json:"reason"` // stop, submitted, expired
}

type BundleManifest struct {
	TaskVersion      string       `json:"task_version"`
	ArchiveSHA256    string       `json:"archive_sha256"`
	ArchiveBytes     int64        `json:"archive_bytes"`
	Files            []BundleFile `json:"files"`
	Git              BundleGit    `json:"git"`
	RequiredRuntimes []Runtime    `json:"required_runtimes,omitempty"`
	SetupCommands    []string     `json:"setup_commands,omitempty"`
	CheckCommands    []string     `json:"check_commands,omitempty"`
	Signature        *Signature   `json:"signature,omitempty"`
}

type BundleFile struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
	Executable bool   `json:"executable,omitempty"`
	Locked     bool   `json:"locked,omitempty"`
}

type BundleGit struct {
	Branch      string `json:"branch"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
}

type Runtime struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	InstallURL string `json:"install_url,omitempty"`
}

type Signature struct {
	KeyID string `json:"key_id"`
	Value string `json:"value"`
}

type Heartbeat struct {
	SentAt  time.Time      `json:"sent_at"`
	Watcher HeartbeatWatch `json:"watcher"`
	Git     *HeartbeatGit  `json:"git,omitempty"`
}

type HeartbeatWatch struct {
	QueueDepth   int        `json:"queue_depth"`
	LastSentSeq  int64      `json:"last_sent_seq"`
	OfflineSince *time.Time `json:"offline_since,omitempty"`
}

type HeartbeatGit struct {
	Branch       string `json:"branch,omitempty"`
	Head         string `json:"head,omitempty"`
	ChangedFiles int    `json:"changed_files"`
}

type HeartbeatReply struct {
	ServerTime    time.Time `json:"server_time"`
	SessionStatus string    `json:"session_status"`
	EndsAt        time.Time `json:"ends_at"`
	Stop          bool      `json:"stop,omitempty"`
}

// Event is one watcher event. Data is the type-specific payload.
type Event struct {
	Seq        int64           `json:"seq"`
	OccurredAt time.Time       `json:"occurred_at"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	PrevHash   string          `json:"prev_hash"`
	Hash       string          `json:"hash"`
}

type EventBatch struct {
	Events []Event `json:"events"`
}

type EventBatchReply struct {
	AckedThroughSeq int64 `json:"acked_through_seq"`
	Stored          int   `json:"stored"`
	Duplicates      int   `json:"duplicates"`
}

type Snapshot struct {
	Seq         int64     `json:"seq"`
	TakenAt     time.Time `json:"taken_at"`
	BaseCommit  string    `json:"base_commit"`
	HeadCommit  string    `json:"head_commit,omitempty"`
	Patch       []byte    `json:"patch"` // gzipped; base64 in JSON
	PatchSHA256 string    `json:"patch_sha256"`
}

type AssistantTurnRequest struct {
	ConversationID string       `json:"conversation_id,omitempty"`
	Message        string       `json:"message"`
	Context        *TurnContext `json:"context,omitempty"`
}

type TurnContext struct {
	Files []ContextFile `json:"files,omitempty"`
}

type ContextFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Tool names.
const (
	ToolReadFile    = "read_file"
	ToolListFiles   = "list_files"
	ToolProposeEdit = "propose_edit"
	ToolRunCommand  = "run_command"
)

type ToolCall struct {
	CallID string          `json:"call_id"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
}

type ReadFileInput struct {
	Path string `json:"path"`
}

type ListFilesInput struct {
	Dir string `json:"dir,omitempty"`
}

type ProposeEditInput struct {
	Path   string     `json:"path"`
	Create bool       `json:"create,omitempty"`
	Edits  []TextEdit `json:"edits"`
}

type TextEdit struct {
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

type RunCommandInput struct {
	Command        string `json:"command"`
	Cwd            string `json:"cwd,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// Tool result statuses.
const (
	ToolOK       = "ok"
	ToolRejected = "rejected"
	ToolRefused  = "refused"
	ToolError    = "error"
)

type ToolResults struct {
	Results []ToolResult `json:"results"`
}

type ToolResult struct {
	CallID      string       `json:"call_id"`
	Status      string       `json:"status"`
	Output      string       `json:"output,omitempty"`
	ExitCode    *int         `json:"exit_code,omitempty"`
	AppliedEdit *AppliedEdit `json:"applied_edit,omitempty"`
}

type AppliedEdit struct {
	EditID       string      `json:"edit_id"`
	Path         string      `json:"path"`
	BeforeSHA256 string      `json:"before_sha256,omitempty"`
	AfterSHA256  string      `json:"after_sha256"`
	Ranges       []LineRange `json:"ranges"`
}

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Assistant stream event types.
const (
	StreamTurnStarted     = "turn.started"
	StreamTextDelta       = "text.delta"
	StreamResearchSource  = "research.source"
	StreamResearchRefused = "research.refused"
	StreamToolCall        = "tool.call"
	StreamCallsPending    = "tool.calls_pending"
	StreamTurnCompleted   = "turn.completed"
	StreamError           = "error"
)

// StreamEvent is the union of every assistant stream event. Type says which
// fields are set.
type StreamEvent struct {
	Type           string    `json:"type"`
	TurnID         string    `json:"turn_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Mode           string    `json:"mode,omitempty"`
	Budget         *Budget   `json:"budget,omitempty"`
	Text           string    `json:"text,omitempty"`
	URL            string    `json:"url,omitempty"`
	Title          string    `json:"title,omitempty"`
	Domain         string    `json:"domain,omitempty"`
	Call           *ToolCall `json:"call,omitempty"`
	CallIDs        []string  `json:"call_ids,omitempty"`
	Code           string    `json:"code,omitempty"`
	Message        string    `json:"message,omitempty"`
}

type SubmissionManifest struct {
	TaskVersion     string           `json:"task_version"`
	Branch          string           `json:"branch"`
	BaseCommit      string           `json:"base_commit"`
	HeadCommit      string           `json:"head_commit"`
	CommitMessage   string           `json:"commit_message"`
	Files           []SubmissionFile `json:"files"`
	Checks          []CheckResult    `json:"checks,omitempty"`
	BundleSHA256    string           `json:"bundle_sha256"`
	BundleBytes     int64            `json:"bundle_bytes"`
	CreatedAt       time.Time        `json:"created_at"`
	TaskFilesIntact bool             `json:"task_files_intact"`
	// GitHubRepo and PushedCommit: where the work was pushed, if anywhere.
	GitHubRepo   string `json:"github_repo,omitempty"`
	PushedCommit string `json:"pushed_commit,omitempty"`
	// DeploymentURL is where the candidate deployed it, if the task asks.
	DeploymentURL string `json:"deployment_url,omitempty"`
}

type SubmissionFile struct {
	Path   string `json:"path"`
	Change string `json:"change"` // added, modified, deleted, renamed
	SHA256 string `json:"sha256,omitempty"`
}

type CheckResult struct {
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	Summary    string `json:"summary,omitempty"`
}

// Submission statuses.
const (
	SubmissionAwaitingBundle = "awaiting_bundle"
	SubmissionVerified       = "verified"
	SubmissionRejected       = "rejected"
)

type Submission struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	HeadCommit string     `json:"head_commit,omitempty"`
	// DeploymentStatus is the HTTP status the server got from the
	// deployment URL (0: not checked or unreachable).
	DeploymentStatus int `json:"deployment_status,omitempty"`
}
