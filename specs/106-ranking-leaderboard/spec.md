# Feature Specification: F106 — Ranking Glicko-2, tier, season và 12 bảng xếp hạng

**Feature Branch**: `106-ranking-leaderboard` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` · **Mốc**: M4b · **Phụ thuộc**: F105
**Input**: Sau mỗi trận ranked, server cập nhật Game Rating Glicko-2 theo (học sinh, mode, môn, mùa), quy ra tier thân thiện, cộng Competition Points; bảng xếp hạng theo metric × phạm vi × kỳ với quy tắc an toàn trẻ em; mùa giải 8 tuần có soft reset. FR nguồn: FR-RNK-001…004, FR-LDB-001…005; US-LDB-001.

## Clarifications

### Session 2026-11-16

- Q: FR-RNK-003 chỉ ẩn số rating với lớp 1–5, COMPETITION_SYSTEM §8.2 ẩn với mọi khối — theo bên nào? → A: Theo tài liệu sở hữu (COMPETITION_SYSTEM §8.2, tham số `tiers` "ẩn số rating thô với mọi học sinh"); chặt hơn PRD nên không vi phạm FR.
- Q: "13 bảng xếp hạng" ánh xạ thế nào? → A: Các loại của FR-LDB-001 = tổ hợp metric × scope × period × filter của `LeaderboardDefinition`; danh mục mặc định ở [COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#92-danh-mục-bảng-mặc-định-confirmed) là seed. *(Số loại đổi từ 13 xuống 12 ở Session 2026-11-21 — bỏ Class theo ADR-0095.)*
- Q: `effort_points` lấy từ đâu? → A: Từ event học tập của module learning (`activity.graded`) đã loại attempt gắn cờ `rapid_guess`; cần bổ sung trường vào payload (contracts §6).
- Q: Opt-out ẩn tên khỏi bảng ngoài trường ([COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em), [Confirmed]) có làm không? → A: Không trong F106 (chưa có contract); mặc định đã ẩn khỏi bảng ngoài trường qua `allow_cross_school_listing = false`.

### Session 2026-11-17

- Q: Tên và tham số sau đối soát (RB-412, RB-415)? → A: Bảng `rating_changes`, khóa `leaderboard_id`, key Valkey theo VALKEY_DESIGN; lớp 6–9 thấy ±3 quanh mình. Phần thưởng thi đấu (mùa, giải, thử thách) là cấu hình `competition-rewards` (F108, quyền `competition.config.reward.write`).

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Bỏ khái niệm lớp ảnh hưởng bảng xếp hạng thế nào? → A: Theo [ADR-0095 mục 5 (D5)](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md#5-game-xã-hội-chat-d2-d5-d7-confirmed--owner-2026-11-21): bỏ scope `class`, metric `class_goal`, các bảng "Mục tiêu lớp", "Lớp em tuần này", "Nhiệm vụ của lớp" và cài đặt trường `class_leaderboard_enabled`; thay bằng **mục tiêu tập thể của khối trong trường** — metric `grade_goal`, scope school + grade, kỳ tuần, khối 1–9: chỉ tiến độ chung `current/target` và phần đóng góp của chính em, không thứ hạng cá nhân, không nickname/tên học sinh khác, ô đóng góp < 5 học sinh bị ẩn ([COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#92-danh-mục-bảng-mặc-định-confirmed), FR-LDB-005).
- Q: Còn bao nhiêu loại bảng? → A: **12** loại của FR-LDB-001 (Global, School, Grade, Subject, Topic, Game Mode, Friends, Team, Daily, Weekly, Monthly, Season); danh mục mặc định còn 8 dòng (Mục tiêu của khối, Đấu trường trường, Đấu trường môn, Bạn bè, Đội, Toàn hệ thống, Xếp hạng rating, Giải đấu). Tiêu đề feature đổi "13" → "12".
- Q: Khối của học sinh lấy từ đâu? → A: `ref.students(school_id, grade_level)` cập nhật theo `student.placement_changed` (ADR-0095 mục 2, 7) thay `ref.enrollments`; đóng góp ghi `grade_level` **tại thời điểm** ghi điểm (không suy ngược từ khối hiện tại).
- Q: Lớp 1–3 còn "Bạn nổi bật tuần này"? → A: Không — `grade_goal` không hiển thị tên (ADR-0095) nên bỏ phần nickname ngẫu nhiên gắn với mục tiêu lớp cũ; lớp 1–3 chỉ có mục tiêu của khối [Confirmed — WP-S, chốt ở COMPETITION_SYSTEM §9.4].
- Q: Nhân sự xem bảng bằng quyền nào? → A: `lms.student.read` (đổi tên từ `lms.class.read`, ADR-0095 mục 6), phạm vi trường của UserRole; xem bảng trường/khối và mục tiêu của khối, không có bảng lớp.
- Q: Ngưỡng ẩn ô đóng góp? → A: `analytics_min_cell_size` (5, CONFIG_DEFAULTS) — cùng ngưỡng FR-ANL-016 [Confirmed — dùng lại tham số sẵn có thay vì thêm tham số mới].

### Session 2026-11-22 (triển khai — [Confirmed], owner chốt 2026-11-20)

Quyết định của agent khi tài liệu thiếu/mâu thuẫn, theo hướng bảo thủ (an toàn trẻ em trước), không mở rộng phạm vi (R34):

- Q: Bảng scope `global` (liên tenant) đọc thế nào khi học sinh chỉ có `TenantScope`? → A: **Không phục vụ trong F106**: đọc dữ liệu học sinh của tenant khác cần projection liên tenant và ADR (R8 — chỉ `system_admin` liên tenant). Definition `global_season` vẫn hợp lệ, seed `status = retired` (platform chưa bật), `listLeaderboards` không liệt kê, `getLeaderboard` → `404`. Scope `team`, `tournament` cũng hợp lệ nhưng chưa phục vụ (chờ thực thể đội/giải của F107) → `404`. Bảng "ngoài trường" trong tenant đơn vị (scope `tenant`) phục vụ đầy đủ, chỉ gồm trường có `allow_cross_school_listing = true`.
- Q: Bảng `tenant`/`global` có trộn mọi khối? → A: Không — chia theo khối của người xem (scope key `<tenant_id>:g<khối>`, `g<khối>`), như ràng buộc công bằng khối (COMPETITION_SYSTEM §5).
- Q: `team_points` lấy từ đâu khi chưa có thực thể đội bền vững? → A: Competition Points mà học sinh nhận trong trận `team_battle` rated (`leaderboard_contributions.source_type = team_match`); bảng "Đội" = scope `school`, lớp 6–9. Filter (`subject_id`, `topic_id`, `mode`) chỉ cho metric có trong sổ cái: `competition_points`, `rating` (`topic_id` chỉ `competition_points`); definition khác có filter bị từ chối.
- Q: Nhân sự chọn khối của bảng khối thế nào (contract chỉ có `scope_id`)? → A: Thêm query `grade_level` (1–9) cho `getLeaderboard`/`getLeaderboardHistory`; nhân sự bắt buộc `scope_id` (trường) và `grade_level` (bảng khối) — thiếu → `422`; học sinh luôn là trường + khối của mình (khác → `404`). `?show_real_names` **không làm** trong F106: module competition không có tên thật (chỉ projection nickname), cần facade identity — nhân sự cũng chỉ thấy nickname.
- Q: "Hoạt động ôn tập do hệ thống gợi ý hoặc nhà trường giao" nhận diện thế nào? → A: `activity.graded` có `assignment_id` (nhà trường giao) hoặc `recommendation_id` (hệ thống gợi ý) → +2. `activity.graded.v1` thêm tùy chọn `rapid_guess_flagged` (vắng = `false`) và `recommendation_id` (vắng = `null`); module learning **chưa điền** hai trường này — cho tới khi F031/F032 điền, attempt gắn cờ rapid guess chưa bị loại khỏi `effort_points` (theo dõi ở mục Việc tiếp theo của báo cáo F106).
- Q: Mục tiêu tuần của `grade_goal`? → A: `display.grade_goal_target = 300` trong seed `leaderboard_definitions.json` [Confirmed — F108 chỉnh].
- Q: "Nhóm X%" lớp 4–5? → A: Nhóm 10/25/50/100% (`percentile_bucket`); người xếp cuối không hiển thị cho người khác (vẫn thấy chính mình); `leaderboard.position.updated` không làm (không thông báo "bị vượt").
- Q: `analytics_min_cell_size` đọc từ đâu (module competition không đọc platform settings của analytics)? → A: Ranking Rules mang `analytics_min_cell_size: 5` (giá trị = CONFIG_DEFAULTS, tài liệu sở hữu vẫn là GAME_ANALYTICS) cùng `friends_max: 200`.
- Q: "Thắng đối thủ tier cao hơn" ở team battle? → A: So thứ hạng tier của mình với **trung bình thứ hạng tier** đội đối phương trước trận (từ `rating_changes.before` hoặc rating hiện tại); học sinh hoặc đối thủ còn provisional (không có tier) → không có +5.
- Q: Soft reset khi mùa mới bắt đầu? → A: Job `season-scheduler` tạo rating mùa mới (soft reset) cho mọi rating của mùa trước rồi mới chuyển mùa sang `active`; trận đầu tiên của học sinh chưa có rating mùa mới dùng soft reset từ rating mùa gần nhất. Mùa đang `active` kết thúc khi qua `ends_at` **hoặc** khi một mùa `scheduled` tới `starts_at` (pre-season seed `ends_at` 2099 nên kết thúc khi F108 publish mùa đầu).
- Q: Snapshot partition theo tháng? → A: Chưa partition trong F106 (bảng TA + index lịch sử; giữ 3 năm theo DATABASE_DESIGN) [Confirmed — partition khi khối lượng cần, migration expand riêng].
- Q: `xp`, `quest_completed` khi payload `xp.awarded`/`quest.completed` chưa có (owner F074/F072)? → A: Metric và definition được hỗ trợ (bảng `friends_xp_weekly` seed), consumer `xp.awarded`/`quest.completed` **không** tạo trong F106 (không tự viết schema của feature khác) — bảng rỗng tới khi các feature đó phát event.
- Q: Đấu trường môn và Xếp hạng rating (cần `subject_id` cụ thể)? → A: Engine hỗ trợ (filter môn/mode, rating theo mode × môn), nhưng **không seed** — F108 publish definition theo từng môn.

### Session 2026-11-26 (follow-up gateway — owner "chốt hết", [Confirmed])

- Q: `GET /leaderboards` (`listLeaderboards`) không có route trên gateway public — `/api/v1/leaderboards/*` không khớp chính URI collection? → A: [Confirmed] Thêm `/api/v1/leaderboards` vào route người chơi `player` (cùng cách `/api/v1/notifications` + `/api/v1/notifications/*`): gateway public, `cors: game`, OIDC, `X-Tenant-Id` bắt buộc, rate `api`; module competition vẫn qua middleware `game_player` (R23). Test: `deploy/apisix/scripts/check-routes.test.mjs`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Rating Glicko-2 sau trận ranked (Priority: P1) 🎯 MVP

Sau trận 1v1 Toán ranked, rating của hai học sinh được cập nhật đúng một lần theo Glicko-2; trận bot, thách đấu bạn, trận hủy không làm đổi rating.

**Why this priority**: Matchmaking (F104) cần rating thật để ghép công bằng; seeding giải (F107) dùng rating.

**Independent Test**: Unit test vector Glickman (r=1500, RD=200 vs 3 đối thủ → r'≈1464.06, RD'≈151.52, sai số < 0.01); phát `match.completed` hai lần → `rating_changes` 1 dòng mỗi học sinh.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F106-AC-01 | KHI nhận `match.completed` có `rated = true` (mode `battle_1v1`, `team_battle`, `timed_quiz`), module competition PHẢI cập nhật rating `(student, mode, subject, season)` bằng Glicko-2 với tham số Ranking Rules hiệu lực (`glicko2_params`) và khớp vector chuẩn Glickman sai số < 0.01 | FR-RNK-001 · AC-CMP-004 · ADR-0018 · [COMPETITION_SYSTEM §7](../../docs/01-game-design/COMPETITION_SYSTEM.md#7-game-rating--glicko-2-adr-0018) | TC-CMP-008 |
| F106-AC-02 | HỆ THỐNG PHẢI ghi thay đổi rating (trước/sau, `rule_version`) và cập nhật rating trong cùng transaction, unique `(match_id, student_id)`; xử lý lại cùng event PHẢI không đổi rating lần hai; sau commit PHẢI phát `rating.updated` | FR-RNK-001 · AC-RNK-002 | TC-CMP-009 |
| F106-AC-03 | NẾU trận unranked, bot, thách đấu bạn, giải đấu, `cancelled` hoặc `aborted`, THÌ HỆ THỐNG PHẢI không đổi rating và không cộng Competition Points | AC-RNK-003 · [COMPETITION_SYSTEM §7.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#73-công-thức-thang-glicko-2) | TC-CMP-008 |
| F106-AC-04 | Ở NƠI mode là `team_battle`, HỆ THỐNG PHẢI cập nhật từng thành viên như đấu với "đối thủ ảo" có μ, φ trung bình của đội kia và kết quả của đội; người `abandoned`/`forfeit` nhận kết quả 0, người còn lại 1 | [COMPETITION_SYSTEM §7.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#73-công-thức-thang-glicko-2) | TC-CMP-008 |
| F106-AC-05 | TRONG KHI học sinh còn provisional theo `rating_provisional`, HỆ THỐNG PHẢI trả trạng thái "Đang xếp hạng (x/10)", không có tier và không đưa vào bảng metric `rating`; matchmaking vẫn dùng μ/RD | [COMPETITION_SYSTEM §7.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#74-provisional) | — |
| F106-AC-06 | KHI job hằng tuần chạy, HỆ THỐNG PHẢI tăng RD của rating không chơi trong tuần bằng bước "no games" của Glicko-2, không vượt RD tối đa, không giảm rating (không rank decay) | [COMPETITION_SYSTEM §7.2, §12](../../docs/01-game-design/COMPETITION_SYSTEM.md#72-tham-số-confirmed) | — |

---

### User Story 2 — Tier thay số rating, tách khỏi năng lực học tập (Priority: P1)

`hs.dung.7a` mở hồ sơ thi đấu: thấy tier "Chồi Non" môn Toán và số trận đã chơi, không thấy con số rating; nhà trường không thấy rating như chỉ số năng lực.

**Why this priority**: Yêu cầu an toàn và tách số liệu (ADR-0018) là điều kiện bắt buộc trước khi hiển thị bất kỳ rating nào.

**Independent Test**: `getMyRatings` trả `tier_code`, `provisional`, `games_played`, không có `rating`/`rd`; quét response mọi API học sinh không có trường `rating`/`rd`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F106-AC-07 | KHI học sinh gọi `getMyRatings`, HỆ THỐNG PHẢI trả theo mode × môn × mùa: `tier_code`, tên tier (theo locale), `provisional`, tiến độ provisional, `games_played`; HỆ THỐNG PHẢI không trả số rating/RD thô cho học sinh ở bất kỳ API hay message realtime nào | FR-RNK-003 · AC-CMP-102 · [COMPETITION_SYSTEM §8.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#82-tier-hiển-thị) | — |
| F106-AC-08 | HỆ THỐNG PHẢI quy rating ra tier theo ngưỡng và tên trong Ranking Rules (`tiers`), chỉ hạ tier khi rating thấp hơn ngưỡng dưới quá hysteresis; ngưỡng/tên không hardcode | FR-RNK-003 · `tiers` | — |
| F106-AC-09 | HỆ THỐNG PHẢI không cung cấp Game Rating cho module learning hay School Learning Dashboard như chỉ số năng lực (không operation facade, không trường trong event học tập); Learning Score, Mastery, Game Rating, Game XP, Competition Points PHẢI lưu và trả tách biệt | FR-RNK-002 · AC-RNK-005 · ADR-0018 | — |

---

### User Story 3 — Competition Points thưởng tham gia và nỗ lực (Priority: P1)

Học sinh hoàn thành trận ranked được điểm tham gia, thắng/hòa được thêm, thua không bị trừ; quá trần ngày không cộng nữa.

**Why this priority**: Là metric chính của bảng xếp hạng thi đấu và season track.

**Independent Test**: 21 trận trong ngày → trận thứ 21 không cộng; tổng ngày ≤ `competition_points_daily_cap`; replay `match.completed` không cộng lần hai.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F106-AC-10 | KHI trận ranked hoàn thành, HỆ THỐNG PHẢI ghi Competition Points theo bảng điểm trong Ranking Rules (hoàn thành, thắng, hòa, câu đúng có trần, thắng tier cao hơn, abandon trừ nhưng không dưới 0) vào sổ cái append-only unique `(source_type, source_id, student_id)` kèm `rule_version`; thua PHẢI không bị trừ; sau commit PHẢI phát `competition_points.awarded` | FR-RNK-002 · [COMPETITION_SYSTEM §8.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#83-competition-points-confirmed) | TC-CMP-009 |
| F106-AC-11 | NẾU tổng điểm ngày đạt `competition_points_daily_cap` hoặc vượt số trận/ngày, THÌ HỆ THỐNG PHẢI cộng 0 cho phần vượt (vẫn ghi bút toán 0 có lý do `capped`) | AC-RNK-006 · `competition_points_daily_cap` | — |

---

### User Story 4 — Bảng xếp hạng an toàn theo khối (Priority: P1)

`hs.an.3a` (lớp 3) mở "Mục tiêu của khối" và thấy thanh tiến độ tập thể của khối 3 trường mình cùng phần em đã góp, không có thứ hạng cá nhân, không tên bạn nào (US-LDB-001); `hs.dung.7a` (lớp 7) thấy top 50 + 3 bạn quanh mình ở "Đấu trường trường"; `ad.truong.a` xem bảng trường/khối và mục tiêu của khối trường mình trong portal.

**Why this priority**: FR-LDB-001…005 là P4 Release; bảng xếp hạng là động lực chính nhưng rủi ro an toàn trẻ em cao.

**Independent Test**: `getLeaderboard` lớp 2 → `visibility = grade_goal`, không `entries[]`, không nickname học sinh khác; khối có < 5 học sinh góp sức → ô `contributors_count` bị ẩn; lớp 7 → top 50 + `around_me`; học sinh trường A đọc mục tiêu khối/bảng trường của trường B → 404; `FLUSHALL` Valkey → API trả `stale = true` rồi đúng như cũ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F106-AC-12 | HỆ THỐNG PHẢI dựng bảng từ `LeaderboardDefinition` đã publish (metric `competition_points`, `effort_points`, `grade_goal`, `xp`, `quest_completed`, `rating`, `team_points`; scope `global`, `tenant`, `school`, `grade`, `friends`, `team`; period `daily`, `weekly`, `monthly`, `season`, `all_time`; filter môn/chủ đề/mode) phủ đủ 12 loại bảng và danh mục mặc định; HỆ THỐNG PHẢI từ chối definition có scope `class` hoặc metric `class_goal` (ADR-0095) | FR-LDB-001 · [COMPETITION_SYSTEM §9.1–§9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#91-mô-hình-metric--scope--period--filter) | TC-CMP-008 |
| F106-AC-13 | HỆ THỐNG PHẢI giữ bảng kỳ đang mở trong Valkey sorted set (bằng điểm → ai đạt trước xếp trên) với nguồn sự thật là sổ cái PostgreSQL; consumer PHẢI kiểm tra `processed_events` trước khi cộng nên event lặp không cộng hai lần | FR-LDB-002 · AC-LDB-006 · ADR-0004 · ADR-0042 | TC-CMP-009 |
| F106-AC-14 | NẾU key Valkey của bảng mất, THÌ HỆ THỐNG PHẢI dựng lại từ sổ cái và trả `stale = true` trong lúc dựng; job đối chiếu định kỳ PHẢI sửa lệch | AC-LDB-002 · [COMPETITION_SYSTEM §9.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#93-kiến-trúc-dữ-liệu) | — |
| F106-AC-15 | KHI kỳ đóng theo giờ `Asia/Ho_Chi_Minh` (tuần: 00:00 thứ Hai), job singleton PHẢI chụp snapshot top 100 + thứ hạng mọi thành viên scope `school`/`grade` (với `grade_goal`: chỉ tổng tập thể và trạng thái đạt mục tiêu, không thứ hạng) vào PostgreSQL và phát `leaderboard.period_closed` idempotent theo `(definition, scope, period_key)` | AC-LDB-003 · [COMPETITION_SYSTEM §9.5](../../docs/01-game-design/COMPETITION_SYSTEM.md#95-reset-và-thưởng-theo-kỳ) | — |
| F106-AC-16 | HỆ THỐNG PHẢI tính `grade_goal` = tổng `effort_points` của học sinh hiệu lực cùng `school_id` + `grade_level` (khối tại thời điểm ghi điểm) so với mục tiêu tuần trong definition, trả `current`, `target`, phần đóng góp của chính người xem và `contributors_count`; HỆ THỐNG PHẢI không trả thứ hạng, nickname hay tên học sinh khác trong `grade_goal` và PHẢI ẩn mọi ô đóng góp có < `analytics_min_cell_size` (5) học sinh; không bảng scope `school`/`grade` nào dùng `xp`; `effort_points` PHẢI tính theo bảng `effort_points` (có trần ngày) và loại attempt gắn cờ `rapid_guess` | FR-LDB-005 · AC-LDB-004 · AC-LDB-007 · `effort_points` · ADR-0095 | — |
| F106-AC-17 | HỆ THỐNG PHẢI áp `leaderboard_visibility` theo khối người xem: lớp 1–3 không thứ hạng cá nhân (chỉ mục tiêu của khối `grade_goal`, không tên); lớp 4–5 top 10 + vị trí mình, ngoài top 10 chỉ "nhóm X%", không hiển thị người cuối; lớp 6–9 top 50 + ±3 quanh mình | FR-LDB-005 · AC-LDB-004 · [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em) | — |
| F106-AC-18 | HỆ THỐNG PHẢI chỉ trả nickname, avatar; không tên thật, `student_code`, tên lớp; không tên trường/khối chi tiết ở scope liên trường; bảng ngoài trường chỉ bật khi platform cho phép **và** trường có `allow_cross_school_listing`; học sinh bị sanction gian lận PHẢI bị loại khỏi kỳ hiện tại | FR-LDB-003 · AC-LDB-005 · AGENTS R16 | — |
| F106-AC-19 | Ở NƠI trường đã tắt bảng trong `disabled_leaderboard_definitions` (chỉ scope `school`/`grade`, kể cả mục tiêu của khối), HỆ THỐNG PHẢI không liệt kê bảng đó cho học sinh trường ấy và trả `404 NOT_FOUND` khi gọi trực tiếp | FR-LDB-004 · `disabled_leaderboard_definitions` | — |
| F106-AC-20 | HỆ THỐNG PHẢI cho học sinh chỉ đọc scope chứa mình (bảng/mục tiêu của khối khác, trường khác, tenant khác → `404 NOT_FOUND`); nhân sự có `lms.student.read` chỉ đọc bảng trường/khối của trường thuộc scope UserRole; `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH` | AC-LDB-001 · AGENTS R8 · NFR-SEC-006 | TC-TEN-001, TC-TEN-003, TC-TEN-011 |
| F106-AC-21 | KHI học sinh gọi `listLeaderboards`, HỆ THỐNG PHẢI trả các bảng được xem đã lọc theo khối, policy trường, cấu hình platform; `getLeaderboard`/`getMyLeaderboardPosition` PHẢI hỗ trợ `scope_id`, `period_key`, cursor; `getLeaderboardHistory` PHẢI trả kỳ đã đóng từ snapshot; tham số sai/vượt `maxLength` PHẢI nhận `422 VALIDATION_FAILED` | FR-LDB-001 · ADR-0041 | TC-TEN-024 |

---

### User Story 5 — Mùa giải 8 tuần và soft reset (Priority: P2)

"Mùa Phép Nhân" bắt đầu: rating mỗi học sinh co về gần trung bình, Competition Points về 0; hết mùa, tier cao nhất và rating cuối được lưu để trao thưởng.

**Why this priority**: Giữ động lực dài hạn nhưng không chặn luồng rating trong một mùa.

**Independent Test**: Đồng hồ giả qua `ends_at` → `season.ended`, `season_results` có tier cao nhất; mùa mới → rating mới `1500 + 0.5·(r−1500)`, RD ≥ 150; `listSeasons` trả mùa cũ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F106-AC-22 | KHI tới `starts_at`/`ends_at` của mùa đã publish, job PHẢI chuyển trạng thái mùa và phát `season.started`/`season.ended` đúng một lần (idempotent theo `season_id`) | FR-RNK-004 · [COMPETITION_SYSTEM §8.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#81-season-confirmed) | — |
| F106-AC-23 | KHI mùa kết thúc, HỆ THỐNG PHẢI lưu kết quả mùa (rating cuối, RD, tier cao nhất, số trận) không xóa; KHI mùa mới bắt đầu, HỆ THỐNG PHẢI tạo rating mùa mới theo `season_soft_reset` (volatility giữ nguyên) và Competition Points mùa mới bắt đầu từ 0; lịch sử mùa cũ PHẢI truy vấn được | FR-RNK-004 · AC-RNK-004 · `season_soft_reset` | — |
| F106-AC-24 | KHI học sinh hoặc nhân sự gọi `listSeasons`/`getSeason`, HỆ THỐNG PHẢI trả mùa hiện tại/đã qua (tên, chủ đề, thời gian, trạng thái) và, với học sinh, tier cao nhất của chính mình | FR-RNK-004 | — |

### Edge Cases

- NẾU `match.completed` đến khi chưa có mùa active, THÌ trận vẫn ghi rating vào mùa chứa `ended_at` của trận; không có mùa nào → rating mùa "pre-season" do seed tạo (F106-AC-01).
- NẾU học sinh chuyển trường hoặc đổi khối giữa kỳ (`student.placement_changed`), THÌ điểm đã cộng ở bảng/mục tiêu của trường/khối cũ giữ trong kỳ đó; điểm sau thời điểm đổi tính theo trường/khối mới.
- NẾU học sinh bị soft-delete (dọn học sinh), THÌ bị loại khỏi bảng đang mở; snapshot cũ giữ nguyên nhưng hiển thị nickname ẩn danh.
- NẾU Ranking Rules publish version mới giữa mùa, THÌ chỉ áp từ `effective_from` cho trận sau đó; `rating_changes` cũ giữ `rule_version` cũ.
- NẾU bảng friends có > 200 bạn, THÌ chỉ tính 200 bạn gần đây nhất (giới hạn đọc).
- NẾU hai học sinh bằng điểm, THÌ ai đạt trước xếp trên (F106-AC-13).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-RNK-001 | Glicko-2 theo `(student, mode, subject, season)` | [COMPETITION_SYSTEM §7](../../docs/01-game-design/COMPETITION_SYSTEM.md#7-game-rating--glicko-2-adr-0018) |
| FR-RNK-002 | Tách 5 loại số liệu; Competition Points | [COMPETITION_SYSTEM §6, §8.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#6-năm-loại-số-liệu-confirmed) |
| FR-RNK-003 | Tier, không số rating thô | [COMPETITION_SYSTEM §8.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#82-tier-hiển-thị) |
| FR-RNK-004 | Mùa và soft reset | [COMPETITION_SYSTEM §8.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#81-season-confirmed) |
| FR-LDB-001 | 12 loại bảng (metric × scope × period × filter; không có Class — ADR-0095) | [COMPETITION_SYSTEM §9.1–§9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#91-mô-hình-metric--scope--period--filter) |
| FR-LDB-002 | Valkey sorted set, nguồn PostgreSQL | [COMPETITION_SYSTEM §9.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#93-kiến-trúc-dữ-liệu) |
| FR-LDB-003 | Ngoài trường chỉ nickname | [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em) |
| FR-LDB-004 | Trường tắt bảng trường/khối (kể cả mục tiêu của khối) | [COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#92-danh-mục-bảng-mặc-định-confirmed) |
| FR-LDB-005 | Mục tiêu tập thể của khối trong trường (`grade_goal`): tiến độ chung theo nỗ lực, không xếp hạng cá nhân, không tên, ẩn ô < 5 học sinh, không XP | [COMPETITION_SYSTEM §9.1–§9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#91-mô-hình-metric--scope--period--filter) |

### Key Entities

- **Rating**: Game Rating của học sinh theo mode × môn × mùa (μ, RD, volatility, số trận, tier hiện tại, tier cao nhất).
- **Rating Change**: thay đổi rating do một trận, kèm phiên bản luật.
- **Competition Point Entry**: bút toán điểm thi đấu append-only theo nguồn.
- **Leaderboard Contribution**: bút toán điểm cho metric bảng khác (`effort_points`, `xp`, `quest_completed`, `team_points`) từ event của module khác, kèm trường và khối của học sinh tại thời điểm ghi.
- **Grade Goal** (mục tiêu tập thể của khối): giá trị gộp `effort_points` theo (trường, khối, tuần) so với mục tiêu tuần; không phải bảng xếp hạng.
- **Leaderboard Definition**: metric × scope × period × filter × khối áp dụng, do platform publish.
- **Leaderboard Snapshot**: thứ hạng cuối kỳ.
- **Season / Season Result**: mùa giải và kết quả mùa của từng học sinh.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `glicko2_params` | r0, RD, σ, τ, RD min/max |
| `rating_provisional` | Ngưỡng provisional |
| `tiers` | Ngưỡng, tên, hysteresis tier |
| `season_length`, `season_soft_reset` | Độ dài mùa, công thức reset |
| `competition_points_daily_cap` | Trần điểm ngày, số trận/ngày |
| `effort_points` | Bảng điểm nỗ lực, trần ngày |
| `leaderboard_visibility` | Hiển thị theo khối |
| `disabled_leaderboard_definitions` | Bảng trường đã tắt |
| `analytics_min_cell_size` | Ẩn ô đóng góp của `grade_goal` có < 5 học sinh [Confirmed — dùng lại] |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Rating chỉ cho thi đấu, không là chỉ số học tập | ADR-0018 · FR-RNK-002 |
| Valkey chỉ là cache bảng; sổ cái/snapshot ở PostgreSQL | AGENTS R9 · ADR-0004 |
| Consumer idempotent (`processed_events`) | ADR-0042 · AGENTS R10 |
| Không lộ tên thật/trường | AGENTS R16 · ADR-0054 |
| Không thông báo kiểu "bạn bị vượt qua", không rank decay | AGENTS R19 · NFR-SAFE-003 |
| Ranking Rules, leaderboard definition chỉ platform | AGENTS R20 · ADR-0027 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Hàm Glicko-2 khớp vector chuẩn Glickman với sai số < 0.01.
- **SC-002**: Replay toàn bộ `match.completed` của một ngày hai lần → rating, Competition Points, điểm bảng không đổi so với lần đầu.
- **SC-003**: 0 response API học sinh chứa trường số rating/RD (contract test quét mọi route học sinh).
- **SC-004**: Học sinh lớp 1–3 không bao giờ nhận thứ hạng cá nhân ở bất kỳ bảng nào.
- **SC-005**: Xóa toàn bộ key Valkey bảng xếp hạng → bảng dựng lại khớp 100% với sổ cái trong ≤ 5 phút cho 50.000 học sinh.

## Ngoài phạm vi

- Ghi kết quả trận — F105; ghép trận dùng rating — F104 (F106 cung cấp `RatingReader`).
- Soạn Ranking Rules, leaderboard definition, season — F108; giao diện bảng/tier trong Godot — F109.
- Season track, thưởng cuối mùa theo tier, XP thi đấu — module game (F077, F074) qua `season.ended`, `match.completed`; soạn cấu hình `competition-rewards` — F108.
- Bật/tắt bảng trong màn cài đặt trường (ghi `disabled_leaderboard_definitions`) — cài đặt trường của module identity/portal (F015, F052).
- Opt-out ẩn tên khỏi bảng ngoài trường; vô hiệu trận và hoàn tác rating do gian lận — feature sau.

## Assumptions

- [Confirmed] Mùa "pre-season" seed để có `season_id` trước khi F108 publish mùa đầu.
- [Confirmed] Nhân sự xem bảng trong portal dùng permission `lms.student.read` (ADR-0095 mục 6); có thể bật tên thật ở bảng trường/khối (không áp cho `grade_goal` — không có danh sách tên) cho chính mình theo [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em) (`?show_real_names=true`, chỉ trong trường thuộc scope, audit).
- [Confirmed] Mục tiêu tuần của `grade_goal` nằm trong payload leaderboard definition (`display.grade_goal_target`), không hardcode.
- Danh sách sanction gian lận đọc từ `competition.competition_restrictions` (`restrict_competition`).
