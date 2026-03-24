package bootstrap

import (
	"os"
	"testing"

	"github.com/gotunnel/pkg/crypto"
)

type testMetadataStore struct {
	values map[string]string
}

func (s *testMetadataStore) GetServerMetadata(key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", os.ErrNotExist
}

func (s *testMetadataStore) SetServerMetadata(key, value string) error {
	s.values[key] = value
	return nil
}

func TestLoadOrCreateTLSConfigPersistsStableCertificate(t *testing.T) {
	store := &testMetadataStore{values: map[string]string{}}

	first, err := loadOrCreateTLSConfig(store)
	if err != nil {
		t.Fatalf("first loadOrCreateTLSConfig returned error: %v", err)
	}
	second, err := loadOrCreateTLSConfig(store)
	if err != nil {
		t.Fatalf("second loadOrCreateTLSConfig returned error: %v", err)
	}

	firstFP := crypto.CertFingerprint(first.Certificates[0].Certificate[0])
	secondFP := crypto.CertFingerprint(second.Certificates[0].Certificate[0])
	if firstFP != secondFP {
		t.Fatalf("expected stable certificate fingerprint, got %s then %s", firstFP, secondFP)
	}
	if store.values[serverTLSCertMetadataKey] == "" || store.values[serverTLSKeyMetadataKey] == "" {
		t.Fatal("expected TLS material to be persisted in metadata store")
	}
}

func TestLoadOrCreateTLSConfigRejectsIncompleteMetadata(t *testing.T) {
	store := &testMetadataStore{
		values: map[string]string{
			serverTLSCertMetadataKey: "only-cert",
		},
	}

	if _, err := loadOrCreateTLSConfig(store); err == nil {
		t.Fatal("expected error for incomplete TLS metadata")
	}
}
