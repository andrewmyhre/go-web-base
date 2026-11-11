package server

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// A Permission is what a caller must hold to invoke a route: a verb on a
// resource, in the form docs/roles.md uses in its Roles section,
// e.g. "Projects: Read". The resources are not fixed in core: the app and
// its features register them in a Resources registry, and Router.Build
// refuses a permission that names one that was never registered. The
// exceptions are Public and Authenticated, for routes that need no role.
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
	// ErrUnknownResource is returned for a resource nobody registered, and
	// for a malformed Public or Authenticated permission.
	ErrUnknownResource = errors.New("unknown permission resource")
	// ErrUnknownVerb is returned for a verb that is neither standard nor
	// registered by any resource.
	ErrUnknownVerb = errors.New("unknown permission verb")
	// ErrVerbNotAllowed is returned for a verb used on a resource it does
	// not apply to: one resource registered it, and this is another.
	ErrVerbNotAllowed = errors.New("verb not allowed on resource")
	// ErrDuplicateResource is returned when a resource is registered twice.
	ErrDuplicateResource = errors.New("resource already registered")
	// ErrBadName is returned for a resource or verb name that is not a plain
	// identifier: letters and digits, starting with a letter.
	ErrBadName = errors.New("bad resource or verb name")
)

// Validate reports whether p is well formed: it declares something, and is
// not both Public and Authenticated. It does not know which resources exist;
// Resources.Check does, and Router.Build runs it for every route.
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
	return nil
}

// Verb is an action on a resource.
type Verb string

// The standard verbs, valid on every resource. A resource may register more
// of its own (ResourceDef.Verbs), for example "Run" on a workflows resource.
const (
	Read   Verb = "Read"
	Create Verb = "Create"
	Patch  Verb = "Patch"
	Delete Verb = "Delete"
)

var standardVerbs = []Verb{Read, Create, Patch, Delete}

// Resource is a kind of object a permission applies to.
type Resource string

// ResourceDef registers one resource: its name and the verbs it has beyond
// the four standard ones.
type ResourceDef struct {
	Name Resource
	// Verbs are extra verbs that apply to this resource, such as "Answer".
	// A verb two resources both list applies to both.
	Verbs []Verb
}

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// Resources is a registry of the resources a router accepts permissions
// for. Every router has one and the app and its features add to it, so
// there is no package-level list to mutate. It is not safe for concurrent
// use: all registration happens at startup.
type Resources struct {
	byName map[Resource]map[Verb]bool // each resource's extra verbs
	order  []Resource
	// extra is every verb some resource registered, to tell an unknown verb
	// from one used on the wrong resource.
	extra map[Verb]bool
}

// NewResources returns an empty registry.
func NewResources() *Resources {
	return &Resources{byName: map[Resource]map[Verb]bool{}, extra: map[Verb]bool{}}
}

// Register adds a resource. It is refused if the name or a verb is not a
// plain identifier, the resource is already registered, or an extra verb is
// one of the standard four.
func (rs *Resources) Register(def ResourceDef) error {
	if !identifier.MatchString(string(def.Name)) {
		return fmt.Errorf("%w: resource %q", ErrBadName, def.Name)
	}
	if _, dup := rs.byName[def.Name]; dup {
		return fmt.Errorf("%w: %q", ErrDuplicateResource, def.Name)
	}
	verbs := map[Verb]bool{}
	for _, v := range def.Verbs {
		if !identifier.MatchString(string(v)) {
			return fmt.Errorf("%w: verb %q on %s", ErrBadName, v, def.Name)
		}
		if slices.Contains(standardVerbs, v) {
			return fmt.Errorf("%w: %q on %s is already a standard verb", ErrBadName, v, def.Name)
		}
		verbs[v] = true
	}
	rs.byName[def.Name] = verbs
	rs.order = append(rs.order, def.Name)
	for v := range verbs {
		rs.extra[v] = true
	}
	return nil
}

// Names returns the registered resources, in registration order.
func (rs *Resources) Names() []Resource { return slices.Clone(rs.order) }

// Check reports whether p is a permission a route may declare: well formed,
// and, unless Public or Authenticated, naming a registered resource and a
// verb that applies to it.
func (rs *Resources) Check(p Permission) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.public || p.authenticated {
		return nil
	}
	verbs, ok := rs.byName[p.Resource]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownResource, p.Resource)
	}
	if slices.Contains(standardVerbs, p.Verb) || verbs[p.Verb] {
		return nil
	}
	if rs.extra[p.Verb] {
		return fmt.Errorf("%w: %s (%s does not apply to %s)", ErrVerbNotAllowed, p, p.Verb, p.Resource)
	}
	return fmt.Errorf("%w: %q", ErrUnknownVerb, p.Verb)
}

// CheckString is Check for a permission in the "Resource: Verb" form a role
// graph uses.
func (rs *Resources) CheckString(s string) error {
	res, verb, ok := strings.Cut(s, ": ")
	if !ok {
		return fmt.Errorf("%w: %q is not in the form \"Resource: Verb\"", ErrUnknownVerb, s)
	}
	return rs.Check(Permission{Resource: Resource(res), Verb: Verb(verb)})
}
