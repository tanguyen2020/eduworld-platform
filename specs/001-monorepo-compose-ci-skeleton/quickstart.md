# Quickstart: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: —
- Công cụ theo [LOCAL_DEVELOPMENT §1](../../docs/11-operations/LOCAL_DEVELOPMENT.md); `make tools`; `gh` CLI đăng nhập với quyền đọc cấu hình repository (kịch bản 13, 20).
- Lệnh: `cp .env.example .env && make env`, `make up`, `make migrate`, `make up-app`, `make up-obs`.
- Tài khoản demo: không cần (F001 chưa có đăng nhập; realm do F004).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `make up`; `docker compose -f deploy/compose/docker-compose.yml ps` | postgres (một cụm, 5 database module + `keycloak`), valkey, nats, keycloak, apisix, mailpit, floci, cdn, sis-mock `healthy`; `s3-init` exited 0 | F001-AC-01 |
| 2 | `git check-ignore .env src/backend/modules/identity/.env` | Cả hai bị ignore; `.env.example` có mọi biến bắt buộc | F001-AC-02 |
| 3 | `make migrate`; chạy `core-api` rồi kiểm `atlas migrate status` từng module | Mỗi database ở revision baseline; log `core-api` không có bước migrate | F001-AC-03 |
| 4 | `curl -s localhost:9100/livez`, `/readyz`, `/version`; `curl localhost:9106/readyz`; `curl localhost:9200/readyz` | 200; `/version` có `binary=core-api`, `version`, `commit` | F001-AC-04, F001-AC-19 |
| 5 | `docker compose stop valkey`; `curl localhost:9100/readyz` và `/livez` | `/readyz` 503 có check `valkey: fail`; `/livez` 200 | F001-AC-05 |
| 6 | `curl localhost:9080/readyz` (qua APISIX) | 404 (không có route) | F001-AC-06 |
| 7 | `make up-obs`; mở Grafana `localhost:3000` | Datasource Prometheus, Loki, Tempo đã có | F001-AC-07 |
| 8 | `psql -U learning_app -d eduworld_identity` | `permission denied for database` | F001-AC-08, F001-AC-09 |
| 9 | Trong `eduworld_learning` bằng `learning_app`: `DELETE FROM <bảng thử>` | `permission denied` | F001-AC-10 |
| 10 | `make db-privileges` | Exit 0; cố ý `GRANT DELETE` → exit ≠ 0 | F001-AC-11 |
| 11 | PR mẫu chỉ sửa `src/backend/modules/learning/README.md` | `go.yml` chạy `test (learning)` + `arch` + `db-privileges`, `security.yml`; không có `test (game)`…, `godot.yml`, `web.yml` | F001-AC-12 |
| 12 | PR mẫu thêm chuỗi giống AWS key | `security.yml` đỏ, PR không merge được | F001-AC-13, F001-AC-16 |
| 13 | `bash scripts/ci/apply-repo-settings.sh --check` | Branch protection `main` có required checks + CODEOWNERS review; Environments `staging`, `production` có required reviewers | F001-AC-16, F001-AC-26 |
| 14 | Nhánh thử có dependency chứa CVE đã biết | `security.yml` (`govulncheck`/OSV) đỏ | F001-AC-14 |
| 15 | `docker inspect ghcr.io/<org>/eduworld/core-api:<sha>`; `cosign verify` | `User` ≥ 10001; có SBOM; chữ ký hợp lệ; không tag `latest` | F001-AC-15 |
| 16 | PR mẫu chỉ sửa `docs/README.md` | Chỉ `docs.yml` và `security.yml` chạy | F001-AC-24 |
| 17 | `node scripts/ci/check-workflows.mjs`; sửa thử một `uses:` sang tag `@v4` | Pass; sau sửa fail chỉ đúng file/dòng | F001-AC-25 |
| 18 | `go -C src/backend build ./cmd/...`; `ls src/backend/modules/*/` | Ba binary build được; mọi module có `api/`, `internal/`, `module.go` | F001-AC-17 |
| 19 | `bash scripts/ci/test-arch-fixtures.sh` | Import `src/backend/modules/learning/internal/...` từ `game` → `go build` fail; import `src/backend/modules/identity/api` từ module ngoài `x-allowed-callers` → `make arch-check` fail; `src/backend/pkg/` import `src/backend/modules/` → fail | F001-AC-21, F001-AC-22, F001-AC-23 |
| 20 | Xem workflow `release.yml`: job deploy | `runs-on: [self-hosted, deploy]`, `environment` có reviewer, điều kiện chặn fork | F001-AC-26 |
| 21 | `make up-app EXCEPT=core-api` + `make run-core-api` | `realtime`, `worker` chạy container, `core-api` chạy native và `/readyz` 200 | F001-AC-18 |
| 22 | `make db-backup MOD=identity`; `make reset`; `make up migrate`; `make db-restore MOD=identity FILE=…` | Dump chỉ chứa `eduworld_identity`; khôi phục thành công | F001-AC-20 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity   # health + role DB (testcontainers)
make arch-check
make db-privileges
bash scripts/ci/test-path-filters.sh
bash scripts/ci/test-arch-fixtures.sh
bash scripts/ci/check-admin-ports.sh
node scripts/ci/check-workflows.mjs
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), thời gian dựng stack trên máy tham chiếu, ảnh danh sách check của PR mẫu (kịch bản 11, 12, 16), output `apply-repo-settings.sh --check`, báo cáo Trivy + SBOM + `cosign verify` của một image.

## 5. Bằng chứng chạy (2026-11-17, Windows 11 + Docker Desktop 8 GB, Go 1.27.0)

| # | Kết quả | Bằng chứng |
|---|---|---|
| 1 | Đạt | 9 dịch vụ hạ tầng healthy, `s3-init` exit 0 — `scripts/ci/smoke-compose.sh` 13/13 ok |
| 2 | Một phần | `.env` và `src/backend/modules/identity/.env` bị git ignore; `.env.example` **chưa tạo** (quyền ghi file `.env*` bị chặn trong phiên, T003/T026) |
| 3 | Đạt | `migrate --all` (native và image `migrate`) áp baseline 5 database; chạy lại không đổi gì; binary không migrate (`TestCoreAPIDoesNotAutoMigrate`) |
| 4 | Đạt | `/livez` `/readyz` `/version` 200 trên 9100/9106/9200 (`scripts/ci/test-up-app-except.sh`) |
| 5 | Đạt | Dừng Valkey → `/readyz` 503 `valkey: fail`, `/livez` 200 (`TestCoreAPIReadinessFollowsDependencies`) |
| 6 | Đạt | APISIX `:9080/readyz` → 404 |
| 7 | Đạt | Grafana có datasource Prometheus, Loki, Tempo; Prometheus scrape `core-api`, `realtime`, `worker`, `otel-collector` = up |
| 8, 9 | Đạt | `TestRolesConnectOnlyToOwnDatabase` (37 role × 6 database), `TestAppRolesCannotDeleteOrTruncate` |
| 10 | Đạt | `db-privileges` ok; cố ý `GRANT DELETE` → exit 1 |
| 11, 16 | Mô phỏng | `scripts/ci/test-path-filters.sh` 19/19 (đánh giá `paths` và `module-filters.yml`); kiểm trên PR thật chờ push GitHub |
| 12 | Mô phỏng | `scripts/ci/test-gitleaks.sh` ok; quét toàn repo: no leaks found; PR thật chờ GitHub |
| 13, 20 | Chưa chạy | `apply-repo-settings.sh --check` cần `gh` CLI và quyền admin repo |
| 14 | Đạt (local) | `scripts/ci/test-vuln-scan.sh`: govulncheck báo GO-2022-1059; trivy fs repo: 0 lỗi (Critical/High có bản vá) |
| 15 | Một phần | `check-image.sh`: 4 image UID 10001, có nhãn version, không shell; SBOM + cosign chỉ kiểm được khi `release.yml` chạy trên GitHub |
| 17 | Đạt | `check-workflows.mjs` 6 workflow 0 lỗi; test 10/10 (ca `uses: …@v4` fail đúng dòng) |
| 18 | Đạt | `go build ./...` (trong `src/backend`); mọi module có `api/`, `internal/`, `module.go` |
| 19 | Đạt | `scripts/ci/test-arch-fixtures.sh` 8/8 |
| 21 | Đạt (container) | `EXCEPT=core-api` chỉ chạy `realtime` + `worker`; chạy `core-api` native được kiểm qua integration test (`run()` trong tiến trình) |
| 22 | Đạt | `scripts/ops/test-backup-restore.sh`: 42 dòng, revision và quyền của `identity_app` giữ nguyên |

Unit + integration: `go -C src/backend test ./...` pass; `go -C src/backend test -tags integration ./...` pass (core-api, worker, test/postgres; `TESTCONTAINERS_RYUK_DISABLED=true` trên Docker Desktop Windows).
