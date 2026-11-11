package server

// Core registers no resources: an app and its features do. These tests use
// three, one with an extra verb each, the way an app would.
const (
	Projects          Resource = "Projects"
	Workflows         Resource = "Workflows"
	PermissionPrompts Resource = "PermissionPrompts"

	Answer Verb = "Answer"
	Run    Verb = "Run"
)

func testResources() *Resources {
	rs := NewResources()
	for _, def := range []ResourceDef{
		{Name: Projects},
		{Name: Workflows, Verbs: []Verb{Run}},
		{Name: PermissionPrompts, Verbs: []Verb{Answer}},
	} {
		if err := rs.Register(def); err != nil {
			panic(err)
		}
	}
	return rs
}

// testRouter is NewRouter with the three test resources registered.
func testRouter(title, version string) *Router {
	return NewRouter(title, version).WithResources(testResources())
}
