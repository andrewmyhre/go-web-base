# Roles

> This documents the role graph in `roles.go` at the project root, which
> `cmd/<slug>/main.go` passes to core as `app.Options.Roles`. Core ships no
> roles of its own, so edit both together: `TestRolesMatchDocs` in
> `roles_test.go` fails until they agree. The test parses this file: keep
> each role as a `## Name (scope)` heading, an optional
> `Everything a X and a Y has` line, and `- Resource: Verb` bullets.
> Optional feature modules add permissions to these roles at startup; those
> grants are not listed here, they are in each feature's own docs.

Roles are cumulative: each role has every permission of the role below it, plus the ones listed under it. In Casbin each role inherits from the one below it, so a permission is defined once.

Verbs are `Read`, `Create`, `Patch` and `Delete`, with two exceptions that each apply to one resource only: `PermissionPrompts: Answer` and `Workflows: Run`. A resource and its extra verbs are registered in `Resources()` in `roles.go`.

No role can read secret values - this is intentional. Secrets can be written and deleted, but their values only ever reach agents.

For owned vs project: project scope is Casbin domains, and "owned" is a matcher condition that the object's owner is the caller.

Roles form a graph, not a ladder. Some roles are narrow on purpose, so one responsibility can be delegated without granting everything above it. A user's permissions are the union of all their role bindings, so a Contributor who is also bound to Secrets Manager in the same project can manage that project's secrets. Inheritance:

```
Contributor  Secrets Manager
     \        /
   Project Manager     Workflow Manager
          |            /
       Manager        /
          |          /
         Admin ------
```

A global-scope binding applies in every project. A project-scope binding applies only in the project it names.

## Contributor (project scope)
- Projects: Read
- AgentConversations: Read
- AgentConversations: Create
- AgentMessages: Read
- AgentMessages: Create
- AgentMessages: Patch
- PermissionPrompts: Answer
- Requests: Read
- Requests: Patch
- ProjectFiles: Read
- ProjectFiles: Create
- ProjectFiles: Patch
- ProjectFiles: Delete
- Workflows: Read
- Workflows: Run
- WorkflowSteps: Read

## Secrets Manager (project scope)
A narrow role. Bind it on its own to delegate secret management for one project.
- Secrets: Create
- Secrets: Patch
- Secrets: Delete

## Project Manager (project scope)
Everything a Contributor and a Secrets Manager have, plus:
- UserInvitation: Create
- UserInvitation: Patch
- UserInvitation: Delete

## Workflow Manager (global scope)
A narrow role. Workflows are global, so this role is too. Running a workflow inside a project still needs `Workflows: Run` and `AgentMessages: Create` in that project, so this role does not give access to any project's agents.
- Workflows: Read
- Workflows: Create
- Workflows: Patch
- Workflows: Delete
- WorkflowSteps: Read
- WorkflowSteps: Create
- WorkflowSteps: Patch
- WorkflowSteps: Delete
- WorkflowLink: Create
- WorkflowLink: Delete

## Manager (global scope)
Everything a Project Manager has, in every project, plus:
- Projects: Create
- Projects: Patch
- Projects: Delete

## Admin (global scope)
Everything a Manager, Secrets Manager and Workflow Manager has, plus:
- Users: Read
- Users: Create
- Users: Patch
- Users: Delete
- AccessReviews: Create
- Providers: Read
- Providers: Create
- Providers: Patch
- Providers: Delete

