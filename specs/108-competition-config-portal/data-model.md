# Data Model: F108 — Cấu hình thi đấu `/game-admin/*` và UI workspace A

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_competition`, schema `competition`. Bảng do F104 tạo (Mới, G); F108 thêm phía ghi. Portal không có dữ liệu bền vững riêng.

## 1. Entity

### GameConfig — `competition.game_configs` (Mới ở F104, G)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `config_id` | UUIDv7 | PK | |
| `config_type` | `varchar(64)` | CHECK 7 loại (`GameConfigType` phần competition, gồm `competition-rewards`) | |
| `code` | `varchar(64)` | UNIQUE `(config_type, code) WHERE deleted_at IS NULL` | |
| `current_version_id` | uuid | NULL = chưa publish/unpublish | Con trỏ (ADR-0043) |
| `latest_version_no` | int | ≥ 0 | |
| `owner_scope` | `varchar(64)` | = `platform` | |
| `deleted_at`, `deleted_by` | | | Soft-delete |

### GameConfigVersion — `competition.game_config_versions` (Mới ở F104, G)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `config_version_id` | UUIDv7 | PK | |
| `config_id`, `version` | uuid, int | UNIQUE `(config_id, version)` | |
| `status` | `varchar(64)` | CHECK `IN ('draft','in_review','approved','published','archived','rejected')` | `ContentStatus` |
| `title` | `varchar(255)` | | |
| `payload` | jsonb | CHECK `octet_length ≤ 65536`; validate JSON Schema theo loại | |
| `effective_from` | timestamptz | NULL tới khi publish | |
| `author_id`, `reviewer_id`, `published_by` | uuid | | |
| `review_note`, `change_note` | `varchar(1000)`, `varchar(500)` | | |
| `based_on_version_id` | uuid | NULL | |
| `published_at` | timestamptz | | |
| `version_etag` | int | | `If-Match` |

Trigger: chặn UPDATE cột nội dung khi `status IN ('published','archived')` (R11).

### ContentRelease — `competition.content_releases` (Mới ở F104, append-only)

`release_id` · `config_id` · `action varchar(64)` CHECK `IN ('publish','rollback','unpublish')` · `from_version_id`, `to_version_id` · `effective_from` · `reason varchar(1000)` (bắt buộc với rollback/unpublish) · `released_by` · `released_at`.

### Runtime áp từ cấu hình (bảng của F106/F107)

| Loại | Bảng runtime | Cách áp |
|---|---|---|
| `seasons` | `competition.seasons` | upsert theo `code`, giữ `season_id` |
| `leaderboard-definitions` | `competition.leaderboard_definitions` | upsert theo `code`, `config_version_id` |
| `matchmaking-configs`, `ranking-rules` | — | đọc trực tiếp version hiệu lực qua `ConfigReader` |
| `tournament-templates`, `challenge-templates` | — | ghim `template_version_id` khi tạo Challenge (F107) |
| `competition-rewards` | — | đọc version hiệu lực khi trao thưởng mùa/giải/thử thách (F106/F107), ghim vào kết quả |

## 2. Vòng đời / trạng thái

`draft → in_review → approved → published → archived`; `in_review → rejected → draft` (bản mới). Rollback/unpublish đổi `current_version_id`, không đổi `status` version cũ.

## 3. Quy tắc kiểm tra dữ liệu

- Payload theo `specs/schemas/<type>.schema.json` (§contracts 5) + invariant ở [spec F108-AC-04](spec.md).
- `effective_from ≥ now()` khi publish; mùa không chồng thời gian.
- Template: `allowed_params ⊆ {name, targets, starts_at, ends_at, participation, question_set_version_id, max_entries, seeding_policy}`; `reward_rule_refs` chỉ cosmetic/badge/title.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `competition_config.published` | Publish, rollback, unpublish | `specs/events/payloads/competition/competition_config.published.v1.schema.json` (Cần thêm) — `config_type`, `config_id`, `config_version_id`, `action`, `effective_from` |
| `audit.recorded` | Mọi thao tác ghi | envelope |
