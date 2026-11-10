package server

import (
	"errors"
	"fmt"
)

// A Permission is what a caller must hold to invoke a route: a verb on a
// resource, in the form docs/roles.md uses in its Roles section,
// e.g. "Projects: Read". The one exception is Public, for the few routes
// no role is needed for.
//
// The zero value is not a permission: a route declaring Permission{} is
// refused with ErrNoPermission, exactly as an undeclared one is. Public and
// Authenticated are deliberate, separate declarations, each marked by an
// unexported field that no code outside this package can set.
type Permission struct {
	Resource Resource
	Verb     Verb

	public        bool
	authenticated bool
}

// Public declares that a route needs no role, for example GET /health. It
// must be named explicitly; it is never the result of leaving a
// permission out.
var Public = Permission{public: true}

// Authenticated declares that a route needs a signed-in user but no role,
// for example GET /me: every user may read their own account. A request
// without a session is refused with 401.
var Authenticated = Permission{authenticated: true}

// IsPublic reports whether p is Public. The authorisation check lets a
// public route through and checks every other one.
func (p Permission) IsPublic() bool { return p.public }

// IsAuthenticated reports whether p is Authenticated.
func (p Permission) IsAuthenticated() bool { return p.authenticated }

// String gives the docs/roles.md form, "Resource: Verb", or "public" or
// "authenticated".
func (p Permission) String() string {
	if p.IsPublic() {
		return "public"
	}
	if p.IsAuthenticated() {
		return "authenticated"
	}
	return string(p.Resource) + ": " + string(p.Verb)
}

// Validation errors. Each is wrapped with the offending route.
var (
	// ErrNoPermission is returned for a route that declares no permission
	// (the zero value).
	ErrNoPermission = errors.New("route has no declared permission")
	// ErrUnknownResource is returned for a resource docs/roles.md does not name.
	ErrUnknownResource = errors.New("unknown permission resource")
	// ErrUnknownVerb is returned for a verb docs/roles.md does not name.
	ErrUnknownVerb = errors.New("unknown permission verb")
	// ErrVerbNotAllowed is returned for a verb used on a resource it does
	// not apply to, e.g. Answer on anything but PermissionPrompts.
	ErrVerbNotAllowed = errors.New("verb not allowed on resource")
)

// Validate reports whether p is a permission a route may declare.
func (p Permission) Validate() error {
	if p.public && p.authenticated {
		return fmt.Errorf("%w: a permission cannot be both Public and Authenticated", ErrUnknownResource)
	}
	if p.public || p.authenticated {
		if p.Resource != "" || p.Verb != "" {
			return fmt.Errorf("%w: %s carries a resource or verb (%q, %q)", ErrUnknownResource, p, p.Resource, p.Verb)
		}
		return nil
	}
	if p == (Permission{}) {
		return ErrNoPermission
	}
	if !knownResources[p.Resource] {
		return fmt.Errorf("%w: %q", ErrUnknownResource, p.Resource)
	}
	only, special := specialVerbs[p.Verb]
	if !special && !standardVerbs[p.Verb] {
		return fmt.Errorf("%w: %q", ErrUnknownVerb, p.Verb)
	}
	if special && p.Resource != only {
		return fmt.Errorf("%w: %s (%s applies only to %s)", ErrVerbNotAllowed, p, p.Verb, only)
	}
	return nil
}

// Verb is an action on a resource.
type Verb string

// Verbs, from docs/roles.md.
const (
	Read   Verb = "Read"
	Create Verb = "Create"
	Patch  Verb = "Patch"
	Delete Verb = "Delete"
	// Answer applies only to PermissionPrompts ("PermissionPrompts: Answer").
	Answer Verb = "Answer"
	// Run applies only to Workflows ("Workflows: Run"). The verb list in docs/roles.md
	// omits it, but its Contributor role grants it and its Workflow Manager
	// role depends on it.
	Run Verb = "Run"
)

var standardVerbs = map[Verb]bool{Read: true, Create: true, Patch: true, Delete: true}

// specialVerbs maps each verb that applies to one resource only to that
// resource.
var specialVerbs = map[Verb]Resource{
	Answer: PermissionPrompts,
	Run:    Workflows,
}

// Resource is a kind of object a permission applies to.
type Resource string

// Resources, one per resource docs/roles.md names. The comment gives the
// line of docs/roles.md it first appears on.
const (
	Projects           Resource = "Projects"           // :33
	AgentConversations Resource = "AgentConversations" // :34
	AgentMessages      Resource = "AgentMessages"      // :36
	PermissionPrompts  Resource = "PermissionPrompts"  // :39
	Requests           Resource = "Requests"           // :40
	ProjectFiles       Resource = "ProjectFiles"       // :42
	Workflows          Resource = "Workflows"          // :46
	WorkflowSteps      Resource = "WorkflowSteps"      // :48
	Secrets            Resource = "Secrets"            // :52
	UserInvitation     Resource = "UserInvitation"     // :58
	WorkflowLink       Resource = "WorkflowLink"       // :72
	Users              Resource = "Users"              // :83
	AccessReviews      Resource = "AccessReviews"      // :87
	Providers          Resource = "Providers"          // :88
)

var knownResources = map[Resource]bool{
	Projects: true, AgentConversations: true, AgentMessages: true,
	PermissionPrompts: true, Requests: true, ProjectFiles: true,
	Workflows: true, WorkflowSteps: true, Secrets: true,
	UserInvitation: true, WorkflowLink: true, Users: true,
	AccessReviews: true, Providers: true,
}
