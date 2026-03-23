package service

import (
	"errors"
	"testing"
)

func TestUpdateServiceApplyClientDelegatesToRuntime(t *testing.T) {
	runtime := &fakeUpdateRuntime{}
	svc := NewUpdateService(runtime)

	if err := svc.ApplyClient("client-1", "https://example.com/client.tar.gz"); err != nil {
		t.Fatalf("ApplyClient returned error: %v", err)
	}
	if runtime.clientID != "client-1" || runtime.downloadURL != "https://example.com/client.tar.gz" {
		t.Fatalf("unexpected runtime call: %+v", runtime)
	}
}

func TestUpdateServiceApplyClientReturnsRuntimeError(t *testing.T) {
	expected := errors.New("runtime failed")
	svc := NewUpdateService(&fakeUpdateRuntime{err: expected})

	err := svc.ApplyClient("client-1", "https://example.com/client.tar.gz")
	if !errors.Is(err, expected) {
		t.Fatalf("expected runtime error, got %v", err)
	}
}

type fakeUpdateRuntime struct {
	clientID    string
	downloadURL string
	err         error
}

func (r *fakeUpdateRuntime) SendUpdateToClient(clientID, downloadURL string) error {
	r.clientID = clientID
	r.downloadURL = downloadURL
	return r.err
}
