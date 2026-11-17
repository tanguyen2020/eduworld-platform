# Data Model: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

F001 không tạo bảng nghiệp vụ. Feature tạo **một cụm PostgreSQL với database, role, schema namespace và hàm hạ tầng theo module** mà mọi feature sau dùng (migration baseline của từng module, ADR-0074 mục 4).

## 1. Entity

### Database và role theo module — (Có sẵn, DATABASE_DESIGN §1.1–§1.2)

Năm database module nằm chung một cụm PostgreSQL (local: một container `postgres`; staging/production: Patroni + PgBouncer do F130). Binary nạp module nào thì mở pool tới database của module đó bằng role của module (`core-api`, `worker` nạp nhiều module; `realtime` không có DB).

| Database | Module | Binary kết nối | Schema tạo trong baseline | Role |
|---|---|---|---|---|
| `eduworld_identity` | `identity` | `core-api`, `worker` (`events`, `jobs`) | `platform`, `identity`, `org`, `catalog`, `sis`, `audit`, `infra` | `identity_app`, `identity_platform`, `identity_migrator` |
| `eduworld_learning` | `learning` | `core-api`, `worker` (`events`, `jobs`) | `curriculum`, `content`, `assessment`, `learning`, `lms`, `ref`, `infra` | `learning_app`, `learning_platform`, `learning_migrator` |
| `eduworld_game` | `game` | `core-api`, `worker` (`events`, `jobs`) | `game`, `gamification`, `analytics`, `ref`, `infra` | `game_app`, `game_platform`, `game_migrator` |
| `eduworld_competition` | `competition` | `core-api`, `worker` (`events`, `jobs`) | `multiplayer`, `competition`, `ref`, `infra` | `competition_app`, `competition_platform`, `competition_migrator` |
| `eduworld_chat` | `chat` | `core-api`, `worker` (`events`, `chat`, `jobs`) | `social`, `chat`, `ref`, `infra` | `chat_app`, `chat_platform`, `chat_migrator`, `chat_purger` |
| `keycloak` | Keycloak | — | (Keycloak tự quản lý) | `keycloak` |

Ghi chú: `catalog` (DATABASE_DESIGN §2.2a) và `analytics` (§2.8b) có trong danh sách bảng nhưng chưa có trong bảng §1.2 → task cập nhật DATABASE_DESIGN §1.2. Role `<module>_readonly`, `<module>_relay`, `<module>_partition_admin` (ADR-0089) do F003 tạo cùng migration `infra`.

| Thuộc tính role | `<module>_migrator` | `<module>_app` | `<module>_platform` |
|---|---|---|---|
| LOGIN | có | có | có |
| Sở hữu schema | có | — | — |
| `BYPASSRLS` | — | `NOBYPASSRLS` | có |
| `SELECT, INSERT, UPDATE` | có | có | có |
| `DELETE, TRUNCATE` | có (DDL) | **không** | **không** |
| `CONNECT` | chỉ DB của module | chỉ DB của module | chỉ DB của module |

### Hàm `infra.current_tenant_id()` — (Có sẵn, DATABASE_DESIGN §4)

| Thành phần | Định nghĩa | Ghi chú |
|---|---|---|
| `infra.current_tenant_id()` | `LANGUAGE sql STABLE`, trả `NULLIF(current_setting('app.tenant_id', true), '')::uuid` | Dùng bởi RLS của F002/F003 và mọi bảng T/TA/H; không set → `NULL` → 0 dòng |

**Quan hệ**: không có FK chéo database, không join hay transaction chéo database (ADR-0037, ADR-0074 mục 4).
**Index**: không có.

## 2. Vòng đời / trạng thái

Migration baseline `modules/<module>/migrations/00000000000001_baseline.sql`: tạo schema → tạo hàm `infra.current_tenant_id()` → `REVOKE DELETE, TRUNCATE ON ALL TABLES IN SCHEMA <schemas> FROM <module>_app, <module>_platform` → `ALTER DEFAULT PRIVILEGES FOR ROLE <module>_migrator IN SCHEMA <schemas> GRANT SELECT, INSERT, UPDATE ON TABLES TO <module>_app, <module>_platform` (không `DELETE`). Áp bằng job `migrate --module=<module>` (image `eduworld/migrate`); Atlas lưu revision trong `atlas_schema_revisions` (schema `public` của database module).

## 3. Quy tắc kiểm tra dữ liệu

- Không role nào có `CONNECT` vào database của module khác (test F001-AC-09, `scripts/ci/db-privileges.sql`).
- `chat_purger` chỉ được cấp quyền ở F117 (ADR-0033); baseline F001 chỉ tạo role, không cấp `DELETE`.
- Không seed vào schema `ref` (LOCAL_DEVELOPMENT §5).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | F001 không phát event (bảng `infra.outbox_events` do F003 tạo) | — |
