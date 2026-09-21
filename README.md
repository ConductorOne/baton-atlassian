![Baton Logo](./baton-logo.png)

# `baton-atlassian` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-atlassian.svg)](https://pkg.go.dev/github.com/conductorone/baton-atlassian) ![verify](https://github.com/conductorone/baton-atlassian/actions/workflows/verify.yaml/badge.svg)

`baton-atlassian` is a connector for [Atlassian](https://www.atlassian.com) built using the [Baton SDK](https://github.com/conductorone/baton-sdk).
This connector is intended to use for managing general aspects of an Atlassian Organization, not specifically limited to any product or site.
People will be able to provision Roles for the users on different Workspaces (product-site) if they have the Atlassian feature enabled for their organization. Users can also be provisioned into and out of groups (group membership).

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Prerequisites


## 1. Create an API Key

1. Log into your Atlassian account (requires **Org admin** role)
2. Navigate to **Organization Settings > API keys**
3. Click **Create API key**
4. **Select "API key without scopes"** and set a name and expiration date

> **Important:** You must use "API key without scopes" because most endpoints used by this connector (users, groups, role-assignments) do not have scopes available yet. See [Atlassian Admin API documentation](https://support.atlassian.com/organization-administration/docs/manage-an-organization-with-the-admin-apis/) and [Organizations REST API Reference](https://developer.atlassian.com/cloud/admin/organization/rest/intro/).

## 2. Get the Organization ID

The Organization ID can be found in the URL when logged into Atlassian Admin:
- `https://admin.atlassian.com/o/{organizationId}/`

# Getting Started

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-atlassian
baton-atlassian --access-token ACCESS_TOKEN --organization-id ORG_ID
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_DOMAIN_URL=domain_url -e BATON_API_KEY=apiKey -e BATON_USERNAME=username public.ecr.aws/conductorone/baton-atlassian:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-atlassian/cmd/baton-atlassian@main

baton-atlassian --access-token ACCESS_TOKEN --organization-id ORG_ID

baton resources
```

# Data Model

`baton-atlassian` will pull down information about the following resources:

| Resource Type | Description |
|---------------|-------------|
| **Organization** | The Atlassian organization with org-level roles (org-admin, site-admin, user-access-admin) |
| **Users** | All users in the organization directory |
| **Groups** | All groups in the organization with their memberships (e.g., org-admins, jira-admins, jira-users) |
| **Workspaces** | Product-sites (Jira, Jira Software, Confluence, Rovo, etc.) |

## Roles and Grants

The connector syncs the following role assignments:

### Organization-level Roles (Platform Roles)

These roles are synced **directly from users only**.

| Role | Description |
|------|-------------|
| `atlassian/org-admin` | Organization administrator with full access |
| `atlassian/site-admin` | Site administrator |
| `atlassian/user-access-admin` | User access administrator |
| `atlassian/ai-access` | Atlassian Intelligence access (Premium/Enterprise only) |

### Workspace-level Roles

These roles can be assigned to **users and groups**.

| Role | Description |
|------|-------------|
| `atlassian/user` | Basic user access to the workspace |
| `atlassian/admin` | Administrator access to the workspace |
| `atlassian/guest` | Guest access |
| `atlassian/contributor` | Contributor access |
| `atlassian/customer` | Customer access |
| `atlassian/basic` | Basic access |
| `atlassian/stakeholder` | Stakeholder access |

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually
building spreadsheets. We welcome contributions, and ideas, no matter how
small&mdash;our goal is to make identity and permissions sprawl less painful for
everyone. If you have questions, problems, or ideas: Please open a GitHub Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-atlassian` Command Line Usage

```
baton-atlassian

Usage:
  baton-atlassian [flags]
  baton-atlassian [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --access-token string          required: The API access token for your Atlassian organization ($BATON_API_TOKEN)
      --client-id string             The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string         The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
  -f, --file string                  The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                         help for baton-atlassian
      --log-format string            The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string             The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --organization-id string       required: ID of the Atlassian Organization that will be synced ($BATON_ORG)
  -p, --provisioning                 If this connector supports provisioning, this must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --ticketing                    This must be set to enable ticketing support ($BATON_TICKETING)
  -v, --version                      version for baton-atlassian

Use "baton-atlassian [command] --help" for more information about a command.
```
