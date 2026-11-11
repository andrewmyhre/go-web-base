package app

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/andrewmyhre/go-web-base/core/config"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/logging"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// A Feature is an optional module an app installs through Options.Features:
// a Go module of its own that adds routes, resources and role grants,
// database migrations, settings and a log scope to the app, and nothing else.
// It has no other way into the program, so everything it adds is declared,
// checked and visible in one place.
//
// Install runs once per command tree, when app.NewRootCmd builds it, before
// any flag is parsed. That is why settings are declared in Install: the
// flags must exist before cobra parses. Install therefore only declares. It
// must not read a setting, open a connection or touch the router; work that
// needs those goes in the function passed to Installer.Routes, which runs at
// startup after the configuration is resolved. Install may run again for
// app.OpenAPISpec, so it must keep no state outside the Installer.
type Feature interface {
	// Name is a lower-case identifier (letters and digits, starting with a
	// letter). It is the feature's log scope, the owner of its migration
	// table (goose_<name>_version) and the prefix of its settings
	// (<PREFIX>_<NAME>_<KEY>). "app" and "core" are reserved.
	Name() string
	// Install declares what the feature contributes.
	Install(i *Installer) error
}

var featureName = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
var settingKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// reservedNames are module names a feature may not take: the migration
// tables of core and the app.
var reservedNames = []string{"app", "core"}

// Installer is what a feature's Install is given. Every method belongs to
// one feature; an Installer is not shared between features.
type Installer struct {
	inst *installation
	name string

	settings   *Settings
	migrations bool
}

// Name is the feature's name.
func (i *Installer) Name() string { return i.name }

// Resource registers a resource the feature's routes (or any role grant)
// may name, with the verbs it has beyond the standard Read, Create, Patch and
// Delete. Resource names are shared by the whole app: registering one that
// the app or another feature already registered is an error, so prefix
// them with the feature when they could clash.
func (i *Installer) Resource(name server.Resource, verbs ...server.Verb) error {
	def := server.ResourceDef{Name: name, Verbs: verbs}
	if err := i.inst.registerResource(def); err != nil {
		return fmt.Errorf("feature %s: %w", i.name, err)
	}
	return nil
}

// Grant adds permissions to a role of the app's role graph (Options.Roles),
// for example the example feature's read permission to "Contributor". The
// app decides which roles exist; a grant to a role it does not have, or of a
// permission whose resource nobody registered, fails startup. A role also
// holds whatever it inherits, so granting to a role grants it to the roles
// that inherit it.
func (i *Installer) Grant(role string, perms ...server.Permission) error {
	for _, p := range perms {
		if p.IsPublic() || p.IsAuthenticated() || p == (server.Permission{}) {
			return fmt.Errorf("feature %s: cannot grant %s to %q: only a resource and verb can be granted", i.name, p, role)
		}
		i.inst.grants = append(i.inst.grants, grant{feature: i.name, role: role, perm: p.String()})
	}
	return nil
}

// Migrations contributes the feature's goose migrations: fsys holds the *.sql
// files at its root (use fs.Sub on an embed.FS). They are applied at
// startup after core's and the earlier features', each module in its own
// version table, goose_<name>_version. Call it at most once.
func (i *Installer) Migrations(fsys fs.FS) error {
	if fsys == nil {
		return fmt.Errorf("feature %s: Migrations(nil)", i.name)
	}
	if i.migrations {
		return fmt.Errorf("feature %s: Migrations called twice", i.name)
	}
	i.migrations = true
	i.inst.migrations = append(i.inst.migrations, db.Migration{Name: i.name, Table: db.ModuleTable(i.name), FS: fsys})
	return nil
}

// Routes contributes the feature's routes. fn runs when the router is built
// (at startup, and for the openapi command with stand-in stores), after
// core's built-in routes and before the app's own, with the settings
// resolved. Every route must go through server.Register or
// server.RegisterHandler with a declared permission, as for the app. fn must
// only register routes.
func (i *Installer) Routes(fn func(r *server.Router, d Deps) error) {
	i.inst.routes = append(i.inst.routes, routeHook{feature: i.name, fn: fn})
}

// String declares a string setting. key is lower case with underscores. It
// becomes the flag --<name>-<key> (underscores as hyphens) on the serve
// command and the variable <PREFIX>_<NAME>_<KEY>, and a flag set on the
// command line wins over the environment, which wins over def.
func (i *Installer) String(key, def, help string) error {
	return i.declare(key, def, help, func(fl *pflag.FlagSet, flag string) { fl.String(flag, def, help) })
}

// Int declares an integer setting; see String.
func (i *Installer) Int(key string, def int, help string) error {
	return i.declare(key, def, help, func(fl *pflag.FlagSet, flag string) { fl.Int(flag, def, help) })
}

// Bool declares a boolean setting; see String.
func (i *Installer) Bool(key string, def bool, help string) error {
	return i.declare(key, def, help, func(fl *pflag.FlagSet, flag string) { fl.Bool(flag, def, help) })
}

// Secret declares a setting that is read from <PREFIX>_<NAME>_<KEY> only. It
// has no flag, so it does not appear in process listings or pod args, like
// the Google client secret. Read it with Settings().String.
func (i *Installer) Secret(key string) error {
	return i.declare(key, "", "", nil)
}

// Settings reads the feature's settings. Its getters return the resolved
// values, so call them inside the function passed to Routes, not in Install.
func (i *Installer) Settings() *Settings { return i.settings }

// Log returns a logger for the feature's own log scope: every line carries
// scope=<name> next to the service field. d is the Deps the Routes function
// received; without a logger (the openapi command) the result logs nothing.
func (i *Installer) Log(d Deps) *zap.Logger {
	if d.Log == nil {
		return zap.NewNop()
	}
	return logging.Scoped(d.Log, i.name)
}

func (i *Installer) declare(key string, def any, help string, flag func(*pflag.FlagSet, string)) error {
	if !settingKey.MatchString(key) {
		return fmt.Errorf("feature %s: setting key %q must be lower case letters, digits and underscores, starting with a letter", i.name, key)
	}
	vkey := i.name + "_" + key
	if slices.Contains(i.inst.v.AllKeys(), vkey) {
		return fmt.Errorf("feature %s: setting %q clashes with an existing setting", i.name, key)
	}
	i.inst.v.SetDefault(vkey, def)
	i.settings.declared[key] = true
	if flag != nil {
		name := strings.ReplaceAll(vkey, "_", "-")
		env := i.inst.o.EnvPrefix + "_" + strings.ToUpper(vkey)
		i.inst.flags = append(i.inst.flags, flagDef{
			key:  vkey,
			name: name,
			add:  func(fl *pflag.FlagSet) { flag(fl, name); fl.Lookup(name).Usage = help + " (env " + env + ")" },
		})
	}
	return nil
}

// Settings gives a feature its resolved configuration. Keys are the ones the
// feature declared, without the feature's prefix.
type Settings struct {
	feature  string
	v        *viper.Viper
	declared map[string]bool
}

func (s *Settings) key(k string) string {
	if !s.declared[k] {
		panic(fmt.Sprintf("feature %s reads setting %q it never declared", s.feature, k))
	}
	return s.feature + "_" + k
}

// String returns a string (or secret) setting.
func (s *Settings) String(key string) string { return strings.TrimSpace(s.v.GetString(s.key(key))) }

// Int returns an integer setting.
func (s *Settings) Int(key string) int { return s.v.GetInt(s.key(key)) }

// Bool returns a boolean setting.
func (s *Settings) Bool(key string) bool { return s.v.GetBool(s.key(key)) }

type flagDef struct {
	key, name string
	add       func(*pflag.FlagSet)
}

type routeHook struct {
	feature string
	fn      func(r *server.Router, d Deps) error
}

type grant struct {
	feature, role, perm string
}

// installation is everything the app and its features contributed, collected
// by newInstallation before anything starts.
type installation struct {
	o Options
	v *viper.Viper

	resourceDefs []server.ResourceDef
	resources    *server.Resources // for validation; routers get a fresh one
	grants       []grant
	roles        []db.RoleDef // Options.Roles plus the grants
	migrations   []db.Migration
	routes       []routeHook
	flags        []flagDef
}

func (n *installation) registerResource(def server.ResourceDef) error {
	if err := n.resources.Register(def); err != nil {
		return err
	}
	n.resourceDefs = append(n.resourceDefs, def)
	return nil
}

// newRouterResources returns a registry holding every contributed resource.
func (n *installation) newRouterResources() *server.Resources {
	rs := server.NewResources()
	for _, def := range n.resourceDefs {
		if err := rs.Register(def); err != nil {
			panic(err) // the same defs registered once already
		}
	}
	return rs
}

// newInstallation validates o, runs every feature's Install and checks the
// result as a whole: the role graph, every grant and every name.
func newInstallation(o Options) (*installation, error) {
	o, err := o.validate()
	if err != nil {
		return nil, err
	}
	n := &installation{o: o, v: config.NewViper(o.EnvPrefix), resources: server.NewResources()}
	for _, def := range o.Resources {
		if err := n.registerResource(def); err != nil {
			return nil, fmt.Errorf("app resources: %w", err)
		}
	}
	names := map[string]bool{}
	for _, f := range o.Features {
		if f == nil {
			return nil, errors.New("app.Options.Features has a nil Feature")
		}
		name := f.Name()
		if !featureName.MatchString(name) {
			return nil, fmt.Errorf("feature name %q must be lower case letters and digits, starting with a letter", name)
		}
		if slices.Contains(reservedNames, name) {
			return nil, fmt.Errorf("feature name %q is reserved", name)
		}
		if names[name] {
			return nil, fmt.Errorf("feature %q is installed twice", name)
		}
		names[name] = true
		i := &Installer{inst: n, name: name, settings: &Settings{feature: name, v: n.v, declared: map[string]bool{}}}
		if err := f.Install(i); err != nil {
			return nil, fmt.Errorf("install feature %q: %w", name, err)
		}
	}
	if o.Migrations != nil {
		n.migrations = append(n.migrations, db.Migration{Name: "app", Table: db.ModuleTable("app"), FS: o.Migrations})
	}
	if err := n.assembleRoles(); err != nil {
		return nil, err
	}
	return n, nil
}

// assembleRoles adds every feature's grants to a copy of the app's role
// graph and checks the result: the structure, and that every permission it
// names is on a registered resource and verb.
func (n *installation) assembleRoles() error {
	roles := db.CloneRoles(n.o.Roles)
	for _, g := range n.grants {
		found := false
		for i := range roles {
			if roles[i].Name == g.role {
				roles[i].Permissions = append(roles[i].Permissions, g.perm)
				found = true
			}
		}
		if !found {
			return fmt.Errorf("feature %q grants %s to role %q, which app.Options.Roles does not define", g.feature, g.perm, g.role)
		}
	}
	if err := db.ValidateRoles(roles); err != nil {
		return err
	}
	for _, r := range roles {
		for _, p := range r.Permissions {
			if err := n.resources.CheckString(p); err != nil {
				return fmt.Errorf("role %q: %w", r.Name, err)
			}
		}
	}
	n.roles = roles
	return nil
}

// addFlags defines every feature's flags on fs and binds them to the
// installation's Viper.
func (n *installation) addFlags(fs *pflag.FlagSet) error {
	for _, f := range n.flags {
		f.add(fs)
		if err := n.v.BindPFlag(f.key, fs.Lookup(f.name)); err != nil {
			return fmt.Errorf("bind --%s: %w", f.name, err)
		}
	}
	return nil
}
