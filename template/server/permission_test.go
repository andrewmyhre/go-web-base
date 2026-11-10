package server

import (
	"errors"
	"net/http"
	"testing"
)

func TestPermissionValidate(t *testing.T) {
	cases := []struct {
		name string
		p    Permission
		want error // nil means valid
	}{
		{"zero value", Permission{}, ErrNoPermission},
		{"public", Public, nil},
		{"standard verb", Permission{Resource: Projects, Verb: Read}, nil},
		{"Answer on PermissionPrompts", Permission{Resource: PermissionPrompts, Verb: Answer}, nil},
		{"Run on Workflows", Permission{Resource: Workflows, Verb: Run}, nil},
		{"unknown resource", Permission{Resource: "Widgets", Verb: Read}, ErrUnknownResource},
		{"resource only", Permission{Resource: Projects}, ErrUnknownVerb},
		{"verb only", Permission{Verb: Read}, ErrUnknownResource},
		{"unknown verb", Permission{Resource: Projects, Verb: "Write"}, ErrUnknownVerb},
		{"verb in the wrong case", Permission{Resource: Projects, Verb: "read"}, ErrUnknownVerb},
		{"Answer on another resource", Permission{Resource: Projects, Verb: Answer}, ErrVerbNotAllowed},
		{"Run on another resource", Permission{Resource: Projects, Verb: Run}, ErrVerbNotAllowed},
		{"Public with a resource", Permission{Resource: Projects, Verb: Read, public: true}, ErrUnknownResource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.p.Validate()
			if tc.want == nil {
				if err != nil {
					t.Fatalf("Validate(%#v) = %v, want valid", tc.p, err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate(%#v) = %v, want %v", tc.p, err, tc.want)
			}
		})
	}
}

// Every resource docs/roles.md names is accepted with every standard verb.
func TestEveryPlanResourceIsKnown(t *testing.T) {
	resources := []Resource{
		Projects, AgentConversations, AgentMessages, PermissionPrompts,
		Requests, ProjectFiles, Workflows, WorkflowSteps, Secrets,
		UserInvitation, WorkflowLink, Users, AccessReviews, Providers,
	}
	if len(resources) != len(knownResources) {
		t.Fatalf("%d resource constants, %d known", len(resources), len(knownResources))
	}
	for _, r := range resources {
		for _, v := range []Verb{Read, Create, Patch, Delete} {
			if err := (Permission{Resource: r, Verb: v}).Validate(); err != nil {
				t.Errorf("%s: %s refused: %v", r, v, err)
			}
		}
	}
}

func TestPublicIsNotTheZeroValue(t *testing.T) {
	if (Permission{}).IsPublic() {
		t.Fatal("the zero Permission reports IsPublic")
	}
	if !Public.IsPublic() {
		t.Fatal("Public does not report IsPublic")
	}
	if (Permission{Resource: Projects, Verb: Read}).IsPublic() {
		t.Fatal("a real permission reports IsPublic")
	}
	if Public == (Permission{}) {
		t.Fatal("Public equals the zero Permission")
	}
}

func TestPermissionString(t *testing.T) {
	if got := (Permission{Resource: Projects, Verb: Read}).String(); got != "Projects: Read" {
		t.Errorf("String() = %q, want %q", got, "Projects: Read")
	}
	if got := Public.String(); got != "public" {
		t.Errorf("Public.String() = %q, want %q", got, "public")
	}
}

func TestRegisterAndBuildRefuseInvalidPermissions(t *testing.T) {
	invalid := map[string]Permission{
		"unknown resource": {Resource: "Widgets", Verb: Read},
		"unknown verb":     {Resource: Projects, Verb: "Write"},
		"Answer misused":   {Resource: Projects, Verb: Answer},
	}
	for name, p := range invalid {
		t.Run(name, func(t *testing.T) {
			router := newTestRouter()
			if err := Register(router, op(http.MethodGet, "/x", p), okHandler); err == nil {
				t.Fatalf("Register accepted %#v", p)
			}
			// Bypass Register: startup must refuse it too.
			router.regs = append(router.regs, registration{
				method: http.MethodGet, path: "/x", permission: p,
				mount: router.regs[0].mount,
			})
			if _, err := router.Build(nil); err == nil {
				t.Fatalf("Build accepted %#v", p)
			}
		})
	}
}

func TestOperationMetadataCarriesPermission(t *testing.T) {
	router := newTestRouter()
	if err := Register(router, op(http.MethodGet, "/things", projectsRead), okHandler); err != nil {
		t.Fatalf("Register: %v", err)
	}
	api, _, err := router.build(nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	got := api.OpenAPI().Paths["/things"].Get.Metadata[MetadataPermission]
	if got != projectsRead {
		t.Fatalf("Metadata[%q] = %#v, want %#v", MetadataPermission, got, projectsRead)
	}
}

func TestAuthenticatedIsItsOwnDeclaration(t *testing.T) {
	if (Permission{}).IsAuthenticated() || Public.IsAuthenticated() {
		t.Fatal("the zero Permission or Public reports IsAuthenticated")
	}
	if !Authenticated.IsAuthenticated() || Authenticated.IsPublic() {
		t.Fatal("Authenticated must report IsAuthenticated and not IsPublic")
	}
	if err := Authenticated.Validate(); err != nil {
		t.Fatalf("Authenticated.Validate() = %v", err)
	}
	if got := Authenticated.String(); got != "authenticated" {
		t.Fatalf("Authenticated.String() = %q, want authenticated", got)
	}
	bad := Authenticated
	bad.Resource = Projects
	if err := bad.Validate(); err == nil {
		t.Fatal("Authenticated carrying a resource validated")
	}
	both := Authenticated
	both.public = true
	if err := both.Validate(); err == nil {
		t.Fatal("a permission both Public and Authenticated validated")
	}
}
