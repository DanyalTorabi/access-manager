# API contract artifacts

| File | Purpose |
|------|---------|
| [openapi.yaml](openapi.yaml) | OpenAPI 3.0 description of all public HTTP routes (`/health`, `/api/v1/...`). Root `security` lists `{}` and `bearerAuth` so the contract matches optional Bearer when the token env is unset. |
| [postman/access-manager.postman_collection.json](postman/access-manager.postman_collection.json) | Postman v2.1 collection for manual calls. |

## Postman

1. **Import** `postman/access-manager.postman_collection.json` (Import → file).
2. Open the collection **Variables** tab (or edit collection variables):
   - **`baseUrl`** — e.g. `http://127.0.0.1:8080` (match `HTTP_ADDR`).
   - **`bearerToken`** — your `API_BEARER_TOKEN` value if the server requires Bearer auth; leave empty if not configured.
3. **Run collection** top-to-bottom (or use the Postman Collection Runner). Create requests have **Tests** scripts that auto-save the returned `ID` into collection variables (`domainId`, `userId`, `groupId`, `resourceId`, `accessTypeId`, `permissionId`), so subsequent requests use the correct IDs without manual copy-paste.
4. Collection auth is **Bearer Token** using `{{bearerToken}}`. Postman omits or sends an empty token when the variable is blank (behavior may vary by version); if requests **return 401**, set the token or disable collection auth for local dev without Bearer. The **Health** request overrides auth to **No auth** so `/health` stays unauthenticated when `bearerToken` is set.
5. The collection is ordered so that entities are **created before they are referenced** and **deleted in reverse dependency order** (revoke grants → delete permissions/access-types/resources → remove memberships → delete users/groups → delete domain).

## OpenAPI

View or validate with any OpenAPI 3 tool (e.g. [Swagger Editor](https://editor.swagger.io/), `npx @redocly/cli lint openapi.yaml`). Keep this file aligned with [`go/internal/api/server.go`](../go/internal/api/server.go) when routes change.
