# Feature Specification: F106 — Ranking Glicko-2, tier, season và 13 bảng xếp hạng

**Feature Branch**: `106-ranking-leaderboard` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` · **Mốc**: M4b · **Phụ thuộc**: F105
**Input**: Sau mỗi trận ranked, server cập nhật Game Rating Glicko-2 theo (học sinh, mode, môn, mùa), quy ra tier thân thiện, cộng Competition Points; bảng xếp hạng theo metric × phạm vi × kỳ với quy tắc an toàn trẻ em; mùa giải 8 tuần có soft reset. FR nguồn: FR-RNK-001…004, FR-LDB-001…005; US-LDB-001.

## Clarifications

### Session 2026-11-16

- Q: FR-RNK-003 chỉ ẩn số rating với lớp 1–5, COMPETITION_SYSTEM §8.2 ẩn với mọi khối — theo bên nào? → A: Theo tài liệu sở hữu (COMPETITION_SYSTEM §8.2, tham số `tiers` "ẩn số rating thô với mọi học sinh"); chặt hơn PRD nên không vi phạm FR.
- Q: "13 bảng xếp hạng" ánh xạ thế nào? → A: 13 loại của FR-LDB-001 = tổ hợp metric × scope × period × filter của `LeaderboardDefinition` (Global, School, Class, Grade, Subject, Topic, Game Mode, Friends, Team, Daily, Weekly, Monthly, Season); danh mục mặc định ở [COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#92-danh-mục-bảng-mặc-định-proposed) là seed.
- Q: `effort_points` lấy từ đâu? → A: Từ event học tập của module learning (`activity.graded`) đã loại attempt gắn cờ `rapid_guess`; cần bổ sung trường vào payload (contracts §6).
- Q: Opt-out ẩn tên khỏi bảng ngoài lớp ([COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em), [Proposed]) có làm không? → A: Không trong F106 (chưa có contract); mặc định đã ẩn khỏi bảng ngoài trường qua `allow_cross_school_listing = false`.

### Session 2026-11-17

- Q: Tên và tham số sau đối soát (RB-412, RB-415)? → A: Bảng `rating_changes`, khóa `leaderboard_id`, key Valkey theo VALKEY_DESIGN; lớp 6–9 thấy ±3 quanh mình. Phần thưởng thi đấu (mùa, giải, thử thách) là cấu hình `competition-rewards` (F108, quyền `competition.config.reward.write`).

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
| F106-AC-06 | KHI job hằng tuần chạy, HỆ THỐNG PHẢI tăng RD của rating không chơi trong tuần bằng bước "no games" của Glicko-2, không vượt RD tối đa, không giảm rating (không rank decay) | [COMPETITION_SYSTEM §7.2, §12](../../docs/01-game-design/COMPETITION_SYSTEM.md#72-tham-số-proposed) | — |

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
| F106-AC-10 | KHI trận ranked hoàn thành, HỆ THỐNG PHẢI ghi Competition Points theo bảng điểm trong Ranking Rules (hoàn thành, thắng, hòa, câu đúng có trần, thắng tier cao hơn, abandon trừ nhưng không dưới 0) vào sổ cái append-only unique `(source_type, source_id, student_id)` kèm `rule_version`; thua PHẢI không bị trừ; sau commit PHẢI phát `competition_points.awarded` | FR-RNK-002 · [COMPETITION_SYSTEM §8.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#83-competition-points-proposed) | TC-CMP-009 |
| F106-AC-11 | NẾU tổng điểm ngày đạt `competition_points_daily_cap` hoặc vượt số trận/ngày, THÌ HỆ THỐNG PHẢI cộng 0 cho phần vượt (vẫn ghi bút toán 0 có lý do `capped`) | AC-RNK-006 · `competition_points_daily_cap` | — |

---

### User Story 4 — Bảng xếp hạng an toàn theo khối (Priority: P1)

`hs.an.3a` (lớp 3) mở "Mục tiêu lớp" và thấy thanh tiến độ tập thể, không có thứ hạng cá nhân; `hs.dung.7a` (lớp 7) thấy top 50 + 3 bạn quanh mình ở "Đấu trường trường"; `ad.truong.a` xem bảng lớp trường mình trong portal (US-LDB-001).

**Why this priority**: FR-LDB-001…005 là P4 Release; bảng xếp hạng là động lực chính nhưng rủi ro an toàn trẻ em cao.

**Independent Test**: `getLeaderboard` lớp 2 → `visibility = class_goal`, không `entries[].rank`; lớp 7 → top 50 + `around_me`; học sinh trường A đọc bảng lớp trường B → 404; `FLUSHALL` Valkey → API trả `stale = true` rồi đúng như cũ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F106-AC-12 | HỆ THỐNG PHẢI dựng bảng từ `LeaderboardDefinition` đã publish (metric `competition_points`, `effort_points`, `class_goal`, `xp`, `quest_completed`, `rating`, `team_points`; scope `global`, `tenant`, `school`, `class`, `grade`, `friends`, `team`; period `daily`, `weekly`, `monthly`, `season`, `all_time`; filter môn/chủ đề/mode) phủ đủ 13 loại bảng và danh mục mặc định | FR-LDB-001 · [COMPETITION_SYSTEM §9.1–§9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#91-mô-hình-metric--scope--period--filter) | TC-CMP-008 |
| F106-AC-13 | HỆ THỐNG PHẢI giữ bảng kỳ đang mở trong Valkey sorted set (bằng điểm → ai đạt trước xếp trên) với nguồn sự thật là sổ cái PostgreSQL; consumer PHẢI kiểm tra `processed_events` trước khi cộng nên event lặp không cộng hai lần | FR-LDB-002 · AC-LDB-006 · ADR-0004 · ADR-0042 | TC-CMP-009 |
| F106-AC-14 | NẾU key Valkey của bảng mất, THÌ HỆ THỐNG PHẢI dựng lại từ sổ cái và trả `stale = true` trong lúc dựng; job đối chiếu định kỳ PHẢI sửa lệch | AC-LDB-002 · [COMPETITION_SYSTEM §9.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#93-kiến-trúc-dữ-liệu) | — |
| F106-AC-15 | KHI kỳ đóng theo giờ `Asia/Ho_Chi_Minh` (tuần: 00:00 thứ Hai), job singleton PHẢI chụp snapshot top 100 + thứ hạng mọi thành viên scope `class`/`school` vào PostgreSQL và phát `leaderboard.period_closed` idempotent theo `(definition, scope, period_key)` | AC-LDB-003 · [COMPETITION_SYSTEM §9.5](../../docs/01-game-design/COMPETITION_SYSTEM.md#95-reset-và-thưởng-theo-kỳ) | — |
| F106-AC-16 | HỆ THỐNG PHẢI chỉ cho bảng scope `class` dùng `effort_points`, `class_goal`, `quest_completed`; không bảng scope `class`/`school`/`grade` nào dùng `xp`; `effort_points` PHẢI tính theo bảng `effort_points` (có trần ngày) và loại attempt gắn cờ `rapid_guess` | FR-LDB-005 · AC-LDB-007 · `effort_points` | — |
| F106-AC-17 | HỆ THỐNG PHẢI áp `leaderboard_visibility` theo khối người xem: lớp 1–3 không thứ hạng cá nhân (chỉ mục tiêu lớp và "Bạn nổi bật tuần này" ngẫu nhiên trong nhóm đạt ngưỡng); lớp 4–5 top 10 + vị trí mình, ngoài top 10 chỉ "nhóm X%", không hiển thị người cuối; lớp 6–9 top 50 + ±3 quanh mình | FR-LDB-005 · AC-LDB-004 · [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em) | — |
| F106-AC-18 | HỆ THỐNG PHẢI chỉ trả nickname, avatar và (scope lớp) tên lớp; không tên thật, `student_code`, tên trường ở scope liên trường; bảng ngoài trường chỉ bật khi platform cho phép **và** trường có `allow_cross_school_listing`; học sinh bị sanction gian lận PHẢI bị loại khỏi kỳ hiện tại | FR-LDB-003 · AC-LDB-005 · AGENTS R16 | — |
| F106-AC-19 | Ở NƠI trường đã tắt bảng trong `disabled_leaderboard_definitions` (chỉ scope `class`/`school`), HỆ THỐNG PHẢI không liệt kê bảng đó cho học sinh trường ấy và trả `404 NOT_FOUND` khi gọi trực tiếp | FR-LDB-004 · `disabled_leaderboard_definitions` | — |
| F106-AC-20 | HỆ THỐNG PHẢI cho học sinh chỉ đọc scope chứa mình (bảng lớp/trường khác, tenant khác → `404 NOT_FOUND`); nhân sự có `lms.class.read` chỉ đọc các lớp trong trường thuộc scope UserRole; `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH` | AC-LDB-001 · AGENTS R8 · NFR-SEC-006 | TC-TEN-001, TC-TEN-003, TC-TEN-011 |
| F106-AC-21 | KHI học sinh gọi `listLeaderboards`, HỆ THỐNG PHẢI trả các bảng được xem đã lọc theo khối, policy trường, cấu hình platform; `getLeaderboard`/`getMyLeaderboardPosition` PHẢI hỗ trợ `scope_id`, `period_key`, cursor; `getLeaderboardHistory` PHẢI trả kỳ đã đóng từ snapshot; tham số sai/vượt `maxLength` PHẢI nhận `422 VALIDATION_FAILED` | FR-LDB-001 · ADR-0041 | TC-TEN-024 |

---

### User Story 5 — Mùa giải 8 tuần và soft reset (Priority: P2)

"Mùa Phép Nhân" bắt đầu: rating mỗi học sinh co về gần trung bình, Competition Points về 0; hết mùa, tier cao nhất và rating cuối được lưu để trao thưởng.

**Why this priority**: Giữ động lực dài hạn nhưng không chặn luồng rating trong một mùa.

**Independent Test**: Đồng hồ giả qua `ends_at` → `season.ended`, `season_results` có tier cao nhất; mùa mới → rating mới `1500 + 0.5·(r−1500)`, RD ≥ 150; `listSeasons` trả mùa cũ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F106-AC-22 | KHI tới `starts_at`/`ends_at` của mùa đã publish, job PHẢI chuyển trạng thái mùa và phát `season.started`/`season.ended` đúng một lần (idempotent theo `season_id`) | FR-RNK-004 · [COMPETITION_SYSTEM §8.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#81-season-proposed) | — |
| F106-AC-23 | KHI mùa kết thúc, HỆ THỐNG PHẢI lưu kết quả mùa (rating cuối, RD, tier cao nhất, số trận) không xóa; KHI mùa mới bắt đầu, HỆ THỐNG PHẢI tạo rating mùa mới theo `season_soft_reset` (volatility giữ nguyên) và Competition Points mùa mới bắt đầu từ 0; lịch sử mùa cũ PHẢI truy vấn được | FR-RNK-004 · AC-RNK-004 · `season_soft_reset` | — |
| F106-AC-24 | KHI học sinh hoặc nhân sự gọi `listSeasons`/`getSeason`, HỆ THỐNG PHẢI trả mùa hiện tại/đã qua (tên, chủ đề, thời gian, trạng thái) và, với học sinh, tier cao nhất của chính mình | FR-RNK-004 | — |

### Edge Cases

- NẾU `match.completed` đến khi chưa có mùa active, THÌ trận vẫn ghi rating vào mùa chứa `ended_at` của trận; không có mùa nào → rating mùa "pre-season" do seed tạo (F106-AC-01).
- NẾU học sinh chuyển trường/lớp giữa kỳ, THÌ điểm đã cộng ở bảng lớp/trường cũ giữ trong kỳ đó; kỳ mới tính theo lớp/trường mới.
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
| FR-RNK-004 | Mùa và soft reset | [COMPETITION_SYSTEM §8.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#81-season-proposed) |
| FR-LDB-001 | 13 loại bảng (metric × scope × period × filter) | [COMPETITION_SYSTEM §9.1–§9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#91-mô-hình-metric--scope--period--filter) |
| FR-LDB-002 | Valkey sorted set, nguồn PostgreSQL | [COMPETITION_SYSTEM §9.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#93-kiến-trúc-dữ-liệu) |
| FR-LDB-003 | Ngoài trường chỉ nickname | [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em) |
| FR-LDB-004 | Trường tắt bảng lớp/trường | [COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#92-danh-mục-bảng-mặc-định-proposed) |
| FR-LDB-005 | Bảng lớp theo nỗ lực/mục tiêu tập thể, không XP | [COMPETITION_SYSTEM §9.1–§9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#91-mô-hình-metric--scope--period--filter) |

### Key Entities

- **Rating**: Game Rating của học sinh theo mode × môn × mùa (μ, RD, volatility, số trận, tier hiện tại, tier cao nhất).
- **Rating Change**: thay đổi rating do một trận, kèm phiên bản luật.
- **Competition Point Entry**: bút toán điểm thi đấu append-only theo nguồn.
- **Leaderboard Contribution**: bút toán điểm cho metric bảng khác (`effort_points`, `xp`, `quest_completed`, `team_points`) từ event của module khác.
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
- Opt-out ẩn tên khỏi bảng ngoài lớp; vô hiệu trận và hoàn tác rating do gian lận — feature sau.

## Assumptions

- [Proposed] Mùa "pre-season" seed để có `season_id` trước khi F108 publish mùa đầu.
- [Proposed] Nhân sự xem bảng trong portal dùng permission `lms.class.read`; có thể bật tên thật cho chính mình theo [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em) (`?show_real_names=true`, chỉ trong trường thuộc scope, audit).
- [Proposed] Ngưỡng "Bạn nổi bật tuần này" và mục tiêu tuần của `class_goal` nằm trong payload leaderboard definition.
- Danh sách sanction gian lận đọc từ `competition.competition_restrictions` (`restrict_competition`).
