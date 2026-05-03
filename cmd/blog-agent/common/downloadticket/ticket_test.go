package downloadticket

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSignerIssueAndVerify(t *testing.T) {
	signer := NewSigner("secret")
	now := time.UnixMilli(1_700_000_000_000)
	signer.now = func() time.Time { return now }

	token, claims, err := signer.Issue(Input{
		FileID:          "file-1",
		UserID:          "alice",
		ObjectKey:       "app/file/alice/demo.apk",
		StorageProvider: "obs",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if claims.UserID != "alice" {
		t.Fatalf("expected user alice, got %q", claims.UserID)
	}

	verified, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if verified.FileID != "file-1" {
		t.Fatalf("expected file_id=file-1, got %q", verified.FileID)
	}
	if verified.ObjectKey != "app/file/alice/demo.apk" {
		t.Fatalf("unexpected object key: %q", verified.ObjectKey)
	}
}

func TestSignerVerifyRejectsTamperedToken(t *testing.T) {
	signer := NewSigner("secret")
	signer.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }

	token, _, err := signer.Issue(Input{
		FileID:    "file-1",
		UserID:    "alice",
		ObjectKey: "obj",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	tampered := token[:len(token)-1] + "A"
	_, err = signer.Verify(tampered)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestSignerVerifyRejectsExpiredToken(t *testing.T) {
	signer := NewSigner("secret")
	base := time.UnixMilli(1_700_000_000_000)
	signer.now = func() time.Time { return base }

	token, _, err := signer.Issue(Input{
		FileID:    "file-1",
		UserID:    "alice",
		ObjectKey: "obj",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	signer.now = func() time.Time { return base.Add(2 * time.Minute) }
	_, err = signer.Verify(token)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestSignerRejectsMissingFields(t *testing.T) {
	signer := NewSigner("secret")
	_, _, err := signer.Issue(Input{
		FileID:    "file-1",
		UserID:    "",
		ObjectKey: "obj",
	}, time.Minute)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected required field error, got %v", err)
	}
}
