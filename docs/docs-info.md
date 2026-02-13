## Connector capabilities

1. What resources does the connector sync?
- The Atlassian connector syncs:
  - **Organization**: The Atlassian organization with platform-level roles
  - **Users**: All users in the organization directory
  - **Groups**: All groups in the organization (e.g., org-admins, jira-admins, jira-users) with their memberships
  - **Workspaces**: Product-sites (Jira, Jira Software, Confluence, Rovo, etc.)

2. What roles/grants does the connector sync?

- **Organization-level Roles (Platform Roles)**: These roles are synced **directly from users only** (not from groups, as users already have these roles assigned directly via the API).
  - `atlassian/org-admin`: Organization administrator with full access
  - `atlassian/site-admin`: Site administrator
  - `atlassian/user-access-admin`: User access administrator
  - `atlassian/ai-access`: Atlassian Intelligence access (Premium/Enterprise plans only)

- **Workspace-level Roles**: Roles assigned to **users and groups** on specific workspaces (product-sites)
  - `atlassian/user`, `atlassian/admin`, `atlassian/guest`, `atlassian/contributor`, `atlassian/customer`, `atlassian/basic`, `atlassian/stakeholder`

- **Group Memberships**: Users that belong to each group

3. Can the connector provision any resources? If so, which ones?
- This connector can provision User Roles, meaning that you can Grant or Revoke roles on different Workspaces (product-sites) within the organization.
  Customer must consider that depending on their settings not all roles may be available for all sites. There are some of them that could be enabled or not on certain sites; there may be other roles that can be only configured if the user meets certain requirements.
  The connector is unable to determine which will be effectively available, but a descriptive error message should communicate the situation when a Grant can't be provisioned.

## Connector credentials

1. What credentials or information are needed to set up the connector? (For example, API key, client ID and secret, domain, etc.)
- For this connector, an access-token and the ID of the Atlassian Organization are required. The flags are `--access-token` and `--organization-id`.

2. For each item in the list above:

   * How does a user create or look up that credential or info? Please include links to (non-gated) documentation, screenshots (of the UI or of gated docs), or a video of the process.

    - **API Key (access-token)**:
      1. Log into admin.atlassian.com with an Org admin account
      2. Navigate to **Organization Settings > API keys**
      3. Click **Create API key**
      4. **Select "API key without scopes"** and set a name and expiration date
      5. Copy the generated API key

      Atlassian Support Guide: https://support.atlassian.com/organization-administration/docs/manage-an-organization-with-the-admin-apis/

    - **Organization ID**:
      The Organization ID can be found in the URL when logged into Atlassian Admin:
      `https://admin.atlassian.com/o/{organizationId}/`

    - **Provisioning (optional)**:
      To be able to configure roles through the API, Customers need to raise a ticket to Atlassian Support (https://support.atlassian.com/contact/#/) asking for them to enable certain endpoints.
      Once they enter on the Atlassian Support Portal, login into their admin account for the organization and select Contact: https://support.atlassian.com/contact/#/
      They should follow these steps:
        1. In the drop-down box under the "What can we help you with?" they should select the option: "Technical issues and bugs"
        2. In the drop-down box under the "Which product is this for?" they should select any site of their Cloud products. For example, one JIRA instance.
        3. A big textbox should appear. Under it, the "Raise a support request" text should be visible. The customer should click on it to deploy the other data inputs and to allow them to create the ticket for Atlassian Support Team.
        4. The email of the admin user should be indicated under the "Include admin or billing/technical/end-customer contact, or additional participants on this ticket" label.
        5. On the "Summarize your issue" text box, customers should enter this:
          -> I need access to the indicated endpoints of the Atlassian Administration Cloud API. The documentation states that access should be requested to the support team.
        6. On the "What is the impact to your business?" drop-down box, "Level 3" should be selected.
        7. On "Give us more details*" section, the following text must be included:
          ->
             I need access to the following endpoints of the Atlassian Administration Cloud API:
             * Grant User Access: https://developer.atlassian.com/cloud/admin/organization/rest/api-group-directory/\#api-v1-orgs-orgid-users-userid-roles-assign-post
             * Revoke User Access: https://developer.atlassian.com/cloud/admin/organization/rest/api-group-directory/\#api-v1-orgs-orgid-users-userid-roles-revoke-post
             * Invite User to Org: https://developer.atlassian.com/cloud/admin/organization/rest/api-group-directory/\#api-v1-orgs-orgid-users-invite-post
             I'm working with an API Client and I need to be able to use these endpoints on my Atlassian Organization.

             My organization ID is: <ATLASSIAN_ORGANIZATION_ID>

             Thanks in advance for your help!

        8. Please be sure to replace "<ATLASSIAN_ORGANIZATION_ID>" with the corresponding Organization ID.


   * Does the credential need any specific scopes or permissions? If so, list them here.
    - The user should be an **Org admin** of the organization.
    - The API key **must be created with "no scopes"** (API key without scopes).

      **Why "API key without scopes" is required:**
      According to Atlassian's official documentation:
      - Support Guide: https://support.atlassian.com/organization-administration/docs/manage-an-organization-with-the-admin-apis/
      - API Reference: https://developer.atlassian.com/cloud/admin/organization/rest/intro/
      
      > "Currently, scopes are not available for all endpoints. If the endpoint you want to use is not listed, you need to use an API key without scopes to access that endpoint."

      The following endpoints used by this connector do not have scopes available yet:
      - `/v2/orgs/{orgId}/directories/-/users`
      - `/v2/orgs/{orgId}/directories/-/groups`
      - `/v2/orgs/{orgId}/directories/-/users/{accountId}/role-assignments`
      - `/v2/orgs/{orgId}/directories/-/groups/{groupId}/role-assignments`
      - `/v1/orgs/{orgId}`

   * If applicable: Is the list of scopes or permissions different to sync (read) versus provision (read-write)? If so, list the difference here.
   - This doesn't apply when the API Key is generated with an admin user and "no scopes" specified. The same API key works for both sync and provisioning.

   * What level of access or permissions does the user need in order to create the credentials? (For example, must be a super administrator, must have access to the admin console, etc.)
   - The user must be an **Org admin** of the Atlassian organization.
