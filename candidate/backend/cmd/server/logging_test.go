package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewLoggerWritesLevelAndMessageAsJSON(t *testing.T) {
	var buf bytes.Buffer
	newLogger(&buf).Info("request", "status", 200)
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("not one JSON line: %q (%v)", buf.String(), err)
	}
	if line["level"] != "INFO" || line["message"] != "request" || line["status"] != float64(200) {
		t.Errorf("got %v, want level INFO, message \"request\", status 200", line)
	}
}
