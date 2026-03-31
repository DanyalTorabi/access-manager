package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelInfo, &buf)

	Info("hello", slog.String("key", "val"))

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v — raw: %s", err, buf.String())
	}
	if m["msg"] != "hello" {
		t.Fatalf("want msg=hello, got %v", m["msg"])
	}
	if m["key"] != "val" {
		t.Fatalf("want key=val, got %v", m["key"])
	}
	if m["level"] != "INFO" {
		t.Fatalf("want level=INFO, got %v", m["level"])
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelInfo, &buf)

	Warn("caution")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["level"] != "WARN" {
		t.Fatalf("want level=WARN, got %v", m["level"])
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelInfo, &buf)

	Error("boom", slog.String("error", "fail"))

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["level"] != "ERROR" {
		t.Fatalf("want level=ERROR, got %v", m["level"])
	}
	if m["error"] != "fail" {
		t.Fatalf("want error=fail, got %v", m["error"])
	}
}

func TestAudit(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelInfo, &buf)

	Audit(context.Background(), "grant_user_permission",
		slog.String("domain_id", "d1"),
		slog.String("user_id", "u1"),
		slog.String("permission_id", "p1"),
	)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v — raw: %s", err, buf.String())
	}
	if m["msg"] != "audit" {
		t.Fatalf("want msg=audit, got %v", m["msg"])
	}
	if m["audit"] != true {
		t.Fatalf("want audit=true, got %v", m["audit"])
	}
	if m["action"] != "grant_user_permission" {
		t.Fatalf("want action=grant_user_permission, got %v", m["action"])
	}
	if m["domain_id"] != "d1" {
		t.Fatalf("want domain_id=d1, got %v", m["domain_id"])
	}
}

func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelWarn, &buf)

	Info("should be filtered")

	if buf.Len() > 0 {
		t.Fatalf("INFO should be filtered at WARN level, got: %s", buf.String())
	}
}
