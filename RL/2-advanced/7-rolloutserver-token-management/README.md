# RolloutServer Token Management

This guide covers RolloutServer's API authentication token system: the permission boundary between regular and admin tokens, configuration and Secret injection, smooth multi-token rotation, and the batch controller's separate authentication. Model API keys (e.g. `OPENAI_API_KEY`) are out of scope here — they are configured per request; see [Serving Evaluation and RL on One RolloutServer](../5-rolloutserver-eval-rl/README.md).

## Two Token Levels

| Token type | Config field | Permissions |
|---|---|---|
| Regular token | `auth_tokens` (list) | All regular endpoints: submission, queries, cancellation |
| Admin token | `admin_tokens` (list) | Everything a regular token can do + quota write operations |

Admin-only endpoints (writes):

- `PUT /api/v1/quotas/{queue}` (runtime queue quota changes)
- `POST /api/v1/quotas/reconcile` (manual quota reconciliation)
- `PUT /api/v1/job-quotas`, `PUT/DELETE /api/v1/job-quotas/{job_id}`

GET queries on the same paths require only a regular token. An admin token also works as a regular token.

Unauthenticated endpoints: `/health`, `/metrics`, `/help`, `/api/v1/help`, `/api/config`, `/api/health`, `/debug/*`. Everything else (including submission, cancellation, and the Viewer's job/trial queries) requires a token.

**When `auth_tokens` is not configured, all endpoints are open** (startup log shows auth=disabled). Always configure it in production. With only `admin_tokens` configured, quota writes still require an admin token while other endpoints are open.

## Presenting a Token

Recognized in this order (any one):

1. `Authorization: Bearer <token>` (recommended)
2. `X-Auth-Token: <token>` header
3. `?token=<token>` query parameter
4. `rollout_token` cookie (for browser UI access, opening a page with `?token=` sets a 7-day HttpOnly cookie)

```bash
curl -s $SERVER/api/v1/quotas -H "Authorization: Bearer $TOKEN"
```

Missing token returns 401; invalid token returns 403. Validation uses constant-time comparison (`hmac.compare_digest`) against timing attacks.

## Configuration and Secret Injection

### Production: existingSecret (recommended)

Create a Secret with two keys:

```bash
kubectl create secret generic kube-rl-auth -n kube-rl \
  --from-literal=tokens='<token1>,<token2>' \
  --from-literal=admin-tokens='<admin-token>'
```

Reference it in values:

```yaml
auth:
  existingSecret: kube-rl-auth
```

Behavior:

- The ConfigMap no longer contains token plaintext; server roles (single-mode / ingress / viewer) receive env vars `KUBE_RL_AUTH_TOKENS` (key `tokens`) and `KUBE_RL_ADMIN_TOKENS` (key `admin-tokens`, optional).
- Environment variables take precedence over same-named config-file keys.
- The batch controller automatically uses the **first** token of `tokens` as its outbound credential.
- Workers receive no tokens (no HTTP surface).

### Testing: inline tokens

```yaml
auth:
  tokens: ["test-token"]
  adminTokens: ["test-admin"]
```

Inline values write tokens into a **plaintext ConfigMap** — for tests only. Use existingSecret in production.

## Smooth Rotation with Multiple Tokens

`auth_tokens` / `admin_tokens` are lists, so old and new tokens can coexist. Rotation steps:

1. **Append** the new token to `tokens` in the Secret (keep the old one).
2. Restart to take effect: `kubectl -n kube-rl rollout restart deploy/kube-rl-ingress` (single mode: `deploy/kube-rl`; likewise for the viewer).
3. Migrate clients to the new token.
4. After confirming no old-token traffic, remove the old token from the Secret and restart again.

Notes:

- Tokens load only at process startup; there is **no hot reload**.
- Changing existingSecret content does **not** trigger a rolling update automatically (Pod annotations carry only the ConfigMap checksum); a manual `rollout restart` is required.
- Inline tokens roll automatically on `helm upgrade` because the ConfigMap changes.

## Batch Controller's Separate Authentication

The batch controller (8082) protects its own API with tokens independent of the server:

- Inject `BATCH_CONTROLLER_API_TOKENS` (comma-separated list) via `batchController.extraEnv`:

```yaml
batchController:
  extraEnv:
    - name: BATCH_CONTROLLER_API_TOKENS
      valueFrom:
        secretKeyRef:
          name: kube-rl-auth
          key: tokens
```

- Once configured, its `/api/v1/*` endpoints require `Authorization: Bearer` (header only; no query/cookie fallback); `/health` and `/api/v1/help` stay unauthenticated.
- Without it, the batch API is fully open.
- The chart does not render this variable automatically; configure it explicitly.

## Security Checklist

- [ ] Production uses `auth.existingSecret`; no token plaintext in ConfigMaps.
- [ ] `admin_tokens` configured; quota writes separated from daily submission permissions.
- [ ] Batch controller has `BATCH_CONTROLLER_API_TOKENS` configured.
- [ ] Clients use `Authorization: Bearer` uniformly; no tokens in URLs (avoids log leakage).
- [ ] Rotation flow verified: append new token → restart → migrate → remove old token.
- [ ] Model API keys are not in the server's global environment or `env_profiles` (per-request configuration; see the dual-mode guide).