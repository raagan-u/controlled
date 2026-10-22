package platform_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/raagan-u/pay-pact/internal/platform"
)

func TestUUIDGenerator(t *testing.T) {
	t.Parallel()

	generator := platform.NewUUIDGeneratorFrom(bytes.NewReader(make([]byte, 16)))
	id, err := generator.NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if id != "00000000-0000-4000-8000-000000000000" {
		t.Fatalf("NewID() = %q", id)
	}
}

func TestUUIDGeneratorPropagatesReaderFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("entropy unavailable")
	generator := platform.NewUUIDGeneratorFrom(failingReader{err: wantErr})
	_, err := generator.NewID()
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewID() error = %v, want %v", err, wantErr)
	}
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}
