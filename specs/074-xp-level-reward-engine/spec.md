# Feature Specification: F074 — XP ledger, level, đường cong XP, danh hiệu, Reward Engine có cap

**Feature Branch**: `074-xp-level-reward-engine` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `game` · **Mốc**: M2 · **Phụ thuộc**: F072
**Input**: Reward Engine phía server cộng XP từ domain event đã kiểm chứng (quest, activity, mastery, bài học, khám phá) theo reward rule có cap ngày, giảm dần khi lặp, miễn giảm cho ôn LO tiên quyết; sổ cái XP append-only idempotent; level theo đường cong XP có version, danh hiệu theo khoảng level, phần thưởng lên cấp một lần; API tiến trình cho học sinh; quy tắc chuyển năm học. FR nguồn: FR-GAM-001…008, FR-GAM-010, FR-GAM-011, FR-RWD-003, NFR-SAFE-003.

## Clarifications

### Session 2026-11-16

- Q: Hoạt động học tập kích hoạt XP bằng event nào? → A: `activity.graded` (đã chấm xong, payload schema có sẵn); `exercise.submitted` trong EVENT_DESIGN §4.3 không có payload schema nên không dùng. Payload cần bổ sung `content_version_id`, `first_try_correct_count`, `recommendation_id`, `content_grade_levels` (delta ở [contracts](contracts/README.md) §6).
- Q: Config type `levels` hay `progression-curves`? → A: `progression-curves` (permission `game.progression.write` — đổi ở Session 2026-11-17). Enum `levels` đổi thành `progression-levels` (Session 2026-11-17); gói level/zone (`game.level_definitions`) thuộc F071.
- Q: Phần thưởng lên cấp loại Xu/vật phẩm/huy hiệu? → A: Reward Engine cấp qua `GrantApplier` theo loại; F074 có applier `xp`, `title`; applier `currency`, `item`, `badge`, `collection_item` do F075 đăng ký. Publish curve có loại thưởng chưa có applier bị từ chối.
- Q: Ranh giới "ngày" của cap XP? → A: Ngày lịch theo giờ `Asia/Ho_Chi_Minh` [Confirmed] (tài liệu sở hữu chưa ghi; xem Assumptions).

### Session 2026-11-17

- Q: Nguồn XP `lesson.completed` (PROGRESSION_SYSTEM §3.1) lấy từ event nào? → A: Domain event `lesson.completed` do module learning phát (owner F031, payload `lesson.completed.v1`) khi học sinh hoàn thành mọi activity bắt buộc của bài, version bài ghim, một lần mỗi học sinh × version bài (Technical Architect 2026-11-17); F074 tiêu thụ trong `game.reward_engine` (F074-AC-27).
- Q: Tên config type bảng level, tên cột ngưỡng và permission? → A: Config type `levels` đổi thành `progression-levels` (kebab, bảng level sinh khi publish curve, chỉ đọc/ghi đè theo level — API_DESIGN, RB-308); cột `game.progression_levels.xp_required` (PROGRESSION_SYSTEM §6, DATABASE_DESIGN §2.8a); permission `game.progression.write` cho `progression-curves` và `progression-levels` (thay `game.level.write`, RB-309). Field API `Progression.xp_to_next` (XP còn thiếu tới level kế) giữ nguyên.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Cộng XP server-side từ sự kiện đã kiểm chứng (Priority: P1)

Học sinh hoàn thành quest, bài tập, bài học, đạt mastery một LO hoặc khám phá một điểm trong thế giới thì nhận XP; làm lại cùng bài nhận ít dần; chạm cap ngày thì được báo nhẹ nhàng. US-GAM-001.

**Why this priority**: XP là động lực cốt lõi của vòng lặp game và phải chống farm, chống cộng trùng ngay từ đầu.

**Independent Test**: Phát `activity.graded` giả lập 4 lần cùng content version → ledger 100%/50%/25%/0%; phát lại cùng `event_id` → không thêm dòng; đẩy tổng XP vượt cap → bút toán `capped = true`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F074-AC-01 | KHI module game nhận `activity.graded`, `lesson.completed`, `quest.completed`, `minigame.completed`, `learning_objective.mastered`, `world.discovery_found` hoặc `world.area_completed`, Reward Engine PHẢI áp mọi reward rule đã publish có `trigger.event_type` khớp và điều kiện thỏa, ghi bút toán XP kèm `source_code`, `rule_id`, `rule_version` | FR-GAM-002 · FR-GAM-007 · [PROGRESSION_SYSTEM §3.1](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | TC-GAM-004 |
| F074-AC-02 | KHI cùng event được giao lại hoặc phát lại với cùng `(student_id, source_type, source_id, rule_id)`, module game PHẢI không cộng XP lần hai | FR-GAM-004 · AC-GAM-003 | TC-GAM-004 |
| F074-AC-03 | HỆ THỐNG PHẢI ghi bút toán XP, cập nhật `xp_total`/`level` cache và outbox `xp.awarded` trong một transaction; job đối soát PHẢI phát cảnh báo khi `xp_total` khác tổng ledger và không tự sửa | FR-GAM-004 · ADR-0049 | TC-ECO-002 |
| F074-AC-04 | KHI cùng content version được hoàn thành lặp trong cửa sổ của `xp_repeat_multipliers`, Reward Engine PHẢI áp hệ số theo thứ tự lần hoàn thành (lần cuối của danh sách áp cho mọi lần sau) | FR-GAM-003 · AC-GAM-001 | TC-GAM-002 |
| F074-AC-05 | KHI XP trong ngày của học sinh chạm `xp_daily_cap` hoặc cap của rule, Reward Engine PHẢI chỉ cộng phần còn lại, ghi `capped = true` (kể cả bút toán 0 XP) và đẩy `progression.xp.updated` mang cờ capped | FR-GAM-003 · AC-GAM-002 | TC-GAM-001 |
| F074-AC-06 | Reward Engine PHẢI tính XP theo câu đúng lần đầu (`first_try_correct_count`): câu sai, câu `rapid_guess`, câu đúng sau khi đã xem giải thích PHẢI cho 0 XP | FR-GAM-003 · [PROGRESSION_SYSTEM §3.2](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | TC-GAM-001 |
| F074-AC-07 | KHI nội dung thấp hơn khối hiện tại của học sinh từ 2 khối trở lên, Reward Engine PHẢI nhân `xp_below_grade_multiplier`; NẾU attempt có `recommendation_id` hoặc mode `assignment`, THÌ PHẢI cho XP đầy đủ (giảm dần và cap vẫn áp) | FR-GAM-011 · AC-GAM-014 | — |
| F074-AC-08 | KHI nhận `learning_objective.mastered`, module game PHẢI cộng `xp_objective_mastered` đúng một lần cho mỗi `(student_id, objective_id)`, kể cả event lặp hoặc LO đạt lại sau decay | FR-GAM-002 · AC-GAM-015 | — |
| F074-AC-09 | HỆ THỐNG PHẢI không có request schema nào của học sinh chứa `xp`, `amount`, `score`; KHÔNG rule nào được kích hoạt bởi đăng nhập, thời gian online, tốc độ hay chuỗi đúng | FR-RWD-003 · NFR-SAFE-003 · ADR-0016 | — |
| F074-AC-10 | module game PHẢI không gọi module learning và không phát event nào làm thay đổi điểm/mastery; module learning PHẢI không có consumer `xp.awarded`/`level.reached` | FR-GAM-005 · FR-GAM-006 | TC-GAM-003 |
| F074-AC-11 | NẾU `tenant_id` của envelope khác tenant của học sinh trong projection `ref.students` hoặc học sinh không tồn tại, THÌ consumer PHẢI không ghi bút toán, chuyển event vào retry rồi DLQ và ghi metric | FR-GAM-004 · NFR-SEC-006 | TC-TEN-005 |
| F074-AC-12 | HỆ THỐNG PHẢI giữ `xp_ledger`, `level_history` append-only: role `game_app` thực thi `UPDATE`/`DELETE` PHẢI bị từ chối | FR-GAM-004 · AGENTS R21 | TC-ECO-008 |
| F074-AC-27 | KHI nhận `lesson.completed` (owner F031), Reward Engine PHẢI cộng XP theo rule `source_code = lesson.completed` với `source_type = lesson`, `source_id = lesson_version_id` (một bút toán cho mỗi học sinh × version bài, kể cả event giao lại), áp `xp_below_grade_multiplier` theo `grade_levels` (đầy đủ khi `mode = assignment` hoặc có `recommendation_id`), giảm dần khi lặp và cap ngày | FR-GAM-002 · FR-GAM-003 · [PROGRESSION_SYSTEM §3.1](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | TC-GAM-004 |

---

### User Story 2 — Level, danh hiệu, phần thưởng lên cấp (Priority: P1)

Đủ XP thì nhân vật lên cấp, có hiệu ứng, danh hiệu mới theo mốc level và phần thưởng của level đó.

**Why this priority**: Mục tiêu gần của học sinh (FR-GAM-008); phần thưởng phải cấp đúng một lần.

**Independent Test**: Cộng XP vượt 2 ngưỡng một lần → hai `level.reached` (n→n+1, n+1→n+2), hai dòng `level_history`, mỗi level có thưởng sinh đúng một RewardGrant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F074-AC-13 | KHI `xp_total` vượt ngưỡng level theo curve version hiện hành, module game PHẢI ghi `level_history`, cập nhật `level`/`title_code`, phát `level.reached` cho từng cấp (kể cả nhiều cấp một lần) và đẩy `progression.level.reached` | FR-GAM-008 · AC-GAM-005 | TC-GAM-005 |
| F074-AC-14 | KHI học sinh đạt một level có `rewards`, Reward Engine PHẢI tạo đúng một RewardGrant idempotent theo `(student_id, level, curve_id)` và áp phần thưởng trong cùng transaction; đạt lại level đó sau điều chỉnh PHẢI không nhận lại | FR-GAM-008 · FR-RWD-003 | TC-GAM-005 |
| F074-AC-15 | module game PHẢI suy danh hiệu từ khoảng level trong `titles` của curve version hiện hành và trả nhãn song ngữ `vi`/`en`; danh hiệu chỉ có ý nghĩa trong game | FR-GAM-007 · [PROGRESSION_SYSTEM §4.3](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | — |
| F074-AC-16 | TRONG KHI học sinh ở `max_level`, module game PHẢI tiếp tục ghi ledger, trả `xp_to_next = null` và không phát `level.reached` | FR-GAM-007 | — |

---

### User Story 3 — Học sinh xem tiến trình và lịch sử XP (Priority: P1)

HUD hiển thị level, danh hiệu, thanh tiến độ, XP còn nhận được hôm nay; màn "Hành trình của tôi" liệt kê giao dịch XP bằng lời thân thiện.

**Why this priority**: Không thấy tiến độ thì XP không tạo động lực; API là contract cho Godot.

**Independent Test**: `hs.an.3a` gọi `getProgression`, `listXpHistory`, `listProgressionLevels` → dữ liệu của chính mình; token nhân sự → 403.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F074-AC-17 | KHI học sinh gọi `getProgression`, module game PHẢI trả `level`, `title`, `xp_total`, `xp_in_level`, `xp_to_next`, `daily_xp_remaining`, `curve_version`, `max_level`, `next_level_rewards` của chính học sinh | FR-GAM-008 | — |
| F074-AC-18 | KHI học sinh gọi `listXpHistory`, module game PHẢI trả trang cursor gồm `amount`, `source_code`, `source_label` thân thiện, `capped`, `created_at`, không chứa `source_id`, `rule_id`, `idempotency_key` | FR-GAM-008 | — |
| F074-AC-19 | HỆ THỐNG PHẢI cho `getProgression`, `listXpHistory`, `listProgressionLevels`, `getXpStatus` chỉ qua middleware `game_player`: token nhân sự PHẢI nhận `403 GAME_ACCESS_STUDENT_ONLY`; `X-Tenant-Id` khác tenant của phiên PHẢI nhận `403 TENANT_MISMATCH`; không route nào nhận `student_id` của người khác | AGENTS R23 · NFR-SEC-006 | TC-AUTH-014 |
| F074-AC-20 | NẾU `limit` ngoài khoảng cho phép hoặc `cursor` sai định dạng/vượt `maxLength`, THÌ module game PHẢI trả `422 VALIDATION_FAILED` chỉ rõ tham số | ADR-0041 | TC-TEN-024 |

---

### User Story 4 — Đường cong XP và reward rule có version (Priority: P2)

Game Designer chỉnh đường cong, danh hiệu, phần thưởng lên cấp, catalog nguồn XP và cap mà không build lại client; publish curve mới không làm ai tụt level.

**Why this priority**: Cân bằng sau pilot; cần trước live-ops (M3).

**Independent Test**: Học sinh level 12, publish curve tăng ngưỡng → vẫn level 12, `xp_to_next` theo curve mới; curve giảm ngưỡng → job rollout phát lần lượt `level.reached`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F074-AC-21 | KHI một version `progression-curves` được publish, module game PHẢI sinh bảng level (ngưỡng tăng đơn điệu, `level_overrides` ghi đè công thức) và áp cho mọi học sinh mà không giảm level đang có (`level_floor_protection`) | FR-GAM-007 · AC-GAM-006 | TC-GAM-006 |
| F074-AC-22 | KHI curve mới khiến học sinh đủ XP lên một hoặc nhiều cấp, job `curve-rollout` PHẢI phát lần lượt từng `level.reached` và cấp thưởng theo F074-AC-14, idempotent khi job chạy lại | FR-GAM-008 · [PROGRESSION_SYSTEM §4.2](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | TC-GAM-005 |
| F074-AC-23 | NẾU version curve hoặc reward rule không qua `progression-curve.schema.json`/`reward-rule.schema.json`, ngưỡng không tăng đơn điệu, cap ≤ 0, `source_code` ngoài catalog, trigger ngoài danh mục hoặc phần thưởng có loại chưa có applier, THÌ module game PHẢI từ chối `422 VALIDATION_FAILED` chỉ rõ field | FR-GAM-007 · [ADMIN_PORTAL §3.12](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F074-AC-24 | KHI reward rule đổi version, Reward Engine PHẢI chỉ áp version mới cho event có `occurred_at` từ `effective_from`/`active_from` và không thu hồi XP đã cấp | FR-GAM-003 · [ADMIN_PORTAL §9](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |

---

### User Story 5 — Chuyển năm học (Priority: P2)

Học sinh lên lớp giữ nguyên level, XP, danh hiệu; quest học tập của khối cũ hết hạn không phạt.

**Why this priority**: Xảy ra mỗi năm học qua SIS; sai sẽ làm mất tiến trình.

**Independent Test**: Phát `student.synced` đổi `grade_level` 3→4 hai lần → level/XP giữ; quest `learning` khối 3 `expired`; lần hai không đổi gì.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F074-AC-25 | KHI nhận `student.synced` đổi `grade_level` (hoặc `enrollment.changed` sang lớp khối khác), module game PHẢI chạy `ApplyGradeTransition` idempotent theo `(student_id, school_year)`: giữ XP, level, danh hiệu, ledger; quest `learning`/`daily`/`weekly` đang `active` có `grade_levels` không chứa khối mới PHẢI chuyển `expired` không phạt | FR-GAM-010 · AC-GAM-016 | — |
| F074-AC-26 | NẾU `grade_level` không đổi (ở lại lớp), THÌ module game PHẢI không thay đổi dữ liệu và không phát thông điệp nào | FR-GAM-010 | — |

### Edge Cases

- NẾU `activity.graded` đến trước khi projection `ref.students` có học sinh, THÌ consumer PHẢI NAK có backoff (F074-AC-11), không bỏ event.
- NẾU hai event của cùng học sinh xử lý đồng thời gần ngưỡng cap, THÌ tổng XP trong ngày PHẢI không vượt cap (khóa dòng `xp_daily_caps`).
- NẾU curve hiện hành bị unpublish, THÌ module game PHẢI giữ curve version đã áp cho học sinh và không đổi level.
- NẾU `attempt.re_evaluated` đổi kết quả, THÌ XP đã cấp giữ nguyên (không thu hồi tự động); điều chỉnh chỉ qua bút toán bù (F075).
- NẾU attempt mode `exam`, THÌ chỉ áp rule có `attempt_modes` chứa `exam` (XP "hoàn thành" cố định).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAM-001 | XP, Level, danh hiệu (badge/collection: F075; season: F077) | [PROGRESSION_SYSTEM](../../docs/01-game-design/PROGRESSION_SYSTEM.md) |
| FR-GAM-002 | XP từ quest, activity, bài học, mastery, khám phá (challenge: F076; co-op/boss: F103; thi đấu: F105) | PROGRESSION_SYSTEM §3.1 |
| FR-GAM-003 | Reward rule, cap ngày, giảm dần khi lặp, chống farm | PROGRESSION_SYSTEM §3.2 |
| FR-GAM-004 | `xp_ledger` append-only, idempotent theo `(student_id, source_type, source_id, rule_id)` | [ADR-0049](../../docs/adr/ADR-0049-so-cai-phan-thuong-va-tien-te-trong-game-reward-currency-led.md) |
| FR-GAM-005 | Phần thưởng game không chạm điểm/mastery | [ADR-0018](../../docs/adr/ADR-0018-glicko-2-cho-game-rating-tach-biet-learning-mastery.md) |
| FR-GAM-006 | Ba miền tiến trình tách biệt | PROGRESSION_SYSTEM §2 |
| FR-GAM-007 | Catalog nguồn XP, đường cong có version, danh hiệu theo khoảng level | PROGRESSION_SYSTEM §4 |
| FR-GAM-008 | `level.reached`, phần thưởng lên cấp, progress, lịch sử level/XP | PROGRESSION_SYSTEM §4.4–4.6 |
| FR-GAM-010 | Chuyển năm học | PROGRESSION_SYSTEM §5.1 |
| FR-GAM-011 | Miễn giảm XP ôn LO tiên quyết (giảm trọng số evidence `timed_challenge`: F031) | PROGRESSION_SYSTEM §3.2 |
| FR-RWD-003 | Phần thưởng chỉ do server trao | [ANTI_CHEAT §7](../../docs/10-security/ANTI_CHEAT.md) |

NFR: NFR-SAFE-003 (không streak, không thưởng đăng nhập) — F074-AC-09.

### Key Entities

- **XpLedgerEntry**: bút toán XP append-only của một học sinh, gắn nguồn, rule đã áp, cờ cap.
- **DailyCapCounter**: XP đã cấp trong ngày theo bucket (tổng, nhóm nguồn) của một học sinh.
- **RewardRule / RewardRuleVersion**: quy tắc thưởng platform có version: trigger, điều kiện, grant, cap, giảm dần.
- **RewardGrant**: một lần trao thưởng idempotent (theo event gốc + rule, hoặc theo level).
- **ProgressionCurve / Version / Level**: đường cong XP có version, bảng level, danh hiệu, thưởng lên cấp.
- **LevelHistory**: mốc lên cấp append-only.
- **GameProfile (phần tiến trình)**: cache `level`, `xp_total`, `curve_version`, `title_code`.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `xp_curve`, `max_level`, `level_titles_at`, `level_floor_protection` | Giá trị mặc định của curve seed và kiểm tra |
| `xp_repeat_multipliers` | Hệ số giảm dần và cửa sổ |
| `xp_daily_cap` | Cap tổng XP/ngày |
| `xp_below_grade_multiplier` | Hệ số nội dung thấp hơn ≥ 2 khối |
| `xp_objective_mastered` | XP nguồn `mastery.objective_mastered` |
| `api_rate_default` | Rate limit API đọc tiến trình |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| XP chỉ từ event server; consumer idempotent (`processed_events`) + unique ledger | AGENTS R5, R10 · ADR-0042 · ADR-0049 |
| Ledger append-only; mọi bảng T có `tenant_id` + RLS | AGENTS R8, R21 · NFR-SCAL-004 |
| Không gộp ba miền tiến trình; không gọi đồng bộ module khác | AGENTS R13, R29 · FR-GAM-006 |
| XP hiển thị trên HUD ≤ 2 s sau event | AC-GAM-004 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Phát lại 1.000 event đã xử lý → 0 bút toán XP trùng.
- **SC-002**: 95% lần cộng XP đến client (`progression.xp.updated`) trong ≤ 2 giây kể từ khi event được publish.
- **SC-003**: 30 ngày chạy staging, job đối soát báo 0 lệch giữa `xp_total` và tổng ledger.
- **SC-004**: 0 trường hợp học sinh bị giảm level sau khi publish curve mới trong test 10.000 hồ sơ.
- **SC-005**: Quét spec: 0 request schema của học sinh chứa trường XP/điểm.

## Ngoài phạm vi

- Applier Xu, vật phẩm, huy hiệu, collection; ví, shop, achievement; bút toán bù (`createEconomyAdjustment`) → F075.
- XP từ challenge → F076; boss/co-op → F103; thi đấu (`xp_competition_matches_per_day`) → F105; điểm sự kiện/mùa → F077.
- Mở khóa vùng, khám phá, chương truyện → F071; quest engine và "Hành trình khối mới" → F072.
- HTTP workflow `/game-admin/*` (soạn, duyệt, publish) → F078; màn Level & XP Configuration → F079.
- School Learning Dashboard không hiển thị game level (AC-GAM-007) → F043/F056; trọng số evidence `timed_challenge` → F031.
- HUD/màn "Hành trình của tôi" trong Godot → nhóm F (chưa có mã trong danh mục).

## Assumptions

- Ngày của cap XP theo lịch `Asia/Ho_Chi_Minh` 00:00–24:00 [Confirmed]; cần ghi vào PROGRESSION_SYSTEM §3.2 (task Polish).
- Release 1.0 chỉ một progression curve platform đang dùng; tạo curve thứ hai trả `409 CONFLICT` [Confirmed].
- Kernel nội dung version hóa của module game (version + con trỏ `current_version_id`, `game.content_releases`, `content.published`) có từ F071/F072; F074 đăng ký module `progression-curves`, `reward-rules`. Trước F078, version published được nạp bằng lệnh seed của worker.
- module learning bổ sung trường cho `activity.graded` (delta §6) trước khi US1 Done; quest/minigame/world event payload do F071–F073 định nghĩa.
