package httpx

import (
	"context"

	"github.com/google/uuid"
)

type Role string

const (
	RoleParent Role = "PARENT"
	RoleAgency Role = "AGENCY"
	RoleAdmin  Role = "ADMIN"
)

// User is the authenticated caller, loaded from the database on every request.
type User struct {
	ID          uuid.UUID
	Email       string
	FullName    string
	Role        Role
	AgencyID    *uuid.UUID
	Designation *string
	Language    string
	IP          string
}

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, keyUser, u)
}

func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(keyUser).(*User)
	return u
}
