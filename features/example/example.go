// Package example is the smallest feature module that uses every hook of
// core's feature contract (app.Feature): a route, a resource and its grant, a
// migration, a setting and a log scope. Copy it to start a real feature.
//
// An app installs it by naming the role that may read the greeting:
//
//	app.Options{
//		Roles:    roles, // must define "Contributor"
//		Features: []app.Feature{example.New("Contributor")},
//	}
//
// It then serves GET /example/hello, which returns the configured greeting to
// anyone holding ExampleGreetings: Read, creates the table example_greetings,
// and reads <PREFIX>_EXAMPLE_GREETING (or --example-greeting).
package example

import (
	"context"
	"embed"
	"io/fs"
	"net/http"

	"go.uber.org/zap"

	"github.com/andrewmyhre/go-web-base/core/app"
	"github.com/andrewmyhre/go-web-base/core/server"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Name is the feature's name: its log scope, its migration table
// (goose_example_version) and its settings prefix.
const Name = "example"

// Greetings is the resource the route is guarded by.
const Greetings server.Resource = "ExampleGreetings"

// HelloPath is the route the feature serves.
const HelloPath = "/example/hello"

// DefaultGreeting is the greeting when the setting is not set.
const DefaultGreeting = "Hello from the example feature"

// Feature is the example feature. The zero value is not usable: build one
// with New.
type Feature struct {
	role string
}

// New returns the feature, granting ExampleGreetings: Read to role, a role
// of the app's role graph (and so to every role that inherits it).
func New(role string) *Feature { return &Feature{role: role} }

// Name implements app.Feature.
func (*Feature) Name() string { return Name }

// HelloOutput is the response of GET /example/hello.
type HelloOutput struct {
	Body struct {
		Greeting string `json:"greeting" example:"Hello from the example feature" doc:"The configured greeting."`
	}
}

// Install implements app.Feature. It declares everything the feature
// contributes; nothing here reads a setting or touches the router, because
// Install runs before the command line is parsed.
func (f *Feature) Install(i *app.Installer) error {
	if err := i.Resource(Greetings); err != nil {
		return err
	}
	if err := i.Grant(f.role, server.Permission{Resource: Greetings, Verb: server.Read}); err != nil {
		return err
	}
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	if err := i.Migrations(migrations); err != nil {
		return err
	}
	if err := i.String("greeting", DefaultGreeting, "greeting returned by GET "+HelloPath); err != nil {
		return err
	}
	settings := i.Settings()
	i.Routes(func(r *server.Router, d app.Deps) error {
		// Routes runs after the configuration is resolved, so the setting is
		// read here, once.
		greeting := settings.String("greeting")
		log := i.Log(d)
		return server.Register(r, server.Operation{
			OperationID: "get-example-hello",
			Method:      http.MethodGet,
			Path:        HelloPath,
			Summary:     "Return the example greeting",
			Tags:        []string{Name},
			Permission:  server.Permission{Resource: Greetings, Verb: server.Read},
		}, func(context.Context, *struct{}) (*HelloOutput, error) {
			log.Debug("hello", zap.String("greeting", greeting))
			out := &HelloOutput{}
			out.Body.Greeting = greeting
			return out, nil
		})
	})
	return nil
}
