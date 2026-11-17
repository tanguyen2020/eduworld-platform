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
| 2 | `git check-ignore .env modules/identity/.env` | Cả hai bị ignore; `.env.example` có mọi biến bắt buộc | F001-AC-02 |
| 3 | `make migrate`; chạy `core-api` rồi kiểm `atlas migrate status` từng module | Mỗi database ở revision baseline; log `core-api` không có bước migrate | F001-AC-03 |
| 4 | `curl -s localhost:9100/livez`, `/readyz`, `/version`; `curl localhost:9106/readyz`; `curl localhost:9200/readyz` | 200; `/version` có `binary=core-api`, `version`, `commit` | F001-AC-04, F001-AC-19 |
| 5 | `docker compose stop valkey`; `curl localhost:9100/readyz` và `/livez` | `/readyz` 503 có check `valkey: fail`; `/livez` 200 | F001-AC-05 |
| 6 | `curl localhost:9080/readyz` (qua APISIX) | 404 (không có route) | F001-AC-06 |
| 7 | `make up-obs`; mở Grafana `localhost:3000` | Datasource Prometheus, Loki, Tempo đã có | F001-AC-07 |
| 8 | `psql -U learning_app -d eduworld_identity` | `permission denied for database` | F001-AC-08, F001-AC-09 |
| 9 | Trong `eduworld_learning` bằng `learning_app`: `DELETE FROM <bảng thử>` | `permission denied` | F001-AC-10 |
| 10 | `make db-privileges` | Exit 0; cố ý `GRANT DELETE` → exit ≠ 0 | F001-AC-11 |
| 11 | PR mẫu chỉ sửa `modules/learning/README.md` | `go.yml` chạy `test (learning)` + `arch` + `db-privileges`, `security.yml`; không có `test (game)`…, `godot.yml`, `web.yml` | F001-AC-12 |
| 12 | PR mẫu thêm chuỗi giống AWS key | `security.yml` đỏ, PR không merge được | F001-AC-13, F001-AC-16 |
| 13 | `bash scripts/ci/apply-repo-settings.sh --check` | Branch protection `main` có required checks + CODEOWNERS review; Environments `staging`, `production` có required reviewers | F001-AC-16, F001-AC-26 |
| 14 | Nhánh thử có dependency chứa CVE đã biết | `security.yml` (`govulncheck`/OSV) đỏ | F001-AC-14 |
| 15 | `docker inspect ghcr.io/<org>/eduworld/core-api:<sha>`; `cosign verify` | `User` ≥ 10001; có SBOM; chữ ký hợp lệ; không tag `latest` | F001-AC-15 |
| 16 | PR mẫu chỉ sửa `docs/README.md` | Chỉ `docs.yml` và `security.yml` chạy | F001-AC-24 |
| 17 | `node scripts/ci/check-workflows.mjs`; sửa thử một `uses:` sang tag `@v4` | Pass; sau sửa fail chỉ đúng file/dòng | F001-AC-25 |
| 18 | `go build ./cmd/...`; `ls modules/*/` | Ba binary build được; mọi module có `api/`, `internal/`, `module.go` | F001-AC-17 |
| 19 | `bash scripts/ci/test-arch-fixtures.sh` | Import `modules/learning/internal/...` từ `game` → `go build` fail; import `modules/identity/api` từ module ngoài `x-allowed-callers` → `make arch-check` fail; `pkg/` import `modules/` → fail | F001-AC-21, F001-AC-22, F001-AC-23 |
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
