package app

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// fakeFeature is a Feature built from a function.
type fakeFeature struct {
	name    string
	install func(i *Installer) error
}

func (f fakeFeature) Name() string { return f.name }
func (f fakeFeature) Install(i *Installer) error {
	if f.install == nil {
		return nil
	}
	return f.install(i)
}

func withFeatures(fs ...Feature) Options {
	o := testOpts
	o.Features = fs
	return o
}

func TestFeatureNamesAreChecked(t *testing.T) {
	for name, fs := range map[string][]Feature{
		"empty":               {fakeFeature{name: ""}},
		"upper case":          {fakeFeature{name: "Example"}},
		"has an underscore":   {fakeFeature{name: "my_feature"}},
		"starts with a digit": {fakeFeature{name: "1x"}},
		"reserved app":        {fakeFeature{name: "app"}},
		"reserved core":       {fakeFeature{name: "core"}},
		"twice":               {fakeFeature{name: "a"}, fakeFeature{name: "a"}},
		"nil":                 {nil},
	} {
		if _, err := newInstallation(withFeatures(fs...)); err == nil {
			t.Errorf("%s: newInstallation accepted the features", name)
		}
	}
}

func TestInstallErrorIsReturnedByEverySubcommand(t *testing.T) {
	boom := errors.New("boom")
	o := withFeatures(fakeFeature{name: "bad", install: func(*Installer) error { return boom }})
	root := NewRootCmd(o)
	for _, sub := range []string{"serve", "openapi"} {
		root.SetArgs([]string{sub})
		root.SilenceErrors = true
		if err := root.Execute(); !errors.Is(err, boom) {
			t.Errorf("%s: err = %v, want the Install error", sub, err)
		}
	}
	if _, err := OpenAPISpec(o); !errors.Is(err, boom) {
		t.Errorf("OpenAPISpec err = %v, want the Install error", err)
	}
}

// TestSettingsBecomeFlagsAndEnv: a declared setting is a flag of the serve
// command before parsing, and resolves flag over env over default.
func TestSettingsBecomeFlagsAndEnv(t *testing.T) {
	var settings *Settings
	f := fakeFeature{name: "demo", install: func(i *Installer) error {
		settings = i.Settings()
		for _, err := range []error{
			i.String("greeting", "hello", "what to say"),
			i.Int("max_items", 3, "how many"),
			i.Bool("loud", false, "shout"),
			i.Secret("token"),
		} {
			if err != nil {
				return err
			}
		}
		return nil
	}}
	t.Setenv("TESTAPP_DEMO_TOKEN", "s3cret")
	t.Setenv("TESTAPP_DEMO_MAX_ITEMS", "7")
	root := NewRootCmd(withFeatures(f))
	serve, _, err := root.Find([]string{"serve"})
	if err != nil {
		t.Fatal(err)
	}
	fl := serve.Flags().Lookup("demo-greeting")
	if fl == nil {
		t.Fatal("serve has no --demo-greeting flag before parsing")
	}
	if !strings.Contains(fl.Usage, "what to say") || !strings.Contains(fl.Usage, "TESTAPP_DEMO_GREETING") {
		t.Errorf("--demo-greeting usage = %q, want the help and the env var", fl.Usage)
	}
	if serve.Flags().Lookup("demo-max-items") == nil || serve.Flags().Lookup("demo-loud") == nil {
		t.Error("missing --demo-max-items or --demo-loud")
	}
	if serve.Flags().Lookup("demo-token") != nil {
		t.Error("a secret has a flag, but must be env only")
	}

	// Defaults and env before parsing a flag.
	if got := settings.String("greeting"); got != "hello" {
		t.Errorf("greeting = %q, want the default", got)
	}
	if got := settings.Int("max_items"); got != 7 {
		t.Errorf("max_items = %d, want 7 from the env", got)
	}
	if got := settings.String("token"); got != "s3cret" {
		t.Errorf("token = %q, want the env value", got)
	}
	// A flag wins over the env.
	t.Setenv("TESTAPP_DEMO_GREETING", "from env")
	if got := settings.String("greeting"); got != "from env" {
		t.Errorf("greeting = %q, want the env value", got)
	}
	if err := serve.Flags().Parse([]string{"--demo-greeting=from flag", "--demo-loud", "--demo-max-items=9"}); err != nil {
		t.Fatal(err)
	}
	if settings.String("greeting") != "from flag" || !settings.Bool("loud") || settings.Int("max_items") != 9 {
		t.Errorf("after parsing: greeting=%q loud=%v max_items=%d", settings.String("greeting"), settings.Bool("loud"), settings.Int("max_items"))
	}
}

func TestReadingAnUndeclaredSettingPanics(t *testing.T) {
	var s *Settings
	_ = mustInstall(t, withFeatures(fakeFeature{name: "demo", install: func(i *Installer) error {
		s = i.Settings()
		return nil
	}}))
	defer func() {
		if recover() == nil {
			t.Error("reading an undeclared setting did not panic")
		}
	}()
	_ = s.String("typo")
}

func TestSettingClashesAreRefused(t *testing.T) {
	// "redis" + "addr" is core's own redis_addr.
	_, err := newInstallation(withFeatures(fakeFeature{name: "redis", install: func(i *Installer) error {
		return i.String("addr", "", "")
	}}))
	if err == nil {
		t.Error("a setting that shadows core's redis_addr was accepted")
	}
	_, err = newInstallation(withFeatures(fakeFeature{name: "demo", install: func(i *Installer) error {
		if err := i.String("x", "", ""); err != nil {
			return err
		}
		return i.Bool("x", false, "")
	}}))
	if err == nil {
		t.Error("a setting declared twice was accepted")
	}
	_, err = newInstallation(withFeatures(fakeFeature{name: "demo", install: func(i *Installer) error {
		return i.String("Bad-Key", "", "")
	}}))
	if err == nil {
		t.Error("a malformed setting key was accepted")
	}
}

func TestResourcesAndGrantsAreChecked(t *testing.T) {
	feature := func(install func(i *Installer) error) Options {
		return withFeatures(fakeFeature{name: "demo", install: install})
	}
	read := server.Permission{Resource: "Widgets", Verb: server.Read}

	// A feature resource granted to an app role works, and shows in the
	// assembled graph without touching the app's Options.Roles.
	o := feature(func(i *Installer) error {
		if err := i.Resource("Widgets", "Spin"); err != nil {
			return err
		}
		return i.Grant(db.AdminRole, read, server.Permission{Resource: "Widgets", Verb: "Spin"})
	})
	inst, err := newInstallation(o)
	if err != nil {
		t.Fatalf("newInstallation: %v", err)
	}
	perms := db.RolePermissions(inst.roles)[db.AdminRole]
	if !contains(perms, "Widgets: Read") || !contains(perms, "Widgets: Spin") {
		t.Errorf("Admin permissions = %v, want the feature's grants", perms)
	}
	if len(o.Roles[0].Permissions) != 1 {
		t.Errorf("Options.Roles was modified: %v", o.Roles)
	}

	for name, install := range map[string]func(*Installer) error{
		"grant of an unregistered resource": func(i *Installer) error { return i.Grant(db.AdminRole, read) },
		"grant to a role the app lacks": func(i *Installer) error {
			return errors.Join(i.Resource("Widgets"), i.Grant("Ghost", read))
		},
		"grant of a verb the resource lacks": func(i *Installer) error {
			return errors.Join(i.Resource("Widgets"), i.Grant(db.AdminRole, server.Permission{Resource: "Widgets", Verb: "Spin"}))
		},
		"grant of Public": func(i *Installer) error { return i.Grant(db.AdminRole, server.Public) },
		"resource the app already has": func(i *Installer) error {
			return i.Resource("Projects")
		},
		"bad resource name": func(i *Installer) error { return i.Resource("Not Valid") },
	} {
		if _, err := newInstallation(feature(install)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// An app role naming a resource nobody registered is refused at startup.
func TestAppRoleGraphMustNameRegisteredResources(t *testing.T) {
	o := testOpts
	o.Roles = []db.RoleDef{{Name: db.AdminRole, Permissions: []string{"Nowhere: Read"}}}
	if _, err := newInstallation(o); !errors.Is(err, server.ErrUnknownResource) {
		t.Errorf("err = %v, want ErrUnknownResource", err)
	}
	o.Roles = []db.RoleDef{{Name: db.AdminRole, Permissions: []string{"Projects: Fly"}}}
	if _, err := newInstallation(o); !errors.Is(err, server.ErrUnknownVerb) {
		t.Errorf("err = %v, want ErrUnknownVerb", err)
	}
	o.Roles = []db.RoleDef{{Name: "Viewer", Permissions: []string{"Projects: Read"}}}
	if _, err := newInstallation(o); !errors.Is(err, db.ErrBadRoles) {
		t.Errorf("err = %v, want ErrBadRoles (no Admin)", err)
	}
}

// A route whose permission names a resource nobody registered cannot be
// served: Build refuses it.
func TestFeatureRouteWithUnregisteredResourceFailsBuild(t *testing.T) {
	f := fakeFeature{name: "demo", install: func(i *Installer) error {
		i.Routes(func(r *server.Router, _ Deps) error {
			return server.Register(r, server.Operation{
				OperationID: "get-x", Method: http.MethodGet, Path: "/x",
				Permission: server.Permission{Resource: "Unregistered", Verb: server.Read},
			}, health)
		})
		return nil
	}}
	if _, err := OpenAPISpec(withFeatures(f)); !errors.Is(err, server.ErrUnknownResource) {
		t.Errorf("OpenAPISpec err = %v, want ErrUnknownResource", err)
	}
}

func TestRouteOrderIsBuiltinsFeaturesThenApp(t *testing.T) {
	var order []string
	mk := func(name string) Feature {
		return fakeFeature{name: name, install: func(i *Installer) error {
			i.Routes(func(*server.Router, Deps) error { order = append(order, name); return nil })
			return nil
		}}
	}
	o := withFeatures(mk("one"), mk("two"))
	o.Routes = func(*server.Router, Deps) error { order = append(order, "app"); return nil }
	if _, err := testRouter(t, o, Deps{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "one,two,app" {
		t.Errorf("route hook order = %s, want one,two,app", got)
	}
}

func TestMigrationOrderAndTables(t *testing.T) {
	fsys := fstest.MapFS{"00001_x.sql": {Data: []byte("-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n")}}
	mk := func(name string) Feature {
		return fakeFeature{name: name, install: func(i *Installer) error { return i.Migrations(fsys) }}
	}
	o := withFeatures(mk("b"), mk("a"))
	o.Migrations = fsys
	inst := mustInstall(t, o)
	var got []string
	for _, m := range inst.migrations {
		got = append(got, m.Name+"="+m.Table)
	}
	want := "b=goose_b_version,a=goose_a_version,app=goose_app_version"
	if strings.Join(got, ",") != want {
		t.Errorf("migrations = %v, want %s (features in order, then the app; core is applied first by db.Migrate)", got, want)
	}
	if db.CoreTable != "goose_db_version" {
		t.Errorf("core table = %q: changing it would re-run core's migrations on existing databases", db.CoreTable)
	}

	twice := fakeFeature{name: "c", install: func(i *Installer) error {
		return errors.Join(i.Migrations(fsys), i.Migrations(fsys))
	}}
	if _, err := newInstallation(withFeatures(twice)); err == nil {
		t.Error("Migrations called twice was accepted")
	}
	nilFS := fakeFeature{name: "d", install: func(i *Installer) error { return i.Migrations(nil) }}
	if _, err := newInstallation(withFeatures(nilFS)); err == nil {
		t.Error("Migrations(nil) was accepted")
	}
}

func TestFeatureLogScope(t *testing.T) {
	var inst *Installer
	mustInstall(t, withFeatures(fakeFeature{name: "demo", install: func(i *Installer) error { inst = i; return nil }}))
	core, logs := observer.New(zap.InfoLevel)
	inst.Log(Deps{Log: zap.New(core)}).Info("hello")
	entries := logs.All()
	if len(entries) != 1 || entries[0].ContextMap()["scope"] != "demo" {
		t.Errorf("log entries = %v, want one with scope=demo", entries)
	}
	inst.Log(Deps{}).Info("no logger: must not panic")
}

// TestOpenAPIIncludesFeatureRoutes: the feature's route is in the document
// the openapi command prints, and its resource is usable.
func TestOpenAPIIncludesFeatureRoutes(t *testing.T) {
	f := fakeFeature{name: "demo", install: func(i *Installer) error {
		if err := i.Resource("Widgets"); err != nil {
			return err
		}
		i.Routes(func(r *server.Router, _ Deps) error {
			return server.Register(r, server.Operation{
				OperationID: "get-widgets", Method: http.MethodGet, Path: "/widgets",
				Permission: server.Permission{Resource: "Widgets", Verb: server.Read},
			}, health)
		})
		return nil
	}}
	spec, err := OpenAPISpec(withFeatures(f))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(spec), `"get-widgets"`) {
		t.Errorf("spec lacks get-widgets: %s", spec)
	}
}
