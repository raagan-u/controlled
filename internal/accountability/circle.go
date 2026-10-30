package accountability

import (
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

type CircleID string

type Circle struct {
	id        CircleID
	ownerID   identity.UserID
	name      string
	partners  []identity.UserID
	createdAt time.Time
}

func NewCircle(
	id CircleID,
	ownerID identity.UserID,
	name string,
	partners []identity.UserID,
	createdAt time.Time,
) (Circle, error) {
	name = strings.TrimSpace(name)
	if id == "" {
		return Circle{}, ErrInvalidCircleID
	}
	if ownerID == "" {
		return Circle{}, ErrInvalidOwnerID
	}
	if name == "" {
		return Circle{}, ErrMissingCircleName
	}
	if createdAt.IsZero() {
		return Circle{}, ErrInvalidTimestamp
	}
	if len(partners) == 0 {
		return Circle{}, ErrMissingPartners
	}

	ordered := make([]identity.UserID, len(partners))
	seen := make(map[identity.UserID]struct{}, len(partners))
	for index, partnerID := range partners {
		if partnerID == "" {
			return Circle{}, ErrInvalidPartnerID
		}
		if partnerID == ownerID {
			return Circle{}, ErrSelfPartner
		}
		if _, exists := seen[partnerID]; exists {
			return Circle{}, ErrDuplicatePartner
		}
		seen[partnerID] = struct{}{}
		ordered[index] = partnerID
	}

	return Circle{
		id: id, ownerID: ownerID, name: name,
		partners: ordered, createdAt: createdAt.UTC(),
	}, nil
}

func (c Circle) ID() CircleID                { return c.id }
func (c Circle) OwnerID() identity.UserID    { return c.ownerID }
func (c Circle) Name() string                { return c.name }
func (c Circle) CreatedAt() time.Time        { return c.createdAt }
func (c Circle) Partners() []identity.UserID { return append([]identity.UserID(nil), c.partners...) }
