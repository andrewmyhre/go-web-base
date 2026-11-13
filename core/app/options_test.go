package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// TestAppRoutesRunAfterBuiltinsAndAppearInTheSpec: the app's own routes go
// through server.Register like the built-ins, and are in the document.
func TestAppRoutesRunAfterBuiltinsAndAppearInTheSpec(t *testing.T) {
	o := testOpts
	o.Routes = func(r *server.Router, d Deps) error {
		if d.Sessions == nil {
			t.Error("Routes got no session store from the spec-only Deps")
		}
		return server.Register(r, server.Operation{
			OperationID: "get-ping", Method: http.MethodGet, Path: "/ping",
			Summary: "Ping", Permission: server.Public,
		}, health)
	}
	spec, err := OpenAPISpec(o)
	if err != nil {
		t.Fatalf("OpenAPISpec: %v", err)
	}
	var doc struct {
		Info  struct{ Title, Version string } `json:"info"`
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Paths["/ping"]["get"].OperationID != "get-ping" || doc.Paths["/health"]["get"].OperationID != "get-health" {
		t.Errorf("spec paths = %v, want /ping and /health", doc.Paths)
	}
	if doc.Info.Title != o.Name || doc.Info.Version != o.Version {
		t.Errorf("spec info = %+v, want title %q version %q", doc.Info, o.Name, o.Version)
	}
}

// TestAppRoutesCannotSkipThePermission: a route without a declared
// permission is refused at registration.
func TestAppRoutesCannotSkipThePermission(t *testing.T) {
	o := testOpts
	o.Routes = func(r *server.Router, _ Deps) error {
		return server.Register(r, server.Operation{OperationID: "no-perm", Method: http.MethodGet, Path: "/x"}, health)
	}
	if _, err := OpenAPISpec(o); err == nil {
		t.Fatal("OpenAPISpec accepted an app route with no permission")
	}
}

// TestOpenAPILogoutHasNoCookieParameter pins that making the session
// cookie name per app did not put a cookie parameter in the document.
func TestOpenAPILogoutHasNoCookieParameter(t *testing.T) {
	spec, err := OpenAPISpec(testOpts)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct{ In, Name string } `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	op, ok := doc.Paths[server.LogoutPath]["post"]
	if !ok {
		t.Fatalf("spec has no POST %s", server.LogoutPath)
	}
	if len(op.Parameters) != 0 {
		t.Errorf("logout parameters = %v, want none", op.Parameters)
	}
}

func TestOptionsAreValidated(t *testing.T) {
	for name, o := range map[string]Options{
		"no name":          {EnvPrefix: "A", CookiePrefix: "a", Roles: testOpts.Roles},
		"no env prefix":    {Name: "a", CookiePrefix: "a", Roles: testOpts.Roles},
		"no cookie prefix": {Name: "a", EnvPrefix: "A", Roles: testOpts.Roles},
		"no roles":         {Name: "a", EnvPrefix: "A", CookiePrefix: "a"},
	} {
		if _, err := OpenAPISpec(o); err == nil {
			t.Errorf("%s: OpenAPISpec accepted %+v", name, o)
		}
	}
	got, err := Options{Name: "a", EnvPrefix: "A", CookiePrefix: "a", Roles: testOpts.Roles}.validate()
	if err != nil || got.Version != "0.0.0" {
		t.Errorf("validate = %+v, %v, want the default version 0.0.0", got, err)
	}
}

func TestServeFlagsNameTheEnvPrefix(t *testing.T) {
	cmd, _, err := NewRootCmd(testOpts).Find([]string{"serve"})
	if err != nil {
		t.Fatal(err)
	}
	if usage := cmd.Flags().Lookup("addr").Usage; !strings.Contains(usage, "TESTAPP_ADDR") {
		t.Errorf("--addr usage = %q, want it to name TESTAPP_ADDR", usage)
	}
}

// TestDefaultRoleMustBeARole: a DefaultRole the graph does not define is
// refused at startup, not at the first sign-in; a defined one, and none,
// are accepted.
func TestDefaultRoleMustBeARole(t *testing.T) {
	o := testOpts
	o.DefaultRole = "No Such Role"
	if _, err := OpenAPISpec(o); err == nil || !strings.Contains(err.Error(), "DefaultRole") {
		t.Errorf("OpenAPISpec with an undefined DefaultRole = %v, want a DefaultRole error", err)
	}
	o.DefaultRole = db.AdminRole
	if _, err := OpenAPISpec(o); err == nil {
		t.Error("OpenAPISpec accepted Admin as the DefaultRole")
	}
	o.Roles = []db.RoleDef{{Name: db.AdminRole, Permissions: []string{"Projects: Read"}}, {Name: "Member", Permissions: []string{"Projects: Read"}}}
	o.DefaultRole = "Member"
	if _, err := OpenAPISpec(o); err != nil {
		t.Errorf("OpenAPISpec with a defined DefaultRole: %v", err)
	}
	o.DefaultRole = ""
	if _, err := OpenAPISpec(o); err != nil {
		t.Errorf("OpenAPISpec with no DefaultRole: %v", err)
	}
}
