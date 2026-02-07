# Change Log

## 2026-02-06

### Summary
- OAuth access token refresh now respects expiry (prevents stale token failures).
- Admin endpoints require a token by default; webhook validation can use a shared token.
- Gemini calls can be combined into a single request (feature-flagged).
- Cloud Run deploy defaults adjusted for cost control.
- JSON structured logs + HTTP access logs added (optional via env).
- NotebookLM sync now skips already-synced files (reduces duplicate OCR/Gemini cost).
- PDF page rendering output is read in numeric page order (avoids 1,10,2 ordering).
- Basic unit tests added (auth/token-expiry/PDF ordering).
- Deploy scripts now pass `LOG_FORMAT` / `LOG_LEVEL` when set.
- Added `/admin/ping` for auth verification.
- `deploy.sh` supports Secret Manager injection via `USE_SECRET_MANAGER=1`.

### Deploy Defaults (before -> after)
- `memory`: 512Mi -> 384Mi
- `concurrency`: 80 -> 4
- `max-instances`: 10 -> 3

### New/Updated Environment Variables
- `ADMIN_AUTH_MODE` = `required|optional|disabled` (default `required`)
- `ADMIN_TOKEN` = admin bearer token for protected endpoints
- `DRIVE_WEBHOOK_TOKEN` = token used in Drive watch channel and validated on callbacks
- `ENABLE_COMBINED_GEMINI` = `true|false` (default `true`)
- `LOG_FORMAT` = `json|text` (default `text`)
- `LOG_LEVEL` = `debug|info|warn|error` (default `info`)

### Rollback Instructions
1. Disable combined Gemini:
   - `ENABLE_COMBINED_GEMINI=false`
2. Disable admin auth (legacy behavior):
   - `ADMIN_AUTH_MODE=disabled`
3. Revert Cloud Run resources in `cloud-run-go/deploy.sh` and `cloud-run-go/deploy-cloudbuild.sh`:
   - `--memory 512Mi`, `--concurrency 80`, `--max-instances 10`
