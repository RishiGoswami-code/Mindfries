package httpapi

import (
	"encoding/json"
	"net/http"
)

// titles are the people-facing line for each problem code in
// contracts/cli-api.yaml. The CLI shows a title but only branches on `code`.
var titles = map[string]string{
	"malformed":             "The request wasn't understood",
	"not_found":             "Not found",
	"rate_limited":          "Too many requests",
	"too_large":             "That's too large",
	"cli_too_old":           "This version of mindfries is too old",
	"setup_code_used":       "That setup code has already been used",
	"setup_code_expired":    "That setup code has expired",
	"setup_code_invalid":    "That setup code isn't valid",
	"authorization_pending": "Waiting for the candidate to approve the sign-in",
	"slow_down":             "Polling too fast",
	"access_denied":         "The sign-in was refused",
	"device_code_expired":   "The sign-in code has expired",
	"device_code_used":      "That sign-in has already been used",
	"device_code_invalid":   "That sign-in code isn't valid",
	"token_missing":         "No session token",
	"token_invalid":         "That session token isn't valid",
	"token_expired":         "The session token has expired",
	"token_revoked":         "The session token was revoked",
	"session_not_active":    "The session isn't active",
	"monitoring_not_ready":  "Monitoring isn't ready",
	"assistant_off":         "The assistant is off for this assessment",
	"budget_exhausted":      "The assistant's budget for this session is used up",
	"turn_not_waiting":      "That turn isn't waiting for tool results",
	"already_submitted":     "The work has already been submitted",
	"bundle_mismatch":       "The uploaded bundle doesn't match the manifest",
	"internal":              "Something went wrong",
	"forbidden":             "Not allowed",
	"not_configured":        "That isn't set up on this server",
	"unauthorized":          "Sign-in required",
	"conflict":              "That changed somewhere else",
	"no_coverage":           "No product covers that role and job description",
}

// problem writes the contract's problem JSON (RFC 9457 plus a stable `code`).
func problem(w http.ResponseWriter, status int, code, detail string) {
	problemWith(w, status, code, detail, nil)
}

// problemWith adds the code-specific fields (session_status, mismatches,
// minimum_version, retry_after_seconds …).
func problemWith(w http.ResponseWriter, status int, code, detail string, extra map[string]any) {
	title := titles[code]
	if title == "" {
		title = http.StatusText(status)
	}
	body := map[string]any{"code": code, "title": title, "status": status}
	if detail != "" {
		body["detail"] = detail
	}
	for k, v := range extra {
		body[k] = v
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
