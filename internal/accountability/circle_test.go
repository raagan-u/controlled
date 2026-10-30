package accountability_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
)

func TestCirclePreservesExplicitPartnerOrder(t *testing.T) {
	partners := []identity.UserID{"third", "first", "second"}
	circle, err := accountability.NewCircle("circle", "owner", "Close friends", partners, time.Now())
	if err != nil {
		t.Fatalf("new circle: %v", err)
	}
	if !reflect.DeepEqual(circle.Partners(), partners) {
		t.Fatalf("partners = %v, want %v", circle.Partners(), partners)
	}

	returned := circle.Partners()
	returned[0] = "mutated"
	if reflect.DeepEqual(circle.Partners(), returned) {
		t.Fatal("partner order leaked mutable storage")
	}
}

func TestCircleRequiresDistinctNonOwnerPartners(t *testing.T) {
	tests := []struct {
		name     string
		partners []identity.UserID
		want     error
	}{
		{name: "missing", want: accountability.ErrMissingPartners},
		{name: "owner", partners: []identity.UserID{"owner"}, want: accountability.ErrSelfPartner},
		{name: "duplicate", partners: []identity.UserID{"partner", "partner"}, want: accountability.ErrDuplicatePartner},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := accountability.NewCircle("circle", "owner", "Circle", test.partners, time.Now())
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}
