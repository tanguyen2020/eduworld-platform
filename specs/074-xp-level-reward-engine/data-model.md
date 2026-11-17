# Data Model: F074 — XP ledger, level, đường cong XP, danh hiệu, Reward Engine có cap

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§2.8, §2.8a, §2.13, §3.13)

Database `eduworld_game`. Cột chung theo loại bảng (T/TA/G) theo DATABASE_DESIGN §2.

## 1. Entity

### XpLedgerEntry — `gamification.xp_ledger` (Có sẵn §2.13 + §2.8a; Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `xp_entry_id` | UUIDv7 | PK | |
| `tenant_id`, `student_id`, `school_id` | uuid | NOT NULL (school_id NULL với tenant chung) | RLS |
| `amount` | int | NOT NULL; CHECK `amount >= 0 OR reason_code = 'adjustment'` | 0 khi bị cắt hết bởi cap |
| `reason_code` | `varchar(64)` (`MaxLen(64)`) | CHECK `reward`/`adjustment` | **Thêm cột** (ADR-0049); `adjustment` do F075 ghi |
| `source_type` | `varchar(64)` | NOT NULL | `activity`, `quest`, `minigame`, `lesson`, `objective`, `world`, `level` |
| `source_code` | `varchar(64)` | NOT NULL | Catalog PROGRESSION_SYSTEM §3.1 |
| `source_id` | uuid | NOT NULL | attempt / quest_progress / objective / interactable (UUIDv5 khi id gốc là chuỗi) |
| `content_version_id` | uuid | NULL | **Thêm cột** — khóa đếm giảm dần theo content version |
| `rule_id`, `rule_version` | uuid, int | NOT NULL | |
| `capped` | bool | NOT NULL DEFAULT false | |
| `idempotency_key` | `varchar(64)` | UNIQUE | hex SHA-256 của `(student_id, source_type, source_id, rule_id)` |
| `created_at` | timestamptz | NOT NULL | giờ server |

**Index**: UNIQUE `(student_id, source_type, source_id, rule_id)`; `(tenant_id, student_id, created_at DESC)`; `(tenant_id, student_id, content_version_id, created_at)`. `REVOKE UPDATE, DELETE, TRUNCATE` cho `game_app`.

### DailyCapCounter — `gamification.xp_daily_caps` (Có sẵn §2.13, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id`, `student_id` | uuid | NOT NULL | |
| `cap_date` | date | NOT NULL | ngày ICT [Proposed]; DATABASE_DESIGN ghi `date` → đổi tên cột tránh từ khóa |
| `bucket` | `varchar(64)` | NOT NULL | `xp:total`, `xp:src:<source_type>`; F075 dùng `currency:xu` |
| `granted` | int | NOT NULL CHECK ≥ 0 | |

**Khóa**: PK `(student_id, cap_date, bucket)`; khóa dòng `FOR UPDATE` trong transaction cấp thưởng.

### RewardRule / RewardRuleVersion — `gamification.reward_rules`, `gamification.reward_rule_versions` (Có sẵn §2.13, G)

- `reward_rules`: `reward_rule_id` PK, `code varchar(64)` UNIQUE partial, `current_version_id`, `latest_version_no int`, `deleted_at`, `deleted_by`.
- `reward_rule_versions`: `reward_rule_version_id` PK, `reward_rule_id`, `version int`, `status varchar(64)`, `payload jsonb` (`reward-rule.schema.json`, CHECK ≤ 65536 byte), **Thêm cột** `trigger_event_type varchar(64)`, `source_code varchar(64)` (index chọn rule), `effective_from timestamptz`, cột phát hành theo §3.13 (`change_note varchar(500)`, `publish_at`, `snapshot_key varchar(512)`, `snapshot_sha256 varchar(64)`, `based_on_version_id`). UNIQUE `(reward_rule_id, version)`; index `(trigger_event_type) WHERE status = 'published'`.

### RewardGrant — `gamification.reward_grants` (Có sẵn §2.13, T; Thêm cột)

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `reward_grant_id` | UUIDv7 PK | |
| `tenant_id`, `student_id`, `school_id` | uuid | |
| `rule_version_id` | uuid NULL | NULL với thưởng lên cấp |
| `trigger_event_id` | uuid NOT NULL | `event_id` gốc; lên cấp: UUIDv5 `(curve_id, level, student_id)` |
| `trigger_kind` | `varchar(64)` | **Thêm cột** — `event_rule`, `level_up` (F075–F077 thêm `achievement`, `collection`, `event_milestone`, `season_tier`…) |
| `grants` | jsonb CHECK ≤ 16384 byte | danh sách `{type, amount?, item_id?, currency_code?}` |
| `idempotency_key` | `varchar(64)` UNIQUE | **Thêm cột** — hex SHA-256 |
| `created_at` | timestamptz | |

UNIQUE `(trigger_event_id, rule_version_id, student_id)` (giữ theo DATABASE_DESIGN) + UNIQUE `idempotency_key`.

### ProgressionCurve — `game.progression_curves`, `game.progression_curve_versions` (Có sẵn §2.8a, G)

- `progression_curves`: `curve_id` PK, `code varchar(64)`, `current_version_id`, `latest_version_no`, `deleted_at`, `deleted_by`.
- `progression_curve_versions`: `curve_version_id` PK, `curve_id`, `version int`, `status varchar(64)`, `max_level int`, `formula jsonb`, `level_overrides jsonb`, `titles jsonb`, `level_floor_protection bool`, `payload jsonb` (đủ `progression-curve.schema.json`), cột phát hành §3.13. UNIQUE `(curve_id, version)`.

### ProgressionLevel — `game.progression_levels` (Có sẵn §2.8a, G; Thêm cột)

`curve_version_id`, `level int`, `xp_required int` (XP cần để lên level kế, NULL tại `max_level`), **Thêm cột** `xp_cumulative bigint` (XP tích lũy để đạt level), `title_code varchar(64)`, `rewards jsonb` (CHECK ≤ 16384). PK `(curve_version_id, level)`. Sinh khi publish; tên cột theo PROGRESSION_SYSTEM §6 và DATABASE_DESIGN §2.8a (RB-308). API `Progression.xp_to_next` tính từ `xp_required` của level hiện tại trừ `xp_in_level`.

### LevelHistory — `game.level_history` (Có sẵn §2.8a, append-only)

`level_history_id` UUIDv7 PK, `tenant_id`, `student_id`, `from_level int`, `to_level int`, `curve_version int`, `trigger_xp_entry_id uuid` NULL (NULL khi do `curve-rollout`), `reached_at`. Index `(tenant_id, student_id, reached_at DESC)`; REVOKE UPDATE/DELETE.

### GameProfile — `game.game_profiles` (Có sẵn §2.8, F070 tạo)

F074 ghi `level int`, `xp_total bigint`, `curve_version int`, `title_code varchar(64)` trong cùng transaction với ledger. Thêm **`grade_transition_year varchar(16)`** (Mới — năm học đã áp `ApplyGradeTransition`, idempotent).

## 2. Vòng đời / trạng thái

Reward rule và curve dùng workflow nội dung chung (CONTENT_MODEL §6): `draft → in_review → approved → published → archived`. Bút toán ledger không có trạng thái (bất biến).

## 3. Quy tắc kiểm tra dữ liệu

- `xp_total` = Σ `xp_ledger.amount`; `level` = level lớn nhất có `xp_cumulative ≤ xp_total`, nhưng không nhỏ hơn level hiện tại (`level_floor_protection`).
- Cap: Σ `granted` của bucket `xp:total` trong `cap_date` ≤ `xp_daily_cap` ([PROGRESSION_SYSTEM §3.2](../../docs/01-game-design/PROGRESSION_SYSTEM.md)).
- Đếm lặp: số bút toán cùng `(student_id, content_version_id, source_code)` trong `xp_repeat_multipliers` cửa sổ → chỉ số hệ số.
- Ngưỡng curve tăng đơn điệu; `titles[0].from_level = 1`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `xp.awarded` | Mỗi bút toán XP (kể cả `capped`, amount 0) | `specs/events/payloads/game/xp.awarded.v1.schema.json` — Cần thêm |
| `level.reached` | Mỗi cấp đạt được | `specs/events/payloads/game/level.reached.v1.schema.json` — Cần thêm |
| `reward.granted` | Mỗi RewardGrant | `specs/events/payloads/game/reward.granted.v1.schema.json` — Cần thêm |
