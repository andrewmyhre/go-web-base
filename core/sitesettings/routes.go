package sitesettings

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/andrewmyhre/go-web-base/core/server"
)

// Paths of the admin routes, under the app's APIPrefix. They are not
// /settings: that is the page in the UI, and with no APIPrefix the API and
// the UI share one origin.
const (
	SettingsPath = "/admin/settings"
	InvitesPath  = "/admin/invites"
)

// Resources are the two resources core registers for these routes, with the
// verbs they use (all standard).
func Resources() []server.ResourceDef {
	return []server.ResourceDef{{Name: SettingsResource}, {Name: InvitesResource}}
}

// AdminPermissions are what an Admin needs to use every route here. core
// grants them to the Admin role.
func AdminPermissions() []server.Permission {
	return []server.Permission{
		{Resource: SettingsResource, Verb: server.Read},
		{Resource: SettingsResource, Verb: server.Patch},
		{Resource: InvitesResource, Verb: server.Read},
		{Resource: InvitesResource, Verb: server.Create},
		{Resource: InvitesResource, Verb: server.Delete},
	}
}

// SettingBody is one setting in a response.
type SettingBody struct {
	Key         string     `json:"key" doc:"The setting's key."`
	Label       string     `json:"label" doc:"A short name for display."`
	Description string     `json:"description" doc:"What the setting does."`
	Options     []string   `json:"options" nullable:"false" doc:"The values the setting allows."`
	Value       string     `json:"value" doc:"The current value."`
	Default     string     `json:"default" doc:"The value when nobody has set one."`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" doc:"When an admin last changed it. Absent while it is at its default."`
	UpdatedBy   string     `json:"updated_by,omitempty" doc:"Who last changed it (an account id)."`
}

// InviteBody is one invite in a response.
type InviteBody struct {
	ID         int64      `json:"id" doc:"The invite's id."`
	Email      string     `json:"email" doc:"The invited address, lower case."`
	Status     string     `json:"status" enum:"pending,revoked,consumed" doc:"pending can still be used; revoked was withdrawn; consumed admitted a first sign-in."`
	CreatedBy  string     `json:"created_by,omitempty" doc:"Who created it (an account id)."`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	ConsumedAt *time.Time `json:"consumed_at,omitempty"`
	ConsumedBy string     `json:"consumed_by,omitempty" doc:"The account id that used it."`
}

func inviteBody(i Invite) InviteBody {
	b := InviteBody{ID: i.ID, Email: i.Email, CreatedBy: i.CreatedBy, CreatedAt: i.CreatedAt, RevokedAt: i.RevokedAt, ConsumedAt: i.ConsumedAt, ConsumedBy: i.ConsumedBy, Status: "pending"}
	switch {
	case i.RevokedAt != nil:
		b.Status = "revoked"
	case i.ConsumedAt != nil:
		b.Status = "consumed"
	}
	return b
}

// ListSettingsOutput is the response of GET /admin/settings.
type ListSettingsOutput struct {
	Body struct {
		Settings []SettingBody `json:"settings" nullable:"false"`
	}
}

// UpdateSettingInput is the request of PUT /admin/settings/{key}.
type UpdateSettingInput struct {
	Key  string `path:"key" doc:"The setting's key."`
	Body struct {
		Value string `json:"value" minLength:"1" doc:"The new value; it must be one of the setting's options."`
	}
}

// UpdateSettingOutput is the response of PUT /admin/settings/{key}.
type UpdateSettingOutput struct {
	Body SettingBody
}

// ListInvitesOutput is the response of GET /admin/invites.
type ListInvitesOutput struct {
	Body struct {
		Invites []InviteBody `json:"invites" nullable:"false" doc:"Every invite, newest first."`
	}
}

// CreateInviteInput is the request of POST /admin/invites.
type CreateInviteInput struct {
	Body struct {
		Email string `json:"email" minLength:"3" maxLength:"254" doc:"The address to invite. Its first sign-in with this Google-verified address is let in."`
	}
}

// CreateInviteOutput is the response of POST /admin/invites.
type CreateInviteOutput struct {
	Body InviteBody
}

// RevokeInviteInput is the request of DELETE /admin/invites/{id}.
type RevokeInviteInput struct {
	ID int64 `path:"id" minimum:"1" doc:"The invite's id."`
}

// RevokeInviteOutput is the response of DELETE /admin/invites/{id}: 204.
type RevokeInviteOutput struct{}

// Routes registers the admin routes on r. prefix is the app's APIPrefix.
// store may be nil, as it is when the OpenAPI document is generated or the
// server has no database: the routes are then described but answer 503.
// Each route declares its permission on one of core's two resources, so an
// Admin (and anyone an app grants them to) can use it and nobody else can.
func Routes(r *server.Router, prefix string, store *Store) error {
	unavailable := huma.Error503ServiceUnavailable("site settings need the database, which is not configured")
	actor := func(ctx context.Context) string {
		if s, ok := server.SessionFromContext(ctx); ok {
			return s.UserID
		}
		return ""
	}
	settingBody := func(st Setting) SettingBody {
		return SettingBody{Key: st.Key, Label: st.Label, Description: st.Description, Options: st.Options, Value: st.Value, Default: st.Default, UpdatedAt: st.UpdatedAt, UpdatedBy: st.UpdatedBy}
	}
	storeErr := func(err error) error {
		switch {
		case errors.Is(err, ErrUnknownSetting), errors.Is(err, ErrInviteNotFound):
			return huma.Error404NotFound(err.Error())
		case errors.Is(err, ErrInvalidValue), errors.Is(err, ErrInvalidEmail):
			return huma.Error422UnprocessableEntity(err.Error())
		case errors.Is(err, ErrInvitePending):
			return huma.Error409Conflict(err.Error())
		}
		return huma.Error503ServiceUnavailable("site settings are unavailable; try again")
	}

	if err := server.Register(r, server.Operation{
		OperationID: "list-site-settings",
		Method:      http.MethodGet,
		Path:        prefix + SettingsPath,
		Summary:     "List the site settings",
		Tags:        []string{"admin"},
		Permission:  server.Permission{Resource: SettingsResource, Verb: server.Read},
	}, func(ctx context.Context, _ *struct{}) (*ListSettingsOutput, error) {
		if store == nil {
			return nil, unavailable
		}
		list, err := store.List(ctx)
		if err != nil {
			return nil, storeErr(err)
		}
		out := &ListSettingsOutput{}
		out.Body.Settings = make([]SettingBody, 0, len(list))
		for _, st := range list {
			out.Body.Settings = append(out.Body.Settings, settingBody(st))
		}
		return out, nil
	}); err != nil {
		return err
	}

	if err := server.Register(r, server.Operation{
		OperationID: "update-site-setting",
		Method:      http.MethodPut,
		Path:        prefix + SettingsPath + "/{key}",
		Summary:     "Change a site setting",
		Tags:        []string{"admin"},
		Permission:  server.Permission{Resource: SettingsResource, Verb: server.Patch},
	}, func(ctx context.Context, in *UpdateSettingInput) (*UpdateSettingOutput, error) {
		if store == nil {
			return nil, unavailable
		}
		if err := store.Set(ctx, in.Key, in.Body.Value, actor(ctx)); err != nil {
			return nil, storeErr(err)
		}
		list, err := store.List(ctx)
		if err != nil {
			return nil, storeErr(err)
		}
		for _, st := range list {
			if st.Key == in.Key {
				return &UpdateSettingOutput{Body: settingBody(st)}, nil
			}
		}
		return nil, huma.Error404NotFound(ErrUnknownSetting.Error())
	}); err != nil {
		return err
	}

	if err := server.Register(r, server.Operation{
		OperationID: "list-site-invites",
		Method:      http.MethodGet,
		Path:        prefix + InvitesPath,
		Summary:     "List the sign-up invites",
		Tags:        []string{"admin"},
		Permission:  server.Permission{Resource: InvitesResource, Verb: server.Read},
	}, func(ctx context.Context, _ *struct{}) (*ListInvitesOutput, error) {
		if store == nil {
			return nil, unavailable
		}
		list, err := store.ListInvites(ctx)
		if err != nil {
			return nil, storeErr(err)
		}
		out := &ListInvitesOutput{}
		out.Body.Invites = make([]InviteBody, 0, len(list))
		for _, i := range list {
			out.Body.Invites = append(out.Body.Invites, inviteBody(i))
		}
		return out, nil
	}); err != nil {
		return err
	}

	if err := server.Register(r, server.Operation{
		OperationID: "create-site-invite",
		Method:      http.MethodPost,
		Path:        prefix + InvitesPath,
		Summary:     "Invite an email address",
		Description: "Core sends no email: share the site's sign-in link with the person.",
		Tags:        []string{"admin"},
		Permission:  server.Permission{Resource: InvitesResource, Verb: server.Create},
	}, func(ctx context.Context, in *CreateInviteInput) (*CreateInviteOutput, error) {
		if store == nil {
			return nil, unavailable
		}
		inv, err := store.CreateInvite(ctx, in.Body.Email, actor(ctx))
		if err != nil {
			return nil, storeErr(err)
		}
		return &CreateInviteOutput{Body: inviteBody(inv)}, nil
	}); err != nil {
		return err
	}

	return server.Register(r, server.Operation{
		OperationID: "revoke-site-invite",
		Method:      http.MethodDelete,
		Path:        prefix + InvitesPath + "/{id}",
		Summary:     "Revoke a pending invite",
		Tags:        []string{"admin"},
		Permission:  server.Permission{Resource: InvitesResource, Verb: server.Delete},
	}, func(ctx context.Context, in *RevokeInviteInput) (*RevokeInviteOutput, error) {
		if store == nil {
			return nil, unavailable
		}
		if _, err := store.RevokeInvite(ctx, in.ID); err != nil {
			return nil, storeErr(err)
		}
		return &RevokeInviteOutput{}, nil
	})
}
