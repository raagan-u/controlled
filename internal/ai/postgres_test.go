package ai

import (
	"context"
	"errors"
	"testing"
)

func TestPostgresAssessmentStoreRequiresPool(t *testing.T) {
	t.Parallel()
	if err := NewPostgresAssessmentStore(nil).SaveAssessment(context.Background(), StoredAssessment{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want %v", err, ErrUnavailable)
	}
}
