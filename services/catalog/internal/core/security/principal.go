package security

import (
	"context"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSeller   Role = "seller"
	RoleAdmin    Role = "admin"
	RoleCustomer Role = "customer"
)

type Principal struct {
	Sub   uuid.UUID
	Roles []Role
}

func (p Principal) HasRole(role Role) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (p Principal) IsAdmin() bool { return p.HasRole(RoleAdmin) }

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
