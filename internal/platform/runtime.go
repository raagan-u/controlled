// Package platform provides small process-level adapters used by application modules.
package platform

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type TokenGenerator struct {
	reader io.Reader
}

func NewTokenGenerator() TokenGenerator {
	return TokenGenerator{reader: rand.Reader}
}

func NewTokenGeneratorFrom(reader io.Reader) TokenGenerator {
	return TokenGenerator{reader: reader}
}

func (g TokenGenerator) NewToken() (string, error) {
	reader := g.reader
	if reader == nil {
		reader = rand.Reader
	}

	var value [32]byte
	if _, err := io.ReadFull(reader, value[:]); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

type UUIDGenerator struct {
	reader io.Reader
}

func NewUUIDGenerator() UUIDGenerator {
	return UUIDGenerator{reader: rand.Reader}
}

func NewUUIDGeneratorFrom(reader io.Reader) UUIDGenerator {
	return UUIDGenerator{reader: reader}
}

func (g UUIDGenerator) NewID() (string, error) {
	reader := g.reader
	if reader == nil {
		reader = rand.Reader
	}

	var value [16]byte
	if _, err := io.ReadFull(reader, value[:]); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	), nil
}
