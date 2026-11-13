![Baton Logo](./baton-logo.png)

# `baton-atlassian` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-atlassian.svg)](https://pkg.go.dev/github.com/conductorone/baton-atlassian) ![verify](https://github.com/conductorone/baton-atlassian/actions/workflows/verify.yaml/badge.svg)

`baton-atlassian` is a connector for [Atlassian](https://www.atlassian.com) built using the [Baton SDK](https://github.com/conductorone/baton-sdk).
This connector is intended to use for managing general aspects of an Atlassian Organization, not specifically limited to any product or site.
People will be able to provision Roles for the users on different Workspaces (product-site) if they have the Atlassian feature enabled for their organization. Users can also be provisioned into and out of groups (group membership), and managed accounts (on a verified domain) can be deprovisioned, which deletes the Atlassian account across every product after Atlassian's 14-day grace period. Connector actions can also suspend or restore a user's access to the organization (`disable_user`, `enable_user`), and deactivate or reactivate a managed account across every Atlassian product (`deactivate_user`, `activate_user`). The connector also syncs organization API tokens and can create accounts through Atlassian's SCIM API when SCIM credentials are configured.

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
docker run --rm -v $(pwd):/out -e BATON_ACCESS_TOKEN=ACCESS_TOKEN -e BATON_ORGANIZATION_ID=ORG_ID public.ecr.aws/conductorone/baton-atlassian:latest -f "/out/sync.c1z"
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
| **API Tokens** | User API tokens issued in the organization, linked to the user that owns each token |

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
| `atlassian/user-access-admin` | User access administration on the workspace |
| `atlassian/site-admin` | Site administration |
| `atlassian/org-admin` | Organization administration |

`atlassian/org-admin`, `atlassian/site-admin` and `atlassian/user-access-admin` appear in both tables. The connector places each grant by the `resourceOwner` of the role assignment, not by the role name: a `platform` assignment lands on the organization and is read from users only, a `project` or `goal` assignment is skipped, and anything else lands on the workspace it belongs to.

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
  config             Get the connector config schema
  health-check       Check the health of a running connector
  help               Help about any command

Flags:
      --access-token string                              required: Access Token used to authenticate with the Atlassian API. ($BATON_ACCESS_TOKEN)
      --auth-method string                               ($BATON_AUTH_METHOD)
      --client-id string                                 The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string                             The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
      --external-resource-c1z string                     The path to the c1z file to sync external baton resources with ($BATON_EXTERNAL_RESOURCE_C1Z)
      --external-resource-entitlement-id-filter string   The entitlement that external users, groups must have access to sync external baton resources ($BATON_EXTERNAL_RESOURCE_ENTITLEMENT_ID_FILTER)
      --external-resource-traits strings                 Resource type traits (e.g. "user", "group", "app") to sync and match from the external resource c1z. When unset the matcher falls back to user and group; passing this flag replaces the full set rather than adding to it. ($BATON_EXTERNAL_RESOURCE_TRAITS)
  -f, --file string                                      The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
      --health-check                                     Enable the HTTP health check endpoint ($BATON_HEALTH_CHECK)
      --health-check-port int                            Port for the HTTP health check endpoint ($BATON_HEALTH_CHECK_PORT) (default 8081)
  -h, --help                                             help for baton-atlassian
      --http-timeout-seconds int                         HTTP client timeout in seconds (max 1800) ($BATON_HTTP_TIMEOUT_SECONDS) (default 300)
      --keep-previous-sync-c1z                           Keep the previously synced c1z on disk to enable ETag replay across service-mode syncs (requires a connector that supports ETag replay; costs one c1z of local disk) ($BATON_KEEP_PREVIOUS_SYNC_C1Z)
      --log-format string                                The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                                 The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --log-level-debug-expires-at string                The timestamp indicating when debug-level logging should expire ($BATON_LOG_LEVEL_DEBUG_EXPIRES_AT)
      --log-path strings                                 The file path to write logs to ($BATON_LOG_PATH)
      --organization-id string                           required: Organization that will be synchronized. ($BATON_ORGANIZATION_ID)
      --otel-collector-endpoint string                   The endpoint of the OpenTelemetry collector to send observability data to (used for both tracing and logging if specific endpoints are not provided) ($BATON_OTEL_COLLECTOR_ENDPOINT)
      --parallel-sync                                    Deprecated: use --workers instead. ($BATON_PARALLEL_SYNC)
  -p, --provisioning                                     This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --scim-base-url string                             The SCIM base URL for user provisioning (you can get it along with the token) ($BATON_SCIM_BASE_URL)
      --scim-token string                                SCIM Token used for user provisioning operations. ($BATON_SCIM_TOKEN)
      --skip-entitlements-and-grants                     This must be set to skip syncing of entitlements and grants ($BATON_SKIP_ENTITLEMENTS_AND_GRANTS)
      --skip-full-sync                                   This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --storage-engine string                            The storage engine to use when opening the sync c1z file. Only pebble is supported; defaults to pebble when unset. ($BATON_STORAGE_ENGINE)
      --sync-resource-types strings                      The resource type IDs to sync ($BATON_SYNC_RESOURCE_TYPES)
      --sync-resources strings                           The resource IDs to sync ($BATON_SYNC_RESOURCES)
      --task-concurrency int                             The number of Baton tasks to run concurrently in service mode. Tasks may include sync, grant, revoke, and more. Minimum value is 1, maximum value is 100. ($BATON_TASK_CONCURRENCY) (default 3)
      --ticketing                                        This must be set to enable ticketing support ($BATON_TICKETING)
  -v, --version                                          version for baton-atlassian
      --workers int                                      The number of sync workers to use. -1 for auto-detect, 0 for sequential, >0 for parallel ($BATON_WORKERS)

Use "baton-atlassian [command] --help" for more information about a command.
```
