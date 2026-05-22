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
	var buf bytes.Buffer
	apiErr := &client.APIError{StatusCode: 404, Detail: "connection not found"}
	code := printError(&buf, apiErr)
	if code != 1 {
		t.Fatalf("API error: got exit code %d, want 1", code)
	}
	if got := buf.String(); !bytes.Contains([]byte(got), []byte("connection not found")) {
		t.Fatalf("API error: stderr missing detail: %q", got)
	}
}
