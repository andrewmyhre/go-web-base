package db

// exampleRoles is the six-role graph the tests run the enforcer over. Core
// ships no roles: this is the graph the template's roles.go carries, copied
// here so these tests do not depend on a generated project.
var exampleRoles = []RoleDef{
	{
		Name: "Contributor",
		Permissions: []string{
			"Projects: Read",
			"AgentConversations: Read", "AgentConversations: Create",
			"AgentMessages: Read", "AgentMessages: Create", "AgentMessages: Patch",
			"PermissionPrompts: Answer",
			"Requests: Read", "Requests: Patch",
			"ProjectFiles: Read", "ProjectFiles: Create", "ProjectFiles: Patch", "ProjectFiles: Delete",
			"Workflows: Read", "Workflows: Run",
			"WorkflowSteps: Read",
		},
	},
	{
		Name:        "Secrets Manager",
		Permissions: []string{"Secrets: Create", "Secrets: Patch", "Secrets: Delete"},
	},
	{
		Name:        "Project Manager",
		Inherits:    []string{"Contributor", "Secrets Manager"},
		Permissions: []string{"UserInvitation: Create", "UserInvitation: Patch", "UserInvitation: Delete"},
	},
	{
		Name: "Workflow Manager",
		Permissions: []string{
			"Workflows: Read", "Workflows: Create", "Workflows: Patch", "Workflows: Delete",
			"WorkflowSteps: Read", "WorkflowSteps: Create", "WorkflowSteps: Patch", "WorkflowSteps: Delete",
			"WorkflowLink: Create", "WorkflowLink: Delete",
		},
	},
	{
		Name:        "Manager",
		Inherits:    []string{"Project Manager"},
		Permissions: []string{"Projects: Create", "Projects: Patch", "Projects: Delete"},
	},
	{
		Name:     AdminRole,
		Inherits: []string{"Manager", "Secrets Manager", "Workflow Manager"},
		Permissions: []string{
			"Users: Read", "Users: Create", "Users: Patch", "Users: Delete",
			"AccessReviews: Create",
			"Providers: Read", "Providers: Create", "Providers: Patch", "Providers: Delete",
		},
	},
}
