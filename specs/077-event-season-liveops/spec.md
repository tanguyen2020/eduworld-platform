# Feature Specification: F077 — Event & Season live-ops: vòng đời sự kiện, community goal, season track, sự kiện trường

**Feature Branch**: `077-event-season-liveops` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `game` · **Mốc**: M3 · **Phụ thuộc**: F075
**Input**: Sự kiện live-ops có version (lễ hội, cuối tuần, chiến dịch học tập, boss đặc biệt, quest giới hạn thời gian, community goal) với vòng đời chạy Scheduled → Live → Ended → Archived, dừng khẩn cấp/tiếp tục/hủy có lý do; điểm sự kiện và mốc thưởng; community goal cộng từ event server đã kiểm chứng; season track miễn phí với điểm mùa có trần; sự kiện trường từ template `allowed_params`; quy tắc lành mạnh, song ngữ, ngân sách trang trí. FR nguồn: FR-EVT-001…008.

## Clarifications

### Session 2026-11-16

- Q: Lịch mùa lấy từ đâu khi module competition (F106, M4b) chưa có? → A: Season track chỉ mở khi nhận `season.started` (projection `ref.seasons`); trước F106, US4 kiểm bằng event fixture trong contract test. Track không tự tạo mùa.
- Q: Văn bản sự kiện trường kiểm tra từ nhạy cảm bằng gì? → A: Gọi module chat qua facade `chat` (operation `screenText` kiểm tra văn bản — "Cần thêm", [contracts](contracts/README.md) §6); module chat lỗi → `503`, không lưu (không fail-open).
- Q: Cập nhật community goal tới client bao lâu một lần? → A: Theo `community_goal_progress_flush_seconds` (CONFIG_DEFAULTS, EVENT_SEASON_SYSTEM §12). AsyncAPI ghi 10 giây — lệch, sửa AsyncAPI theo tài liệu sở hữu (delta).
- Q: Event Builder/Season Track UI? → A: F079 (workspace A); màn sự kiện trường workspace C → chưa có mã trong danh mục (ghi báo cáo). F077 chỉ backend.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F077-AC-06: bỏ điều kiện "thiếu MFA" và mã `403 MFA_REQUIRED`; phần permission/`azp` giữ nguyên. T015, plan, quickstart, contracts nhắc MFA là lịch sử. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Bỏ khái niệm lớp ([ADR-0095](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) mục 5, D5) — đối tượng sự kiện và đóng góp community goal đổi thế nào? → A: [Confirmed] `audience` chỉ còn tenant, trường, khối (`grade_levels`), môn; không còn `class_ids`/phạm vi `class` ([event.schema.json](../schemas/event.schema.json), `SchoolEventCreate/Update.audience` — đã áp 2026-11-21, WP-S). Đóng góp community goal hiển thị theo **khối của trường** (`CommunityGoalProgress.by_grade[] {grade_level, progress}` thay `by_class`, cờ cấu hình `show_grade_contributions` thay `show_class_contributions`): không tên, không xếp hạng cá nhân, khối có < `analytics_min_cell_size` (5) học sinh đóng góp bị ẩn (không trả dòng). Lọc `audience` theo `ref.students` (`school_id`, `grade_level` — cập nhật từ `student.placement_changed`, owner F010), không dùng `ref.enrollments`. `community_goal_contributions` ghi `grade_level` tại thời điểm đóng góp (ADR-0095 mục 2), không `class_id`.

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: F077-AC-06, Constitution, plan. Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

### Session 2026-11-23 (triển khai — owner chốt theo lệnh "chốt hết" — [Confirmed] 2026-11-23)

- **Bảng và khóa:** ledger `event_point_ledger`, `season_point_ledger`, `community_goal_contributions` **chưa partition** (giống `xp_ledger` F074, `currency_ledger` F075 — partition khi lớn), nên khóa idempotent là UNIQUE `(event_run_id | season_id | goal_id, source_event_id, student_id)` không kèm `created_at` (một sự kiện nguồn có thể mang nhiều học sinh — `party.activity_completed`, `match.completed`); cột ngày ICT `earned_on`/`contributed_on` cho trần ngày. Mảng `varchar(64)[]`/`smallint[]` của data-model là cột `jsonb` (CHECK là mảng, ≤ 50 phần tử — ent không có mảng PostgreSQL); `event_participation`, `community_goal_progress`, `season_track_progress` có khóa kỹ thuật + UNIQUE nghiệp vụ partial. Cột thêm: `events.school_id`, `events.school_params` (tham số trường đã điền), `event_runs.audience` (đối tượng của version ghim), `event_runs.school_id`, `cancelled_at`, `archived_at`, `event_participation.reached_milestones` (mốc đã đạt — không bao giờ giảm), `season_track_progress.auto_granted_tiers`, `ref.seasons.code/name/ended_at`. Một run hoạt động (scheduled/live/suspended) mỗi sự kiện (UNIQUE partial).
- **Nguồn điểm (danh mục đóng, sự kiện và mùa dùng chung):** `quest.completed`, `daily_quest.completed`, `weekly_quest.completed`, `learning_activity.completed` (`activity.graded` không gắn `rapid_guess_flagged`), `event_activity.completed` và `event_quest.completed` (quest là thành phần của run đang chạy), `event_boss.phase_cleared` (chấp nhận khi publish, chưa có nguồn tới F103), `discovery.found`, `coop.completed` (`party.activity_completed`), `match.completed`. Mỗi sự kiện nguồn cộng **một** lần mỗi run/mùa với nguồn có điểm cao nhất. Mã ngoài danh mục (đăng nhập, điểm danh, chuỗi ngày, thời gian online) → `422 VALIDATION_FAILED` (F077-AC-27).
- **Community goal:** metric → event: `correct_answers` (câu đúng lần đầu của `activity.graded`), `activities_completed`, `quests_completed`, `discoveries_found`, `coop_sessions`; `boss_phases_cleared` chưa có nguồn (F103). "Cờ `suspicious_speed`" = `rapid_guess_flagged` của `activity.graded`; **cờ gian lận của tài khoản chưa có nguồn trong module game** (ANTI_CHEAT chưa phát event) — khi có event cờ, consumer thêm điều kiện (Việc tiếp theo). Học sinh đóng góp **lần đầu sau khi** mốc đã đạt cũng nhận mốc đó (mọi học sinh có ≥ 1 đóng góp, đúng một lần — khóa grant theo goal, mốc, học sinh). Goal phạm vi `platform`: một dòng progress mỗi tenant, tổng chung đọc qua hàm `SECURITY DEFINER game.community_goal_platform_total` (chỉ số đếm); mốc toàn cục được job `goal-recompute` cấp cho người đóng góp của các tenant khác. Valkey: đọc bộ đếm, thiếu → dựng lại từ PostgreSQL; `goal-recompute` (5 phút [Proposed]) tính lại từ contributions, sửa progress (audit `community_goal.recomputed`) và ghi đè bộ đếm. `by_grade` chỉ khối 4–9 (EVENT_SEASON_SYSTEM §6), ẩn khối < `analytics_min_cell_size`.
- **Realtime:** `event.status.changed` đẩy tới học sinh **đang có phiên game** thuộc đối tượng (trường/khối) khi run live/suspended/resumed/ended/cancelled (manifest dựng lại phát `content.manifest_updated`); `event.community_goal.updated` tối đa một lần mỗi `community_goal_progress_flush_seconds` mỗi (goal, phạm vi) (Valkey `SET NX PX`), tới học sinh đang online của phạm vi — sửa AsyncAPI (bỏ "10 giây", room hub). Manifest tenant chứa mục `event` của run `live` (kèm `audience`); `getContentManifest` bỏ mục của đối tượng khác trường/khối học sinh (ETag thêm hậu tố khi lọc) — F077-AC-07.
- **Đổi luật khi đang chạy (AC-11):** version mới của run live/suspended có `effective_from` = `event_rule_change_effective` (04:00 ICT ngày sau, env giờ); scheduler ghim lại khi tới giờ; `end_at` mới sớm hơn → `422` (chỉ dừng khẩn cấp/hủy rút ngắn); run scheduled ghim ngay. Mốc đã đạt lưu `reached_milestones` nên ngưỡng tăng không làm mất mốc.
- **Trạng thái và lỗi:** suspended — thành phần ẩn (manifest, `getGameEvent`), không cộng điểm/đóng góp, claim → `409 CONFLICT`, join → `409`; join trước `starts_at` → `409 CONFLICT`; mốc/cấp đã nhận hoặc đã tự cấp → `409 CONFLICT` (cùng `Idempotency-Key` phát lại 200); chuyển trạng thái sai → `422 INVALID_STATE_TRANSITION` (spec thêm `422`, `428` cho suspend/resume/cancel); hủy sự kiện trường đã kết thúc → `409`. Thiếu key → `428 IDEMPOTENCY_KEY_REQUIRED` của middleware `pkg/httpx` dùng chung (AC-10 ghi `PRECONDITION_REQUIRED` — giống quyết định F075).
- **Phần thưởng:** sự kiện/template: `currency` (chỉ `xu`), `item`, `badge`, `title`, `collection_item` — **không `xp`** (XP chỉ từ reward rule); season track: `currency` (`xu`), `item` loại cosmetic, `badge`, `title` — `xp`, `collection_item` → `422`; vật phẩm phải đã publish (`422 ITEM_REF_INVALID`). Mọi thưởng qua `applyGrant` + `GrantApplierRegistry` (trần Xu/ngày giữ nguyên), khóa idempotent theo nguồn; `trigger_kind` thêm `event_participation` (`participation_rewards` cấp một lần khi tham gia), `event_milestone`, `community_goal`, `season_tier` (delta `reward.granted`).
- **Achievement (F075):** category `event`; trigger thêm `event.participation_started`, `event.milestone_claimed`, `season_track.tier_reached` (sự kiện của từng học sinh). `community_goal.completed` là sự kiện của phạm vi (không có học sinh) nên **không** là trigger (lệch contracts §5 — consumer achievement từ chối event không có học sinh).
- **`screenText` (chat facade):** thêm vào `chat.internal.v1.yaml` (`x-allowed-callers: [game]`), hiện thực bằng pipeline tên hiển thị của F116 trong module chat; `texts[]` ≤ 10 × `maxLength` 4000 (mô tả `TenantLocalizedText` tới 4000; contracts ghi 1000); trả `allowed` + chỉ số văn bản vi phạm, không từ khớp. Adapter game: deadline 800 ms, lỗi/không nối → `503 UNAVAILABLE`, không lưu.
- **Sự kiện trường:** `allowed_params` thuộc danh mục `description`, `audience.school_ids`, `audience.grade_levels`, `community_goal_target`, `quest_ids`, `challenge_ids`, `reward_option_id` (`name`, `starts_at`, `ends_at`, `audience` luôn điền); template bắt buộc có community goal (phạm vi trường/tenant), phần thưởng của `reward_option` cấp ở mốc cao nhất của goal; template chưa gán/không có → `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED`; trường ngoài tenant → `422` cùng mã. School Admin tạo cho trường của mình (mặc định); sự kiện toàn tenant cần grant toàn tenant; nhân sự giới hạn trường chỉ thấy sự kiện của trường mình (khác → `404`). Sửa chỉ khi run còn `scheduled` (`409`), `If-Match` = `version` của sự kiện (`412 VERSION_MISMATCH`), `updateSchoolEvent` nhận `Idempotency-Key` (breaking có ADR-0010 trong `breaking-allowlist.yaml`). Trạng thái `SchoolEvent`: suspended → `live`, archived → `ended`/`cancelled`. `audience.subject_ids` lọc nội dung, không lọc học sinh (không có ghi danh môn — ADR-0095).
- **Health/ngân sách:** `event.schema.json` thêm `decoration.bundle_size_bytes` (bắt buộc khi có `bundle_id`, kiểm ở domain); chồng trang trí: version hiện hành của sự kiện platform khác có run hoạt động, cùng vùng, giao thời gian → `422`. "Khóa vùng học tập": schema không có trường khóa — trường lạ bị `additionalProperties: false` từ chối (`422`). Quest thêm/ngày = quest thành phần có `recurrence` daily (hoặc `quest_type` daily) ≤ `event_extra_daily_quests_max`; quest không tồn tại → `422 ITEM_REF_INVALID`; challenge/boss chưa kiểm (F076/F103).
- **Scheduler/job:** `event-scheduler` mỗi 15 s dưới khóa Valkey `ew:lock:job:event-scheduler` (TTL 12 s); bảo đảm không trùng ở DB: khóa dòng + `status_version`; `goal-recompute` mỗi 5 phút [Proposed]. Tham số đọc env `EW_GAME_EVENT_*`, `EW_GAME_CGOAL_*`, `EW_GAME_SCHOOL_EVENT*`, `EW_GAME_ANALYTICS_MIN_CELL_SIZE` (CONFIG_DEFAULTS) tới khi có khóa platform settings (cùng cách F074/F075).
- **Season:** track mở khi `ref.seasons` `started` và đang trong lịch; `season.ended` tự cấp cấp đã đạt chưa nhận (`auto_granted_tiers`), lịch sử là các mùa đã kết thúc; CHECK DB `paid_track = false`. Seed `season-tracks.v1.json` gắn mùa `pre-season` của module competition.
- **Shop (F075):** `economysql.NoLiveOps` được thay bằng `liveopssql.Store`: mặt hàng gắn `event_id` bán/hiện khi sự kiện có run `live` của tenant; gắn `season_id` khi mùa `started` đang trong lịch.
- **Màn hình:** F077 chỉ backend; màn sự kiện trường (workspace C) chưa có mã trong danh mục, Event Builder/Season Track UI ở F079 (giữ nguyên Clarifications 2026-11-16).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Vòng đời chạy sự kiện và vận hành khẩn cấp (Priority: P1)

Đội live-ops publish sự kiện có lịch; hệ thống tự mở, tự kết thúc đúng giờ; khi có sự cố, vận hành dừng khẩn cấp, tiếp tục hoặc hủy kèm lý do mà không cắt lượt chơi đang dở.

**Why this priority**: Nền cho mọi thành phần sự kiện; sai giờ hoặc không dừng được là rủi ro vận hành lớn nhất.

**Independent Test**: Publish sự kiện với fake clock → `scheduled`; tới `start_at` → `live` + `event.started`; chạy 2 worker đồng thời → một chuyển trạng thái; `suspendEventRun` → thành phần ẩn, quest đang dở vẫn hoàn thành theo version ghim.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F077-AC-01 | KHI một version `events` có lịch được publish, module game PHẢI tạo event run `scheduled` và phát `event.scheduled`; KHI tới `start_at`/`end_at`, scheduler (khóa singleton) PHẢI chuyển `live`/`ended` trong ±1 phút, phát `event.started`/`event.ended`, và chạy lặp hay nhiều instance PHẢI không tạo chuyển trạng thái trùng | FR-EVT-001 · FR-EVT-003 · AC-EVT-002 | TC-EVT-002 |
| F077-AC-02 | KHI run chuyển `live`, `suspended`, `ended`, `cancelled`, module game PHẢI dựng lại content manifest của các tenant trong `audience` (thêm/gỡ thành phần sự kiện) và đẩy `event.status.changed` | FR-EVT-003 · [EVENT_SEASON_SYSTEM §5.2](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) | TC-EVT-003 |
| F077-AC-03 | KHI người có `game.event.operate` gọi `suspendEventRun`, `resumeEventRun` hoặc `cancelEventRun` với lý do 3–1000 ký tự và `Idempotency-Key`, module game PHẢI đổi trạng thái theo sơ đồ, ghi audit kèm lý do, phát `event.suspended`/`event.resumed`/`event.cancelled`; chuyển không hợp lệ PHẢI trả `422 INVALID_STATE_TRANSITION` | FR-EVT-003 · AC-EVT-003 | TC-EVT-003 |
| F077-AC-04 | TRONG KHI run `suspended`, module game PHẢI ẩn thành phần sự kiện, không cấp thưởng sự kiện mới, và cho lượt chơi đã bắt đầu kết thúc bình thường theo version đã ghim | FR-EVT-003 · ADR-0043 | TC-EVT-003 |
| F077-AC-05 | KHI tới `claim_until` (`end_at` + `event_claim_grace_hours`) hoặc run bị hủy, module game PHẢI tự cấp mọi mốc đã đạt chưa nhận (một lần) rồi chuyển `archived` | FR-EVT-003 · AC-EVT-012 | TC-EVT-012 |
| F077-AC-06 | NẾU người gọi `/game-admin/events/*` không có `game.event.operate` (thao tác) hoặc `game.liveops.read` (`getEventRunStats`), không có role platform hay không phải `eduworld-portal`, THÌ module game PHẢI trả `403 PERMISSION_DENIED` (không MFA — ADR-0094) và ghi `authz.denied` | AGENTS R20 · ADR-0027 | TC-TEN-004 |

---

### User Story 2 — Học sinh tham gia sự kiện, điểm và mốc thưởng (Priority: P1)

Học sinh thấy sự kiện đang chạy và sắp diễn ra trong phạm vi của mình, tham gia, tích điểm sự kiện từ hoạt động, nhận thưởng mốc; bỏ lỡ vài ngày không mất gì.

**Why this priority**: Giá trị trực tiếp cho học sinh; nơi dễ phát sinh dark pattern nhất.

**Independent Test**: `hs.an.3a` thấy sự kiện khối 3, `hs.dung.7a` không thấy (`404 EVENT_NOT_AVAILABLE`); hoàn thành quest sự kiện → điểm sự kiện; claim mốc 2 lần cùng key → một grant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F077-AC-07 | KHI học sinh gọi `listGameEvents`/`getGameEvent`, module game PHẢI chỉ trả run `live`, `ended` (trong thời gian nhận thưởng) hoặc `scheduled` trong `event_upcoming_visible_days` mà `audience` (tenant, trường, khối, môn — không có lớp, ADR-0095) chứa học sinh; ngoài phạm vi PHẢI trả `404 EVENT_NOT_AVAILABLE` và manifest PHẢI không chứa sự kiện đó | FR-EVT-001 · AC-EVT-001 | TC-EVT-001 |
| F077-AC-08 | KHI học sinh gọi `joinGameEvent` hoặc hoàn thành hoạt động đầu tiên thuộc sự kiện, module game PHẢI tạo đúng một tham gia, phát `event.participation_started`; bắt đầu hoạt động sự kiện sau `end_at` PHẢI trả `409 EVENT_ENDED` | FR-EVT-001 · [EVENT_SEASON_SYSTEM §4](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) | — |
| F077-AC-09 | KHI event server đã kiểm chứng thuộc `event_points.sources` của run `live` đến, module game PHẢI ghi điểm sự kiện idempotent theo `(source_event_id, student_id)`, không vượt `daily_cap` của sự kiện (`event_points_daily_cap`), và đẩy `event.progress.updated` | FR-EVT-001 · FR-EVT-004 | — |
| F077-AC-10 | KHI học sinh gọi `claimEventMilestone` với `Idempotency-Key` cho mốc đã đạt, module game PHẢI cấp phần thưởng mốc qua Reward Engine đúng một lần, phát `event.milestone_claimed`, ghi audit; mốc chưa đạt PHẢI trả `409 EVENT_MILESTONE_NOT_REACHED`; thiếu key PHẢI trả `428 PRECONDITION_REQUIRED` | FR-EVT-001 · AC-EVT-012 | TC-EVT-012 |
| F077-AC-11 | KHI version sự kiện mới đổi luật, phần thưởng hoặc lịch kết thúc khi run đang `live`, module game PHẢI áp từ mốc `event_rule_change_effective` và không bao giờ giảm điểm, tiến độ hay mốc đã đạt của học sinh | FR-EVT-003 · AC-EVT-004 | TC-EVT-004 |
| F077-AC-12 | HỆ THỐNG PHẢI cho mọi route `/game/events*`, `/game/community-goals/*`, `/game/season-track*` chỉ qua `game_player`: token nhân sự PHẢI nhận `403 GAME_ACCESS_STUDENT_ONLY`; `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH` | AGENTS R23 · NFR-SEC-006 | TC-TEN-001 |

---

### User Story 3 — Community goal toàn trường (Priority: P2)

Cả trường cùng góp sức thắp sáng đèn lồng; mỗi mốc đạt được đổi trang trí cho cả trường và mọi bạn đã góp ít nhất một lần đều nhận thưởng; không có bảng xếp hạng cá nhân.

**Why this priority**: Tạo gắn kết lành mạnh, phụ thuộc US1–US2.

**Independent Test**: Phát lại cùng `source_event_id` → tổng không đổi; vượt trần ngày → không cộng; xóa Valkey → tổng dựng lại khớp PostgreSQL; đạt 50% → học sinh có đóng góp nhận thưởng một lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F077-AC-13 | KHI event server đã kiểm chứng khớp `metric` của goal đến, module game PHẢI ghi đóng góp append-only theo `scope` (trường/tenant/platform) với khử trùng `source_event_id`, trần `community_goal_contribution_daily_cap` mỗi học sinh, và bỏ đóng góp từ attempt bị cờ `suspicious_speed`/tài khoản có cờ gian lận | FR-EVT-004 · AC-EVT-005 | TC-EVT-005 |
| F077-AC-14 | HỆ THỐNG PHẢI coi bảng đóng góp PostgreSQL là nguồn sự thật: bộ đếm Valkey mất PHẢI được dựng lại đúng tổng; job tính lại định kỳ PHẢI sửa lệch cache | FR-EVT-004 · AGENTS R9 | TC-EVT-005 |
| F077-AC-15 | KHI tổng đạt một mốc, module game PHẢI phát `community_goal.milestone_reached` (và `community_goal.completed` ở 100%) một lần, cấp thưởng mốc đúng một lần cho mọi học sinh trong phạm vi có ≥ 1 đóng góp, và không cấp cho học sinh không đóng góp | FR-EVT-004 · AC-EVT-006 | TC-EVT-006 |
| F077-AC-16 | KHI học sinh gọi `getCommunityGoal`, module game PHẢI trả tiến độ phạm vi của học sinh, các mốc, và đóng góp theo khối của trường (`by_grade`) chỉ khi sự kiện bật `show_grade_contributions`, ẩn khối có < 5 học sinh đóng góp (ADR-0095); PHẢI không trả xếp hạng hay danh tính học sinh; cập nhật realtime `event.community_goal.updated` không nhanh hơn `community_goal_progress_flush_seconds` | FR-EVT-004 · AGENTS R16 | — |

---

### User Story 4 — Season track miễn phí (Priority: P2)

Mỗi mùa có một đường phần thưởng miễn phí; học sinh tích điểm mùa từ nhiều hoạt động có trần mỗi ngày, bấm nhận thưởng từng cấp; hết mùa, thưởng chưa nhận tự vào kho.

**Why this priority**: FR-EVT-005 (P4 trong PRD); phụ thuộc lịch mùa của F106.

**Independent Test**: Phát `season.started` fixture → track mở; điểm mùa vượt trần ngày bị cắt; claim cấp 2 lần cùng key → một grant; phát `season.ended` → thưởng chưa nhận tự cấp.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F077-AC-17 | KHI nhận `season.started`/`season.ended`, module game PHẢI cập nhật projection mùa idempotent và mở/đóng season track đã publish của mùa đó | FR-EVT-005 · [EVENT_SEASON_SYSTEM §7](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) | TC-EVT-007 |
| F077-AC-18 | KHI event nguồn thuộc `season_point_sources` đến trong mùa đang mở, module game PHẢI ghi điểm mùa idempotent theo `(source_event_id, student_id)`, không vượt `season_points_daily_cap`, tính cấp theo `season_track_points_per_tier`, phát `season_track.tier_reached` và đẩy `season.track.updated` | FR-EVT-005 · AC-EVT-007 | TC-EVT-007 |
| F077-AC-19 | KHI học sinh gọi `claimSeasonTier` với `Idempotency-Key` cho cấp đã đạt, module game PHẢI cấp thưởng đúng một lần và phát `season_track.reward_claimed`; cấp chưa đạt PHẢI trả `409 SEASON_TIER_NOT_REACHED` | FR-EVT-005 · AC-EVT-007 | TC-EVT-007 |
| F077-AC-20 | KHI mùa kết thúc, module game PHẢI tự cấp mọi cấp đã đạt chưa nhận (`auto_granted = true`), giữ lịch sử cấp (`listSeasonTrackHistory`) và đưa điểm mùa về 0 cho mùa mới; NẾU version `season-tracks` có `paid_track` khác `false` hoặc phần thưởng ngoài cosmetic/Xu/huy hiệu/danh hiệu, THÌ PHẢI từ chối `422 VALIDATION_FAILED` | FR-EVT-005 · ADR-0049 | TC-EVT-007 |

---

### User Story 5 — Sự kiện của trường từ template (Priority: P2)

School Admin/Tenant Admin tạo "Tuần lễ Toán học" của trường từ template platform đã gán, chỉ điền tham số cho phép; tên có từ nhạy cảm bị chặn.

**Why this priority**: FR-EVT-006; tenant không được cấu hình game ngoài template (ADR-0027).

**Independent Test**: `ad.truong.a` tạo sự kiện với tham số ngoài template → `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED`; sự kiện thứ (`school_events_concurrent_per_tenant` + 1) → bị chặn; `ad.truong.b` không thấy sự kiện của trường A.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F077-AC-21 | KHI người có `lms.school_event.write` gọi `listEventTemplates`, module game PHẢI chỉ trả template đã publish và được gán cho tenant của phiên | FR-EVT-006 · ADR-0029 | TC-TEN-019 |
| F077-AC-22 | KHI gọi `createSchoolEvent`/`updateSchoolEvent` (`If-Match`) với `Idempotency-Key`, module game PHẢI tạo/sửa sự kiện tenant từ template, ghi audit; NẾU có tham số ngoài `allowed_params`, quest/challenge/phần thưởng ngoài danh sách template, thời lượng vượt `school_event_max_duration_days`, hay đối tượng ngoài phạm vi trường/tenant của người gọi, THÌ PHẢI trả `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED` | FR-EVT-006 · AC-EVT-008 | TC-EVT-008 |
| F077-AC-23 | NẾU tên/mô tả chứa từ nhạy cảm theo danh mục hiệu lực, THÌ module game PHẢI trả `422 VALIDATION_FAILED` không nêu từ khớp; NẾU dịch vụ kiểm tra không khả dụng, THÌ PHẢI trả `503 UNAVAILABLE` và không lưu; `vi` bắt buộc, `en` tùy chọn; chuỗi vượt `maxLength` PHẢI trả `422` | FR-EVT-006 · FR-EVT-008 · AGENTS R15 | TC-EVT-008 |
| F077-AC-24 | NẾU tenant đã có `school_events_concurrent_per_tenant` sự kiện `scheduled`/`live`, THÌ tạo thêm PHẢI trả `409 CONFLICT`; `cancelSchoolEvent` PHẢI yêu cầu lý do, ghi audit, giữ phần thưởng đã đạt | FR-EVT-006 · FR-EVT-003 | TC-EVT-008 |
| F077-AC-25 | HỆ THỐNG PHẢI giới hạn `/lms/school-events*` theo tenant phiên và scope trường của UserRole: sự kiện của trường/tenant khác PHẢI trả `404`; role không có `lms.school_event.write` (vd. `academic_manager`) PHẢI nhận `403 PERMISSION_DENIED`; học sinh PHẢI nhận `403 PORTAL_ACCESS_STAFF_ONLY` | FR-EVT-006 · NFR-SEC-006 | TC-TEN-003 |

---

### User Story 6 — Quy tắc lành mạnh, song ngữ, ngân sách trang trí (Priority: P3)

Sự kiện không ép online, không đếm ngược gây áp lực, không khóa học tập; nội dung platform đủ hai ngôn ngữ; trang trí vừa máy tầm trung.

**Why this priority**: FR-EVT-007, FR-EVT-008 — chặn ở bước publish.

**Independent Test**: Publish sự kiện platform thiếu `en` → `422 TRANSLATION_INCOMPLETE`; lễ hội 5 ngày → `422`; bundle trang trí vượt ngân sách → `422`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F077-AC-26 | NẾU version `events` platform thiếu `en` ở tên, mô tả, tagline hoặc tên phần thưởng, THÌ submit/publish PHẢI trả `422 TRANSLATION_INCOMPLETE`; API học sinh PHẢI trả văn bản theo `Accept-Language` | FR-EVT-008 · AC-EVT-010 | TC-EVT-010 |
| F077-AC-27 | NẾU version `events` là `world_festival` ngắn hơn `event_festival_min_duration_days`, có phần thưởng điểm danh/chuỗi ngày, `eligibility` dùng level cao cho sự kiện học tập, thêm quest/ngày quá `event_extra_daily_quests_max`, hoặc khóa vùng học tập, THÌ module game PHẢI từ chối `422 VALIDATION_FAILED` | FR-EVT-007 · NFR-SAFE-003 · AC-EVT-009 | TC-EVT-009 |
| F077-AC-28 | NẾU gói trang trí vượt `event_decoration_bundle_max_mb`, thiếu `low_preset_bundle_id`, hoặc một vùng có hơn một bộ trang trí sự kiện chồng thời gian, THÌ publish PHẢI trả `422 VALIDATION_FAILED` | FR-EVT-008 · ADR-0057 | — |
| F077-AC-29 | HỆ THỐNG PHẢI không gửi thông báo đẩy ngoài game cho sự kiện và không trừ điểm/thưởng khi học sinh bỏ lỡ ngày | FR-EVT-007 · AC-EVT-009 | TC-EVT-009 |

### Edge Cases

- NẾU worker scheduler chết đúng lúc `start_at`, THÌ instance khác giữ khóa singleton chuyển trạng thái khi chạy lại (trễ tối đa một chu kỳ, vẫn trong ±1 phút).
- NẾU sự kiện bị hủy trước `start_at`, THÌ chuyển `cancelled → archived`, không phát thưởng.
- NẾU học sinh chuyển trường giữa sự kiện trường, THÌ đóng góp cũ giữ ở trường cũ; phạm vi mới tính từ lúc chuyển.
- NẾU `season.ended` đến trước `season.started` (lệch thứ tự), THÌ projection theo `source_version` bỏ bản cũ.
- NẾU community goal `platform`, THÌ mỗi tenant chỉ thấy tổng chung và đóng góp của tenant mình.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-EVT-001 | Loại sự kiện, mô hình definition/version/run, thành phần tham chiếu, điểm sự kiện | [EVENT_SEASON_SYSTEM §3–4](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) |
| FR-EVT-002 | Backend cho Event Builder: validate, run stats `getEventRunStats`, vận hành (UI: F079) | EVENT_SEASON_SYSTEM §14 · [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-EVT-003 | Vòng đời run, dừng khẩn cấp/tiếp tục/hủy, không giảm tiến độ | EVENT_SEASON_SYSTEM §5 |
| FR-EVT-004 | Community goal: nguồn kiểm chứng, trần, không xếp hạng, thưởng mốc | EVENT_SEASON_SYSTEM §6 |
| FR-EVT-005 | Season track miễn phí, điểm mùa có trần, tự cấp thưởng chưa nhận | EVENT_SEASON_SYSTEM §7 |
| FR-EVT-006 | Sự kiện trường từ template `allowed_params` | EVENT_SEASON_SYSTEM §10 · [ADR-0029](../../docs/adr/ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md) |
| FR-EVT-007 | Quy tắc lành mạnh | EVENT_SEASON_SYSTEM §9 |
| FR-EVT-008 | Song ngữ, ngân sách trang trí | EVENT_SEASON_SYSTEM §11–12 · [ADR-0056](../../docs/adr/ADR-0056-game-song-ngu-viet-anh.md) |

### Key Entities

- **EventDefinition / EventVersion**: sự kiện platform hoặc tenant, nội dung bất biến sau publish.
- **EventTemplate**: mẫu sự kiện platform với `allowed_params`, gán cho tenant.
- **EventRun**: một lần chạy theo lịch, trạng thái chạy.
- **EventParticipation / EventPointEntry**: tham gia và điểm sự kiện của học sinh.
- **CommunityGoal / Contribution / Progress**: mục tiêu chung, đóng góp append-only, tổng theo phạm vi.
- **SeasonTrack (version) / SeasonPointEntry / SeasonTrackProgress**: đường thưởng mùa, điểm mùa, cấp và cấp đã nhận.
- **Season (projection)**: lịch mùa từ module competition.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `event_claim_grace_hours` | Thời gian nhận thưởng sau `end_at` |
| `event_festival_min_duration_days` | Thời lượng tối thiểu lễ hội |
| `event_extra_daily_quests_max` | Quest sự kiện thêm/ngày |
| `event_points_daily_cap` | Trần điểm sự kiện mặc định |
| `event_upcoming_visible_days` | Hiển thị "Sắp diễn ra" |
| `event_rule_change_effective` | Thời điểm áp luật mới khi đang chạy |
| `community_goal_contribution_daily_cap` | Trần đóng góp/học sinh/ngày |
| `community_goal_progress_flush_seconds` | Nhịp đẩy tiến độ |
| `season_track_tiers`, `season_track_points_per_tier`, `season_points_daily_cap`, `season_point_sources`, `season_unclaimed_rewards` | Season track |
| `school_event_max_duration_days`, `school_events_concurrent_per_tenant` | Sự kiện trường |
| `event_decoration_bundle_max_mb` | Ngân sách trang trí |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Phần thưởng chỉ do server, idempotent, qua sổ cái | ADR-0049 · AGENTS R5, R10 |
| Community goal: Valkey chỉ là cache | AGENTS R9 · NFR-AVAIL-003 |
| Không streak, không push ngoài game, không track trả phí | AGENTS R19 · NFR-SAFE-003 |
| Thao tác vận hành chỉ qua gateway nội bộ + role platform; tenant chỉ dùng template | AGENTS R20 · ADR-0027 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% chuyển trạng thái run xảy ra trong ±1 phút quanh `start_at`/`end_at` qua 100 lần chạy giả lập, 0 chuyển trùng.
- **SC-002**: Dừng khẩn cấp ẩn thành phần sự kiện với học sinh mới trong ≤ 30 giây, 0 lượt chơi đang dở bị cắt.
- **SC-003**: Xóa Valkey giữa sự kiện → tổng community goal khớp PostgreSQL 100%.
- **SC-004**: 0 học sinh mất điểm/thưởng đã đạt sau mọi lần đổi version, hủy hay hết mùa.
- **SC-005**: 0 sự kiện platform publish thiếu bản `en`; 0 sự kiện trường có tham số ngoài template.

## Ngoài phạm vi

- Event Builder, lịch sự kiện, Season Track UI (workspace A), giám sát sự kiện (E) → F079; màn sự kiện trường (workspace C) → chưa có mã trong danh mục.
- HTTP workflow `/game-admin/events`, `event-templates`, `season-tracks` (soạn/duyệt/publish) → F078.
- Lịch mùa, soft reset, Competition Points → F106; bảng xếp hạng sự kiện → F106.
- Quest sự kiện (`special_event`/`seasonal`) → F072; boss đặc biệt → F103; shop sự kiện (mặt hàng gắn `event_id`/`season_id`) → F075.
- Trang trí/hiệu năng trong Godot (AC-EVT-011) → nhóm F; chỉ số live-ops → F080.

## Assumptions

- Endpoint kiểm tra văn bản của module chat chưa có trong spec internal (delta) [Confirmed].
- Sự kiện tenant thuộc phạm vi một trường (School Admin) hoặc toàn tenant (Tenant Admin) theo scope UserRole [Confirmed].
- Ngày của trần điểm theo lịch ICT như F074 [Confirmed].
- Season track chỉ hoạt động sau khi F106 phát `season.started`; trước đó kiểm bằng fixture.
