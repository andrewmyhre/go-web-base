package server

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"
)

// MeRole is one role the signed-in user is bound to, and where.
type MeRole struct {
	Role   string `json:"role" doc:"Role name, for example Admin."`
	Domain string `json:"domain" doc:"Where the binding applies: global, or a project ID."`
}

// MeOutput is the response of GET /me: the signed-in user.
type MeOutput struct {
	Body struct {
		ID     string   `json:"id" doc:"Google account ID (the OIDC sub). This is the identity."`
		Name   string   `json:"name" doc:"Display name."`
		Email  string   `json:"email" doc:"Email, shown only and never matched on. Empty unless Google says it is verified."`
		Avatar string   `json:"avatar" doc:"Avatar URL."`
		Roles  []MeRole `json:"roles" nullable:"false" doc:"Every role the user is bound to, in every domain, read from the role bindings. Sorted by domain, then role."`
	}
}

// NewMe returns the GET /me handler, reading the user's roles from roles.
// The route is declared Authenticated, so the authorization check has
// already answered 401 for a request without a session; the check here is
// a second line of defence. A nil roles reports no roles (a server without
// a database, where nobody can sign in anyway). A failure to read the
// roles is a 503, never an empty list, so an outage never reads as "no
// roles".
func NewMe(roles RoleReader, log *zap.Logger) func(context.Context, *struct{}) (*MeOutput, error) {
	if log == nil {
		log = zap.NewNop()
	}
	return func(ctx context.Context, _ *struct{}) (*MeOutput, error) {
		s, ok := SessionFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not signed in")
		}
		out := &MeOutput{}
		out.Body.ID = s.UserID
		out.Body.Name = s.Name
		out.Body.Email = s.Email
		out.Body.Avatar = s.Avatar
		out.Body.Roles = []MeRole{}
		if roles == nil {
			return out, nil
		}
		bindings, err := roles.RoleBindingsOf(s.UserID)
		if err != nil {
			log.Warn("read role bindings failed", zap.String("request_id", RequestIDFromContext(ctx)), zap.Error(err))
			return nil, huma.Error503ServiceUnavailable("could not read your roles; try again")
		}
		for _, b := range bindings {
			out.Body.Roles = append(out.Body.Roles, MeRole{Role: b.Role, Domain: b.Domain})
		}
		return out, nil
	}
}
