package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

func TestPrintErrorAbortedExitCode(t *testing.T) {
	// A run cancelled by ctrl-c / SIGTERM must exit 130 (128 + SIGINT) so
	// shell scripts can tell a user abort apart from a genuine failure.
	var buf bytes.Buffer
	wrapped := fmt.Errorf("call GET /v1/clusters/conn-1: %w", context.Canceled)
	code := printError(&buf, wrapped)
	if code != exitAborted {
		t.Fatalf("aborted run: got exit code %d, want %d", code, exitAborted)
	}
	if got := buf.String(); got != "Error: aborted\n" {
		t.Fatalf("aborted run: got stderr %q, want %q", got, "Error: aborted\n")
	}
}

func TestPrintErrorGenericExitCode(t *testing.T) {
	var buf bytes.Buffer
	code := printError(&buf, errors.New("boom"))
	if code != 1 {
		t.Fatalf("generic error: got exit code %d, want 1", code)
	}
	if got := buf.String(); got != "Error: boom\n" {
		t.Fatalf("generic error: got stderr %q", got)
	}
}

func TestPrintErrorConfigHintExitCode(t *testing.T) {
	var buf bytes.Buffer
	code := printError(&buf, config.ErrNoCurrent)
	if code != 1 {
		t.Fatalf("config error: got exit code %d, want 1", code)
	}
	if got := buf.String(); !bytes.Contains([]byte(got), []byte("hint:")) {
		t.Fatalf("config error: stderr missing hint line: %q", got)
	}
}

func TestPrintErrorAPIErrorExitCode(t *testing.T) {
	// 4xx must exit with exitClientError (4) so CI/scripts can tell a
	// caller-side failure (bad request, auth, not found — retry won't help)
	// apart from a 5xx (retry may help) and from a transport/parse error
	// (which keeps the generic exit code 1).
	cases := []struct {
		name   string
		status int
		want   int
	}{
		{"400 bad request", 400, exitClientError},
		{"404 not found", 404, exitClientError},
		{"422 unprocessable", 422, exitClientError},
		{"499 client-side boundary", 499, exitClientError},
		{"500 server error", 500, exitServerError},
		{"502 bad gateway", 502, exitServerError},
		{"503 unavailable", 503, exitServerError},
		{"599 server boundary", 599, exitServerError},
		{"600 out of band", 600, 1},
		{"399 out of band", 399, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			apiErr := &client.APIError{StatusCode: tc.status, Detail: "boom"}
			code := printError(&buf, apiErr)
			if code != tc.want {
				t.Fatalf("status %d: got exit code %d, want %d", tc.status, code, tc.want)
			}
			if got := buf.String(); !bytes.Contains([]byte(got), []byte("boom")) {
				t.Fatalf("status %d: stderr missing detail: %q", tc.status, got)
			}
		})
	}
}

func TestPrintErrorAPIErrorWrappedExitCode(t *testing.T) {
	// errors.As must still see the APIError through one layer of wrapping
	// (the client uses fmt.Errorf("call %s %s: %w", ...) in places); verify
	// the structured exit code survives wrapping.
	var buf bytes.Buffer
	wrapped := fmt.Errorf("list connections: %w", &client.APIError{StatusCode: 503, Detail: "upstream down"})
	code := printError(&buf, wrapped)
	if code != exitServerError {
		t.Fatalf("wrapped 503: got exit code %d, want %d", code, exitServerError)
	}
}
