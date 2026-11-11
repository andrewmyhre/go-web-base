package db_test

import (
	"strings"
	"testing"

	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// TestRolePermissionsAreServerPermissions proves every permission a role
// grants is one a route can declare, so a route's server.Permission and the
// enforcer's policy use the same strings. It is an external test package so
// server may later import db without a cycle.
func TestRolePermissionsAreServerPermissions(t *testing.T) {
	for role, perms := range db.RolePermissions() {
		if len(perms) == 0 {
			t.Errorf("role %q grants nothing", role)
		}
		for _, p := range perms {
			res, verb, ok := strings.Cut(p, ": ")
			if !ok {
				t.Errorf("%s: malformed permission %q", role, p)
				continue
			}
			perm := server.Permission{Resource: server.Resource(res), Verb: server.Verb(verb)}
			if err := perm.Validate(); err != nil {
				t.Errorf("%s: %s: %v", role, p, err)
			}
		}
	}
}
