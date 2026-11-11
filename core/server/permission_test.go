package server

import (
	"errors"
	"net/http"
	"testing"
)

func TestResourcesCheck(t *testing.T) {
	rs := testResources()
	cases := []struct {
		name string
		p    Permission
		want error // nil means valid
	}{
		{"zero value", Permission{}, ErrNoPermission},
		{"public", Public, nil},
		{"authenticated", Authenticated, nil},
		{"standard verb", Permission{Resource: Projects, Verb: Read}, nil},
		{"extra verb on its resource", Permission{Resource: PermissionPrompts, Verb: Answer}, nil},
		{"another extra verb on its resource", Permission{Resource: Workflows, Verb: Run}, nil},
		{"unregistered resource", Permission{Resource: "Widgets", Verb: Read}, ErrUnknownResource},
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
			err := rs.Check(tc.p)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("Check(%#v) = %v, want valid", tc.p, err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("Check(%#v) = %v, want %v", tc.p, err, tc.want)
			}
		})
	}
}

// An empty registry accepts only Public and Authenticated: core registers no
// resources of its own.
func TestEmptyRegistryAcceptsNoResource(t *testing.T) {
	rs := NewResources()
	if err := rs.Check(Public); err != nil {
		t.Errorf("Public: %v", err)
	}
	if err := rs.Check(Authenticated); err != nil {
		t.Errorf("Authenticated: %v", err)
	}
	if err := rs.Check(Permission{Resource: Projects, Verb: Read}); !errors.Is(err, ErrUnknownResource) {
		t.Errorf("Projects: Read on an empty registry = %v, want ErrUnknownResource", err)
	}
}

func TestResourcesRegister(t *testing.T) {
	rs := NewResources()
	if err := rs.Register(ResourceDef{Name: "Reports", Verbs: []Verb{"Export"}}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	for _, p := range []Permission{{Resource: "Reports", Verb: Delete}, {Resource: "Reports", Verb: "Export"}} {
		if err := rs.Check(p); err != nil {
			t.Errorf("Check(%s) = %v", p, err)
		}
	}
	bad := map[string]struct {
		def  ResourceDef
		want error
	}{
		"duplicate":           {ResourceDef{Name: "Reports"}, ErrDuplicateResource},
		"empty name":          {ResourceDef{Name: ""}, ErrBadName},
		"name with colon":     {ResourceDef{Name: "A: B"}, ErrBadName},
		"name with space":     {ResourceDef{Name: "A B"}, ErrBadName},
		"bad verb":            {ResourceDef{Name: "Other", Verbs: []Verb{"do it"}}, ErrBadName},
		"redeclared standard": {ResourceDef{Name: "Other", Verbs: []Verb{Read}}, ErrBadName},
	}
	for name, c := range bad {
		if err := rs.Register(c.def); !errors.Is(err, c.want) {
			t.Errorf("%s: Register = %v, want %v", name, err, c.want)
		}
	}
	if got := rs.Names(); len(got) != 1 || got[0] != "Reports" {
		t.Errorf("Names() = %v, want [Reports]", got)
	}
}

func TestCheckString(t *testing.T) {
	rs := testResources()
	for s, ok := range map[string]bool{
		"Projects: Read": true, "Workflows: Run": true, "Projects: Run": false,
		"Widgets: Read": false, "Projects Read": false, "": false,
	} {
		if err := rs.CheckString(s); (err == nil) != ok {
			t.Errorf("CheckString(%q) = %v, want ok=%v", s, err, ok)
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

func TestBuildRefusesUnregisteredPermissions(t *testing.T) {
	invalid := map[string]Permission{
		"unknown resource": {Resource: "Widgets", Verb: Read},
		"unknown verb":     {Resource: Projects, Verb: "Write"},
		"Answer misused":   {Resource: Projects, Verb: Answer},
	}
	for name, p := range invalid {
		t.Run(name, func(t *testing.T) {
			// A permission naming an unregistered resource or verb is accepted
			// by Register, since its resource may be registered later, and
			// refused by Build.
			router := newTestRouter()
			if err := Register(router, op(http.MethodGet, "/x", p), okHandler); err != nil {
				t.Fatalf("Register refused %#v: %v", p, err)
			}
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
