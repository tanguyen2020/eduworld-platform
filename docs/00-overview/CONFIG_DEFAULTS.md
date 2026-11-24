# CONFIG_DEFAULTS — Sổ đăng ký tham số mặc định

> Trạng thái: Draft · Phiên bản: 1.1 · Owner: Technical PM + Solution Architect · Cập nhật: 2026-11-21 (ADR-0095: bỏ channel `class`, `study_group` khỏi `persistent_chat_retention_days`, `chat_allowed_hours`; đơn vị nghiên cứu cụm trường × khối — OQ-25 Answered: `efficacy_recommended_cohorts_per_arm` = 4 [Confirmed]) · 2026-11-21 (ADR-0094: `portal_session_sso` — phiên `platform_staff` tối đa 8 giờ thay step-up MFA; `portal_setup_ticket_ttl` không còn OTP) · 2026-11-20 (owner chốt OQ còn mở: `availability`, `maintenance_window`, `object_storage_backend` = S3 managed tại VN, `chat_allowed_hours`, `world_chat_scope`, `persistent_chat_retention_days`, `daily_play_limit_minutes`) · (2026-11-17: `common_student_batch_max` — ACCOUNT_PROVISIONING §3.4; ADR-0091: `portal_setup_ticket_ttl`, `portal_password_reset_ttl`, `portal_login_rate`, sửa `portal_session_sso`; ADR-0090: `feature_toggle_cache_ttl`; ADR-0074: tách `internal_api_resilience` thành `module_call_deadline` và `external_api_resilience`; đăng ký tham số feature spec; quyết định Technical Architect 2026-11-16 và 2026-11-17: ADR-0075, ADR-0078, ADR-0086 bổ sung)
> Liên quan: [docs/README](../README.md) · [PRODUCT_REQUIREMENTS](PRODUCT_REQUIREMENTS.md) · [GLOSSARY](GLOSSARY.md) · [OPEN_QUESTIONS](../12-project/OPEN_QUESTIONS.md)

## 1. Mục đích và quy tắc

Một chỉ mục duy nhất cho **mọi tham số mặc định cấu hình được** của EduWorld (thời gian, giới hạn, ngưỡng, tỷ lệ), để tránh mỗi tài liệu tự ghi một con số.

| Quy tắc | Nội dung |
|---|---|
| R1 — Một nơi sở hữu | Mỗi tham số có **đúng một tài liệu sở hữu** (cột "Sở hữu"). Tài liệu sở hữu là nguồn sự thật; tài liệu khác chỉ link, không chép lại con số |
| R2 — Đổi cùng lúc | Đổi giá trị phải sửa **tài liệu sở hữu và dòng ở đây trong cùng một merge request**; nếu là [Confirmed] cần quyết định stakeholder/ADR |
| R3 — Nhãn | Mặc định mọi giá trị là **[Confirmed]**; chỉ ghi [Confirmed] khi đã có quyết định stakeholder hoặc Master Prompt |
| R4 — Phạm vi | `platform` = chỉ đội platform cấu hình (Game Operations Console/Platform Administration); `tenant` = tenant được ghi đè trong giới hạn platform cho phép; `code` = hằng số trong mã/hạ tầng, đổi bằng release |
| R5 — Tên | Tên `snake_case`, dùng làm khóa cấu hình trong hệ thống (`platform_settings`, `tenant_settings`, cấu hình game có version) |

Quy ước sở hữu đã chốt cho các vùng chồng lấn: trần và giảm dần XP → [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md); trần Xu → [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md); [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) chỉ sở hữu ngưỡng phát hiện và xử lý gian lận; nhắc nghỉ → [CHILD_SAFETY](../10-security/CHILD_SAFETY.md); lời mời party → [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md); SLA báo cáo → [MODERATION](../03-social-multiplayer/MODERATION.md); giới hạn độ dài và rate limit chat → [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §3.1; ngưỡng kiểm duyệt (spam, danh mục từ) → [MODERATION](../03-social-multiplayer/MODERATION.md); retention dữ liệu cá nhân → [DATA_PRIVACY §2](../10-security/DATA_PRIVACY.md); rate limit API → [SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md).

## 2. Thế giới và phiên chơi

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `session_target_minutes` | 15–30 | phút | code | [Confirmed] | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) |
| `learning_block_max_items` | g1_3: 3 · g4_5: 5 · g6_9: 8 | câu liên tiếp trước một nhịp game | platform | [Confirmed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `hub_instance_capacity` | 40 | người | platform | [Confirmed] (ADR-0012) | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `hub_instance_idle_close` | 5 | phút | code | [Confirmed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `private_instance_capacity` | ≤ 8 | người | platform | [Confirmed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `battle_room_size` | 2–8 | người | platform | [Confirmed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `area_transition_max` | < 5 | giây | code (NFR) | [Confirmed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `discovery_report_rate` | ≤ 30 | lần/phút | code (env `EW_GAME_DISCOVERY_REPORT_RATE`, 1–120) | [Confirmed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `interact_radius_default` | 3 | m | code | [Confirmed] (F071, owner chốt 2026-11-20) | [WORLD_DESIGN §8](../01-game-design/WORLD_DESIGN.md#8-khám-phá-và-hoàn-thành-bản-đồ) |
| `interact_distance_tolerance` | 1,5 | m | code | [Confirmed] (F071, owner chốt 2026-11-20) | [WORLD_DESIGN §8](../01-game-design/WORLD_DESIGN.md#8-khám-phá-và-hoàn-thành-bản-đồ) |
| `party_seat_hold` | 30 | giây | code | [Confirmed] | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |

## 3. Progression và XP

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `xp_curve` | `power_linear`: base 100, exponent 1,35, linear 20 | — | platform (version hóa: curve seed `seed/progression/curve.default.v1.json` — F074) | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `max_level` | 60 | level | platform | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `level_titles_at` | 1 / 10 / 20 / 35 / 50 | level | platform | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `level_floor_protection` | true | — | platform | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_repeat_multipliers` | 100% / 50% / 25% / 0% trong 7 ngày (theo content version) | — | platform (env `EW_GAME_XP_REPEAT_MULTIPLIERS` danh sách 0–1, `EW_GAME_XP_REPEAT_WINDOW_DAYS` 1–90, tới khi có khóa platform settings; rule có `diminishing_returns` riêng thì dùng của rule — F074) | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_daily_cap` | 1.000 | XP/ngày (ngày lịch ICT) | platform (env `EW_GAME_XP_DAILY_CAP`, 1–100000 — F074) | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_competition_matches_per_day` | 10 | trận/ngày | platform | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_below_grade_multiplier` | ×0,25 khi nội dung thấp hơn ≥ 2 khối | — | platform (env `EW_GAME_XP_BELOW_GRADE_MULTIPLIER`, 0–1 — F074) | [Confirmed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_objective_mastered` | 50 XP, một lần cho mỗi LO khi đạt mức mastered | XP | platform (reward rule seed `xp.objective_mastered` — F074) | [Confirmed — Technical Architect 2026-11-15] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) §3.1 |

## 4. Quest

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `daily_quests_per_day` | 3 | quest | platform (env `EW_GAME_DAILY_QUESTS_PER_DAY`, 0–10, tới khi có khóa platform settings — F072) | [Confirmed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `weekly_quests_per_week` | 3 | quest | platform (env `EW_GAME_WEEKLY_QUESTS_PER_WEEK`, 0–10 — F072) | [Confirmed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `quest_reset_time` | 04:00 ICT; weekly: thứ Hai 04:00 | — | platform (env `EW_GAME_QUEST_RESET_TIME` HH:MM ICT — F072) | [Confirmed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `max_active_quests` | 10 | quest | platform (env `EW_GAME_MAX_ACTIVE_QUESTS`, 1–50 — F072) | [Confirmed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `quest_pass_rule` | tỷ lệ đúng 0,7; tối thiểu 8 câu; 5 lần/ngày | — | platform (theo quest) | [Confirmed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `coop_quest_reconnect_grace` | 120 | giây | platform | [Confirmed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `interdependent_roles` | 2–4 vai mỗi bước `interdependent` | vai | platform | [Confirmed — Technical Architect 2026-11-15] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) §3.2 |

## 5. Challenge và boss

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `difficulty_levels` | Easy, Normal, Hard, Expert, Legendary (bảng tham số theo mức) | — | platform (hằng số code của bảng sở hữu; version challenge ghi đè theo profile — F076) | [Confirmed] tên mức · [Confirmed] tham số | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `g1_3_challenge_rules` | thời gian ×1,5; chỉ Easy–Hard | — | platform | [Confirmed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `challenge_attempts_per_day` | 5 | lượt | platform (env `EW_GAME_CHALLENGE_ATTEMPTS_PER_DAY` — F076) | [Confirmed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `challenge_fail_cooldown` | 60 (từ mức Hard) | giây | platform (env `EW_GAME_CHALLENGE_FAIL_COOLDOWN` — F076) | [Confirmed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `challenge_survival_regen_streak` | 3 (vòng đúng hết liên tiếp hồi 1 sức bền, không vượt sức bền của mức) | vòng | code | [Confirmed] (F076) | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) §2.1 |
| `challenge_room_lease` | lease 10 s, gia hạn 3 s, quét room mất owner 2 s, timer 250 ms (room `challenge_solo`, `src/backend/pkg/roomhost`) | — | code | [Confirmed] (F076) | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) §4 |
| `challenge_deadline_scan` | 15 (job `challenge-deadline`, dự phòng timer room host) | giây | code | [Confirmed] (F076, owner "chốt hết" 2026-11-24) | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) §2.3 |
| `boss_reconnect_grace` | 120 | giây | platform | [Confirmed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |

## 6. Kinh tế, vật phẩm, sưu tầm

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `xu_daily_cap` | 300 | Xu/ngày (ngày lịch ICT) | platform (`gamification.currencies.daily_cap` của dòng `xu`, migration `f075_economy`; Game Ops sửa qua config type `currencies` — F075) | [Confirmed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `duplicate_item_xu` | 5 / 10 / 25 / 50 / 100 theo độ hiếm | Xu | platform (env `EW_GAME_DUPLICATE_ITEM_XU` = `common=5,uncommon=10,rare=25,epic=50,legendary=100`, mỗi giá trị 1–100000 — F075) | [Confirmed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `shop_purchase_limit_default` | 1 | lần/item | platform (theo item; env `EW_GAME_SHOP_PURCHASE_LIMIT_DEFAULT` 1–999 — F075) | [Confirmed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `badges_displayed_max` | 3 | huy hiệu | platform | [Confirmed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `trading_enabled` | false | — | platform | [Confirmed] (tắt đến khi được review) | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `real_money_purchase` | không có | — | — | [Confirmed, OQ-14] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |

## 7. Nhân vật

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `walk_speed` / `run_speed` | 3,0 / 5,5 | m/s | code | [Confirmed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `jump_height` | 1,0 | m | code | [Confirmed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `camera_distance` | 4–8 | m | code | [Confirmed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `emote_rate` | 1 lần/2 s | — | code | [Confirmed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `emote_wheel_slots` | 6 | ô | code | [Confirmed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `avatar_creation_presets` / `saved_outfits_max` | 6 / 3 | bộ | platform | [Confirmed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `nickname_change_monthly_limit` | 1 (trường có thể siết thêm, không nới) | lần/tháng | tenant | [Confirmed — Technical Architect 2026-11-16] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) §5.3 |
| `nickname_suggestion_ttl` | 10 | phút | code | [Confirmed — Technical Architect 2026-11-16] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) §5.3 |

## 8. Thi đấu, ranking, leaderboard, tournament

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `battle_1v1_format` | 7 câu × 20 s (g1_3: 30 s); 100 điểm + bonus tốc độ ≤ 50; câu cuối ×2 | — | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `team_battle_format` | 10 câu; lượt hỗ trợ 30% điểm | — | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `timed_quiz_format` | 10 câu × 20 s (g1_3: 30 s); tối đa 40 học sinh; top 5 từ lớp 4; `grade_goal` = 60% (người × câu) cho lớp 1–3 | — | platform | [Confirmed — owner chốt 2026-11-20] | [F105 Clarifications 2026-11-21](../../specs/105-battle-engine/spec.md#session-2026-11-21-triển-khai--confirmed-owner-chốt-2026-11-20) |
| `coop_challenge_format` | 12 câu × 25 s; mục tiêu 10 câu đúng tập thể; grace 120 s | — | platform | [Confirmed — owner chốt 2026-11-20] | [F105 Clarifications 2026-11-21](../../specs/105-battle-engine/spec.md#session-2026-11-21-triển-khai--confirmed-owner-chốt-2026-11-20) |
| `battle_room_timers` | lobby chờ 15 s; đếm ngược 3 s; reveal 3 s; chờ chấm 3 s; chờ reveal 2 s; trễ chấp nhận 300 ms; RTT trừ 150 ms; làm lại câu tối đa 2 lần | — | platform | [Confirmed — owner chốt 2026-11-20] | [F105 Clarifications 2026-11-21](../../specs/105-battle-engine/spec.md#session-2026-11-21-triển-khai--confirmed-owner-chốt-2026-11-20) |
| `bot_profiles` | lớp 1–3: 60%, 6–18 s; lớp 4–5: 70%, 4–14 s; lớp 6–9: 75%, 3–12 s | — | platform | [Confirmed — owner chốt 2026-11-20] | [F105 Clarifications 2026-11-21](../../specs/105-battle-engine/spec.md#session-2026-11-21-triển-khai--confirmed-owner-chốt-2026-11-20) |
| `grade_window_g6_9` | ±1 khối sau 30 s chờ (g1_5 luôn cùng khối) | — | platform | [Confirmed] nguyên tắc · [Confirmed] số | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `glicko2_params` | r0 1500, RD 350, σ 0,06, τ 0,5; RD 50–350 | — | code | [Confirmed] (ADR-0018) | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `rating_provisional` | < 10 trận hoặc RD > 110 | — | code | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `season_length` | 8 | tuần | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `season_soft_reset` | hệ số 0,5; RD sau reset ≥ 150 | — | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `tiers` | 6 tier; hysteresis 50; ẩn số rating thô với mọi học sinh | — | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `competition_points_daily_cap` | 300; quá 20 trận/ngày không cộng | điểm | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `cheat_fast_answer_flag` | < 400 ms (g1_5: < 600 ms) liên tiếp ≥ 3 câu | — | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `leaderboard_visibility` | g1_3: không xếp hạng cá nhân; g4_5: top 10; g6_9: top 50 + ±3 quanh mình | — | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `tournament_timers` | check-in 15 phút; no-show 3 phút; disconnect grace 60 s × 2; khiếu nại 10 phút | — | platform | [Confirmed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `speed_bonus_min_grade` | 6 (lớp 1–5 không có bonus tốc độ ở 1v1 và Timed Quiz) | khối | platform | [Confirmed — Technical Architect 2026-11-15] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) §3.1 |
| `effort_points` | +1/hoạt động (tối đa 10/ngày), +2/hoạt động ôn tập, +3/ngày hoạt động; trần 25/ngày | điểm | platform | [Confirmed — Technical Architect 2026-11-15] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) §9.2 |
| `disabled_leaderboard_definitions` | Rỗng (chính sách trường; school_admin tắt được bảng khối/trường của trường mình — không còn bảng lớp, ADR-0095) | — | tenant | [Confirmed — Technical Architect 2026-11-15] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) §9.2 |
| `grade_goal_target` | 300 điểm nỗ lực/tuần cho mục tiêu của khối (`display.grade_goal_target` của definition `grade_goal_weekly`) | điểm | platform | [Confirmed — F106 2026-11-22] | [F106 Clarifications 2026-11-22](../../specs/106-ranking-leaderboard/spec.md#session-2026-11-22-triển-khai--confirmed-owner-chốt-2026-11-20) |
| `leaderboard_friends_max` | 200 bạn gần đây nhất được tính ở bảng bạn bè (`friends_max` của Ranking Rules) | bạn | platform | [Confirmed] (edge case F106) | [F106 spec](../../specs/106-ranking-leaderboard/spec.md#edge-cases) |

### 8.1. Sự kiện và season track

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `event_claim_grace_hours` | 72 | giờ | platform (theo sự kiện: `schedule.claim_grace_hours`; mặc định env `EW_GAME_EVENT_CLAIM_GRACE_HOURS` 0–336 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_festival_min_duration_days` | 7 | ngày | platform (env `EW_GAME_EVENT_FESTIVAL_MIN_DAYS` 1–60 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_extra_daily_quests_max` | 2 | quest/ngày | platform (env `EW_GAME_EVENT_EXTRA_DAILY_QUESTS_MAX` 0–20 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_points_daily_cap` | 200 | điểm sự kiện/ngày | platform (theo sự kiện: `event_points.daily_cap`; mặc định env `EW_GAME_EVENT_POINTS_DAILY_CAP` 1–10000 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_upcoming_visible_days` | 7 | ngày | platform (env `EW_GAME_EVENT_UPCOMING_VISIBLE_DAYS` 0–60 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_rule_change_effective` | 04:00 ICT hôm sau | — | code (giờ ICT của ngày sau: env `EW_GAME_EVENT_RULE_CHANGE_HOUR` 0–23, mặc định 4 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `community_goal_contribution_daily_cap` | 50 | đơn vị/học sinh/ngày | platform (theo goal: `per_student_daily_cap`; mặc định env `EW_GAME_CGOAL_CONTRIBUTION_DAILY_CAP` 1–10000 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `community_goal_progress_flush_seconds` | 30 (nhịp đẩy tiến độ community goal tới client) | giây | platform (env `EW_GAME_CGOAL_FLUSH_SECONDS` 1–3600 — F077) | [Confirmed — Technical Architect 2026-11-16] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_track_tiers` / `season_track_points_per_tier` | 30 cấp / 100 điểm | — | platform (theo track) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_points_daily_cap` | 100 | điểm mùa/ngày | platform (theo track) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_point_sources` | daily quest 20 · weekly quest 50 · hoạt động học tập 5 · hoạt động sự kiện 10 · co-op 10 · trận 5 | điểm | platform (theo track) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_unclaimed_rewards` | tự cấp khi hết mùa | — | code | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `school_event_max_duration_days` | 28 | ngày | platform (env `EW_GAME_SCHOOL_EVENT_MAX_DAYS` 1–28 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `school_events_concurrent_per_tenant` | 3 | sự kiện | platform (env `EW_GAME_SCHOOL_EVENTS_CONCURRENT` 1–50 — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_decoration_bundle_max_mb` | 20 | MB (mobile) | platform (env `EW_GAME_EVENT_DECORATION_BUNDLE_MAX_MB` 1–1024; kích thước gói khai báo ở `decoration.bundle_size_bytes` — F077) | [Confirmed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |

## 9. Party, bạn bè, club

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `party_max_members` | 4 (cấu hình 2–8) | người | platform + tenant | [Confirmed] cấu hình được · [Confirmed] số | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_invite_ttl` | 5 | phút | platform | [Confirmed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_invite_rate` | 10/phút, 50/ngày mỗi học sinh | lời mời | platform | [Confirmed — Technical Architect 2026-11-16] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) §10 |
| `party_invite_repeat_rules` | 1 lời mời pending tới cùng một người; bị từ chối → chờ 5 phút; 3 lần bị từ chối/ngày → chặn mời người đó đến hết ngày | — | platform | [Confirmed — Technical Architect 2026-11-16] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) §10 |
| `party_pending_invites_max` | 20 lời mời đang chờ mỗi người nhận; vượt → lời mời cũ nhất hết hạn | lời mời | platform | [Confirmed — Technical Architect 2026-11-16] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) §10 |
| `party_create_rate` | 10 lần tạo/giải tán mỗi giờ | lần/giờ | platform | [Confirmed — Technical Architect 2026-11-16] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) §10 |
| `party_kick_rate` | 10 mỗi giờ mỗi leader | lần/giờ | platform | [Confirmed — Technical Architect 2026-11-16] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) §10 |
| `party_lobby_cmd_rate` | 5 (ready, chọn hoạt động) | lệnh/giây | platform | [Confirmed — Technical Architect 2026-11-16] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) §10 |
| `party_activity_min_members_timeout` | 120 (số thành viên online < `min_members` quá ngưỡng → hoạt động kết thúc `timeout`) | giây | platform | [Confirmed — Technical Architect 2026-11-16] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) §9 |
| `party_leader_grace` | 60 | giây | platform | [Confirmed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_dormant_timeout` | 30 | phút | platform | [Confirmed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_kick_reinvite_block` | 10 | phút | platform | [Confirmed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `friend_limits` | ≤ 10 lời mời đang chờ; ≤ 20 lời mời/ngày; ≤ 100 bạn | — | platform | [Confirmed] | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |
| `friend_request_ttl` / `friend_code_ttl` | 7 ngày / 10 phút | — | platform | [Confirmed] | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |
| `club_max_members` / `clubs_per_student_max` | 50 / 5 | — | platform + tenant | [Confirmed] (ADR-0050) | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |
| `club_invitation_ttl` | 7 (lời mời club hết hạn → `410`; env `EW_CHAT_CLUB_INVITATION_TTL`) | ngày | platform | [Proposed — F120 2026-11-23] | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) §6.5 |
| `student_club_creation` | `disabled` (chính sách trường; giá trị `disabled` / `requires_approval` / `auto`) | — | tenant | [Confirmed — Technical Architect 2026-11-15] | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) §6.1 |
| `cross_school_social_enabled` | `false` (chỉ áp dụng tenant `unit`; bật → kết bạn, DM, party, club, World mở rộng, ghép trận liên trường trong tenant) | — | tenant | [Confirmed — Technical Architect 2026-11-16] (ADR-0054) | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) §7 |

## 10. Matchmaking

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `mm_rating_window` | 100 / 200 / 350 / ∞ tại 0 / 10 / 20 / 40 s | — | platform | [Confirmed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_ready_check` | 15 | giây | platform | [Confirmed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_bot_after` | 90 (bot luyện tập unranked) | giây | platform | [Confirmed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_ticket_ttl` / `mm_tickets_per_user` | 5 phút / 1 | — | platform | [Confirmed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_ticket_rate` | 30/giờ; cooldown 2 phút sau 3 lần từ chối/hết hạn ready check trong 10 phút (cùng giới hạn áp cho `createDuel` — F104) | — | platform | [Confirmed] | [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) |

## 11. Multiplayer và realtime

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `disconnect_grace` | hub 30 s; quest co-op 120 s; boss 120 s; lobby 10 s; 1v1 30 s; đội 45 s; giải 60 s (tối đa 2 lần) | — | platform | [Confirmed] | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| `abandon_penalty` | ≥ 3 trận/24 h → hạn chế ranked 15 phút | — | platform | [Confirmed] | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| `realtime_ticket_ttl` | 60 s, dùng 1 lần | — | code | [Confirmed] (ADR-0008) | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `ws_heartbeat` / `ws_timeout` / `ws_ping` | 15 / 45 / 20 | giây | code | [Confirmed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `ws_max_frame` | 16 | KB | code | [Confirmed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `resume_window` | 120 s, ≤ 500 message | — | code | [Confirmed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `nats_request_timeout` | 2 | giây | code | [Confirmed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `hub_sync_rate` | 10 | Hz | code | [Confirmed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `ws_per_gateway_node` | 4.000 | kết nối | code | [Confirmed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `ws_frame_rate` | Tổng frame 30/s, burst 60; `hub.player.move` 12/s; vi phạm liên tục > 3 lần/phút → đóng `4429` | frame/giây/kết nối | code | [Confirmed — Technical Architect 2026-11-16] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) §8.1 |
| `ws_inflight_commands` | 8 (vượt → `RATE_LIMITED`) | NATS request/kết nối | code | [Confirmed — Technical Architect 2026-11-16] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) §8 |
| `ws_send_queue` | 256 message; đầy → bỏ message `droppable`; message quan trọng không đẩy được trong 5 s → đóng `4408`; write deadline 10 s | — | code | [Confirmed — Technical Architect 2026-11-16] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) §8 |
| `ws_accept_rate` | 200 kết nối mới/giây mỗi node; quá tải → close `4429` kèm thời gian thử lại | kết nối/giây | code | [Confirmed — Technical Architect 2026-11-16] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) §11 |
| `ws_protocol_errors` | 5 lỗi giao thức (envelope sai, `type` lạ, frame quá lớn) trong 1 phút → đóng `4400` | lần/phút/kết nối | code | [Confirmed] (F090) | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) §8 |
| `ws_drain_window` | 15 s (≤ 30 s, trong `EW_HTTP_SHUTDOWN_TIMEOUT`): rải việc đóng `1012` sau `system.server.draining` khi tắt node | giây | code | [Confirmed] (F090) | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) §3.1 |
| `game_session_heartbeat_timeout` | 45 s không heartbeat → kết thúc game session (`end_reason = timeout`); job `game-session-timeout` quét mỗi 15 s | giây | code | [Confirmed — Technical Architect 2026-11-16] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) §3.2 |
| `audit_hub_membership` | bật (pilot); bản ghi `session_memberships` của hub giữ 90 ngày | — | platform + tenant | [Confirmed — Technical Architect 2026-11-16] | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) §10 |
| `client_reconnect_backoff` | 0,5–15 s, jitter ±20% | — | code | [Confirmed] | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| `client_interpolation` | 100 ms (tối đa 200) | — | code | [Confirmed] | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| `client_input_rate` / `client_command_timeout` | ≤ 10 Hz / 10 s | — | code | [Confirmed] | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| `presence_ttl` / `active_session_ttl` | 90 | giây | code | [Confirmed] | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) |

## 12. Chat và kiểm duyệt

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `chat_max_length` | 120 / 200 / 300 theo loại channel (trần DB 1.000) | ký tự | platform | [Confirmed] (ADR-0041) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_edit_window` / `chat_recall_window` | 15 phút / 24 giờ | — | platform | [Confirmed] (ADR-0034) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_pins_max` / `chat_mentions_max` / `chat_reactions_per_user` | 3 / 5 / 3 | — | platform | [Confirmed] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_forward_targets_max` | 5 | channel | platform | [Confirmed] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `gameplay_chat_retention_days` | 7 (tối thiểu 3) | ngày | platform + tenant | [Confirmed] (ADR-0033) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `ephemeral_chat_retention` | 24 (World/Area) | giờ | platform | [Confirmed] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_recent_buffer` | 200 tin / 10 phút | — | code | [Confirmed] (ADR-0035) | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) |
| `chat_rate_per_user` | ≤ 30 tin/phút trên mọi channel; rate theo channel (msg/phút, burst) theo bảng mặc định platform | tin/phút | platform | [Confirmed — Technical Architect 2026-11-16] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §3.1 |
| `chat_burst_window` | 10 (tối đa `rate.burst` tin của một người gửi trong một channel trong mọi cửa sổ 10 s; slow mode = khoảng cách tối thiểu giữa hai tin của người đó trong channel) | giây | code | [Confirmed — owner 2026-11-20, F116 Clarifications 2026-11-21] | [MODERATION](../03-social-multiplayer/MODERATION.md) §3.4 |
| `persistent_chat_retention_days` | 180 (channel `direct`, club; tin bị report giữ theo case — channel `class`, `study_group` bỏ theo ADR-0095; đọc `chat_retention.persistent_days`, dự phòng `EW_CHAT_PERSISTENT_RETENTION_DAYS`; job `persistent-chat-retention` mỗi giờ, lô 500 — F120) | ngày | platform | [Confirmed — Technical Architect 2026-11-16] tên · [Confirmed — owner 2026-11-20, OQ-09] giá trị | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §2 |
| `chat_publish_ack_timeout` | 2 s chờ JetStream PubAck; quá hạn → gửi thất bại, không fan-out | giây | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0035) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §5.2 |
| `chat_persister_batch` | ≤ 500 sự kiện hoặc 50 ms mỗi lô; `max_deliver` 5 → DLQ + alert | — | code | [Confirmed — Technical Architect 2026-11-16] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §5.3 |
| `chat_facts_cache` | 30 s (channel, thành viên, thẻ, block) / 60 s (giới nghiêm) — độ tươi facts của đường gửi trong Valkey | giây | code | [Confirmed F115] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §5.4 |
| `chat_facts_stale_ttl` | 1 giờ — giữ facts cũ để vẫn gửi/nhận khi PostgreSQL ngừng; hết hạn → từ chối gửi | giờ | code | [Confirmed F115] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §5.4 |
| `gameplay_chat_purge_job` | chạy hằng giờ; lô 1.000 dòng; cảnh báo khi tồn đọng quá hạn > 2 ngày (env `EW_CHAT_PURGE_INTERVAL`, `EW_CHAT_PURGE_BATCH`, `EW_CHAT_PURGE_BACKLOG_ALERT` — F117) | — | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0033) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §6.1 |
| `chat_message_partition_days_ahead` | 9 (partition ngày của `chat.gameplay_messages`, `chat.ephemeral_messages` tạo trước; cảnh báo khi < 2; env `EW_CHAT_PARTITION_DAYS_AHEAD`) | ngày | code | [Confirmed F117 — 2026-11-22] (ADR-0033) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §6.1 |
| `chat_sticker_gif_rate` | 4 (sticker và GIF tính chung) | lượt/phút/người gửi/channel (`EW_CHAT_STICKER_GIF_RATE`, F118) | platform | [Confirmed — Technical Architect 2026-11-16] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §7.5 |
| `chat_asset_file_max` | sticker WebP/PNG ≤ 512 KB; GIF ≤ 2 MB, ≤ 480 px, có preview tĩnh | — | code | [Confirmed — Technical Architect 2026-11-16] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §7.5 |
| `chat_typing_throttle` | 1 lần/3 s; tự hết sau 5 s không cập nhật | — | code | [Confirmed — Technical Architect 2026-11-16] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §7.10 |
| `chat_receipt_coalesce` | ≤ 1 lần/2 s mỗi thành viên mỗi channel | — | code | [Confirmed — Technical Architect 2026-11-16] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §8.1 |
| `chat_spam_thresholds` | ≥ 3 tin giống nhau (similarity ≥ 0,9) trong 60 s → block tin lặp; cùng nội dung ≥ 4 channel trong 2 phút → block + tăng điểm spam; > 70% chữ hoa hoặc > 10 emoji liên tiếp → mask/giảm; 3 lần block trong 10 phút → auto-mute (`auto_mute_spam`) | — | platform | [Confirmed — Technical Architect 2026-11-16] | [MODERATION](../03-social-multiplayer/MODERATION.md) §3.4 |
| `sensitive_terms_engine_limits` | RE2 timeout 5 ms/pattern; tối đa 200 pattern/tenant | — | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0030) | [MODERATION](../03-social-multiplayer/MODERATION.md) §11.5 |
| `moderation_config_propagation_max` | ≤ 60 s từ khi publish danh mục từ hoặc đổi công tắc kiểm duyệt đến khi mọi node áp dụng (pub/sub + biên dịch nền) | giây | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0030) | [MODERATION](../03-social-multiplayer/MODERATION.md) §11.6, §12.2 |
| `moderation_snapshot_retention` | đến khi case đóng + 1 năm | — | platform | [Confirmed — Technical Architect 2026-11-16] | [MODERATION](../03-social-multiplayer/MODERATION.md) §10 |
| `moderation_switches` | bật tất cả bước; bước `pii` và phát hiện grooming **luôn chạy**, không nằm trong công tắc | — | platform + tenant (chỉ System Admin) | [Confirmed] cơ chế (ADR-0030) · [Confirmed — Technical Architect 2026-11-17] phạm vi (ADR-0078) | [MODERATION](../03-social-multiplayer/MODERATION.md) §12.2 |
| `moderation_switch_max_duration` | 24 (mọi lần tắt một bước bắt buộc có `expires_at` ≤ 24 giờ; hết hạn tự bật lại) | giờ | platform | [Confirmed — Technical Architect 2026-11-17] (ADR-0078) | [MODERATION](../03-social-multiplayer/MODERATION.md) §12.2 |
| `moderation_switch_approvals` | 2 (người đề xuất + một System Admin khác phê duyệt; không tự duyệt) | người | code | [Confirmed — Technical Architect 2026-11-17] (ADR-0078) | [MODERATION](../03-social-multiplayer/MODERATION.md) §12.2 |
| `chat_allowed_hours` | 06:30–21:30 ICT cho channel bền vững (`direct`, club, `world`/`area`; `class`, `study_group` bỏ theo ADR-0095); luôn nằm ngoài `platform_curfew`; trường chỉ thu hẹp; channel `match`/`team`/`party` không có khung giờ riêng (vẫn dừng trong giờ giới nghiêm) | — | platform + tenant (chỉ thu hẹp) | [Confirmed — owner 2026-11-20, OQ-09] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §3.1 |
| `world_chat_scope` | `school`; tenant `unit` có thể bật `tenant`; không có phạm vi organization/liên tenant; tenant `common` tắt World (ADR-0078) | — | tenant | [Confirmed — owner 2026-11-20, OQ-09] (ADR-0054) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) §2 |
| `chat_policy_preset` | `standard`; trường có thể chọn `quick_message_emote_only` (chỉ quick message + emote, áp cho cả trường, không theo khối) | — | tenant (cấp trường) | [Confirmed — Technical Architect 2026-11-17] (ADR-0078) | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) §3.5 |
| `common_tenant_social_defaults` | Tenant `common`: tắt kết bạn, DM, World chat; giờ giới nghiêm bắt buộc; kiểm duyệt bởi Platform Safety Team | — | platform | [Confirmed — Technical Architect 2026-11-17] (ADR-0078) | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) §3.6 |
| `moderation_pipeline_latency` | p95 < 20 ms (trần 50 ms) | — | code | [Confirmed] | [MODERATION](../03-social-multiplayer/MODERATION.md) |
| `report_sla` | p1: phản hồi 1 giờ / xong 24 giờ; p2: 1 / 3 ngày học; p3: 3 / 7 ngày học | — | platform | [Confirmed] | [MODERATION](../03-social-multiplayer/MODERATION.md) |
| `auto_mute_spam` | 10 phút → 1 giờ khi tái phạm | — | platform | [Confirmed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `filter_quality_gate` | recall ≥ 95%; false positive ≤ 3% | — | release gate | [Confirmed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `school_notification_ack_sla_hours` | 24 (p1, tự hại) · 72 (khác) | giờ | platform | [Confirmed — Technical Architect 2026-11-15] | [MODERATION](../03-social-multiplayer/MODERATION.md) §6.4 |
| `p1_unclaimed_alert_minutes` | 15 (cảnh báo on-call khi case p1 chưa có người nhận) | phút | platform | [Confirmed — Technical Architect 2026-11-15] | [MODERATION](../03-social-multiplayer/MODERATION.md) §6.3 |
| `oncall_shift_max_hours` | 168 (độ dài tối đa một ca trực Safety Team; env `EW_ONCALL_SHIFT_MAX_HOURS`) | giờ | platform | [Confirmed — F124 2026-11-16] | [MODERATION](../03-social-multiplayer/MODERATION.md) §6.3 |
| `break_glass_max_duration_hours` | 4 (thời hạn tối đa của quyền break-glass sau khi duyệt; env `EW_BREAK_GLASS_MAX_DURATION_HOURS`, chỉ được siết) | giờ | platform | [Confirmed — F124 2026-11-16] | [MODERATION](../03-social-multiplayer/MODERATION.md) §6.1 |
| `break_glass_max_window_days` | 30 (khoảng thời gian tin tối đa một yêu cầu break-glass được đọc; env `EW_BREAK_GLASS_MAX_WINDOW_DAYS`, chỉ được siết) | ngày | platform | [Confirmed — F124 2026-11-16] | [MODERATION](../03-social-multiplayer/MODERATION.md) §6.1 |
| `email_delivery_max_attempts` | 8 lần gửi email nhân sự (thông báo vụ việc, nhắc lại, gọi trực, break-glass), backoff lũy thừa 1, 2, 4… phút tới 1 giờ; hết lượt → `failed` + alert `EmailDeliveryFailing` (env `EW_EMAIL_DELIVERY_MAX_ATTEMPTS`) | lần | platform | [Confirmed — F124 2026-11-16] | [MODERATION](../03-social-multiplayer/MODERATION.md) §6.4 |

## 13. Học tập

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `pilot_efficacy_threshold` | Hedges' g ≥ 0,2 (mô hình hồi quy đa mức, điểm trước làm biến kiểm soát) với khoảng tin cậy 95% nằm trên 0, tỉ lệ rời nghiên cứu < 20%, tối thiểu 8 tuần; tỉ lệ học sinh tiến bộ báo cáo kèm, không là ngưỡng | — | pilot | [Confirmed — owner 2026-11-20] (ADR-0080) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §7 |
| `efficacy_min_questions_per_lo_per_form` | 3 | câu mỗi LO mỗi dạng đề | platform | [Confirmed] (F035) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §3.2 |
| `efficacy_attrition_warn` | ≥ 20% (cảnh báo tỉ lệ rời nghiên cứu, báo cáo phân tích độ nhạy) | — | platform | [Confirmed] (F035) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §4 |
| `efficacy_recommended_cohorts_per_arm` | 4 cụm (trường, khối) mỗi nhóm; dưới mức này báo cáo gắn `descriptive_only` (ADR-0095, OQ-25 Answered 2026-11-21; thay `efficacy_recommended_classes_per_arm` = 6 lớp — đổi tên trong code ở WP-L2, `efficacy.Config.RecommendedCohortsPerArm`) | cụm mỗi nhóm | platform | [Confirmed] (F035, OQ-25) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §4 |
| `efficacy_validity_rho_min` | 0,4 (LO dưới ngưỡng được đánh dấu) | Spearman ρ | platform | [Confirmed] (F035) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §5 |
| `efficacy_validity_calibration_min` | 70% học sinh thỏa | — | platform | [Confirmed] (F035) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §5 |
| `efficacy_validity_auc_min` | 0,7 | AUC | platform | [Confirmed] (F035) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §5 |
| `experiment_guardrail_drop_pp` | ≥ 5 (arm thấp hơn control từ mức này thì dừng sớm) | điểm phần trăm | platform | [Confirmed] (F035) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §6 |
| `experiment_eval_delay_days` | 7 (độ trễ đánh giá chỉ số chính: câu mới cùng LO) | ngày | platform | [Confirmed] (F035) | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) §6 |
| `attempt_idle_timeout` | 5 | phút | platform | [Confirmed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `attempt_resume_window` | 24 giờ (practice/retry/quest/challenge) | — | platform (module game: env `EW_GAME_ATTEMPT_RESUME_WINDOW` cho challenge không giới hạn giờ — F076) | [Confirmed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `deadline_grace` | 2 | giây | code | [Confirmed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `suspicious_speed_flag` | < 1,5 s/câu trên ≥ 10 câu | — | platform | [Confirmed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `score_scale` | thang 10, làm tròn 1 chữ số | — | tenant | [Confirmed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `attempts_per_question` | g1_3: 3; khối khác: 2 | lượt | platform | [Confirmed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `retry_wrong_schedule` | 1–3–7 ngày | — | platform | [Confirmed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `question_font_px` | 32 / 28 / 24 / 22 (g1_3 / g4_5 / g6_7 / g8_9) | px | code | [Confirmed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `mastery_model` | EWMA có trọng số theo ngữ cảnh, prior 0,30, ngưỡng mức | — | platform (version hóa) | [Confirmed] (ADR-0017) | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) |
| `latex_max_length` / `latex_max_depth` / `math_svg_max` | 2.000 ký tự / 20 / 256 KB | — | code | [Confirmed] (ADR-0055) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `publish_to_visible_max` | ≤ 30 | giây | code (SLO) | [Confirmed] (ADR-0043) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `video_completion` | xem thật ≥ 90% + trả lời câu chèn bắt buộc; heartbeat 10 s | — | platform | [Confirmed] | [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) |
| `video_recommended_length` | ≤ 8 phút (g1_5); ≤ 15 phút (g6_9) | — | hướng dẫn biên tập | [Confirmed] | [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) |
| `attempt_start_rate` | 20/giờ | lượt | platform | [Confirmed] | [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) |
| `w_hint` | 1 / 0,6 / 0,3 (0 / 1 / ≥ 2 gợi ý; chỉ áp khi trả lời đúng) | hệ số | platform | [Confirmed — Technical Architect 2026-11-15] | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) §3.1 |
| `guess_coefficient` | true_false 0,5 · single_choice 1/k · multiple_choice, matching, ordering, drag_drop 0,1 · dạng tự nhập 0 | hệ số | platform | [Confirmed — Technical Architect 2026-11-15] | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) §3.1 |
| `rapid_guess_min_response_ms` | 1,5 / 2 / 3 / 4 s theo loại câu; × 1,5 cho g1_3 | ms | platform | [Confirmed — Technical Architect 2026-11-15] | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) §3.1 |
| `w_ctx` (bổ sung) | challenge 0,6 · timed_challenge 0,4 | hệ số | platform | [Confirmed — Technical Architect 2026-11-15] | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) §3.1 |
| `diagnostic_prior` | M0 = clamp(0,15 + 0,6·acc; 0,15; 0,75); ≥ 2 câu/LO; tối đa 15/20 câu | — | platform | [Confirmed — Technical Architect 2026-11-15] | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) §3.5 |
| `bkt_irt_readiness` | ≥ 1 học kỳ; ≥ 200 học sinh/LO; ≥ 200 lượt/item; log-loss tốt hơn ≥ 5% | — | code | [Confirmed — Technical Architect 2026-11-15] | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) §3.4 |
| `interleave_mix` | 50/50; 2–4 chủ đề; tối đa 2 câu liên tiếp cùng chủ đề | — | platform | [Confirmed — Technical Architect 2026-11-15] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) §5.2 |
| `difficulty_calibration` | ≥ 100 lượt trả lời lần đầu; bảng p_value → độ khó; người duyệt xác nhận | — | platform | [Confirmed — Technical Architect 2026-11-15] | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) §9.1 |
| `variant_balance_max_mean_p_diff` | 0,05 | — | platform | [Confirmed — Technical Architect 2026-11-15] | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) §4.1 |
| `exam_reliability_warn` | < 0,7 (cần ≥ 30 học sinh) | — | platform | [Confirmed — Technical Architect 2026-11-15] | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) §8.1 |
| `exam_offline_sync_grace` | 30 s sau `deadline_at` để nhận câu trả lời đệm khi mất kết nối (`answered_at_client` < `deadline_at`); client chỉ đệm trong RAM, không ghi đĩa | giây | code | [Confirmed — Technical Architect 2026-11-16] | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) §6 |
| `exam_autosubmit_interval` | 15 (job `exam.autosubmit`) | giây | code | [Confirmed — Technical Architect 2026-11-16] | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) §7.2 |
| `item_analysis_min_responses` | ≥ 30 lượt trả lời lần đầu có tính điểm (loại `rapid_guess`); tính hằng đêm | lượt | platform | [Confirmed — Technical Architect 2026-11-16] | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) §9 |
| `param_sampling_max_tries` | 1.000 (số lần thử tối đa khi sinh tham số câu hỏi; không tìm được bộ giá trị → `422` khi lưu) | lần | code | [Confirmed — Technical Architect 2026-11-16] | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) §5 |
| `simulation_action_log_max` | ≤ 500 hành động, ≤ 64 KB | — | code | [Confirmed — Technical Architect 2026-11-16] | [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) §5 |
| `assessment_reference_thresholds` | Bảng ánh xạ mức tham khảo TT27/TT22 | — | platform | [Confirmed — Technical Architect 2026-11-15] | [ASSESSMENT_REGULATION_PROFILE](../02-learning/ASSESSMENT_REGULATION_PROFILE.md) §4 |

## 14. LMS và Admin Portal

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `lesson_completion_heartbeat` | ≥ 50% thời lượng | — | platform | [Confirmed] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `school_dashboard_refresh` | 15 phút; trang chủ p95 < 1,5 s (School Learning Dashboard, ADR-0072) | — | code | [Confirmed] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `report_link_ttl` | 15 | phút | code | [Confirmed] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `former_teacher_access` | 1 năm dữ liệu lớp cũ | — | tenant | Không dùng (ADR-0072: không có vai trò giáo viên) | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `scheduled_publish_min_lead` | ≥ 5 | phút | platform | [Confirmed] | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `assets_per_version_max` | 50 | asset | platform | [Confirmed] (ADR-0044) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `game_render_max_attempts` | 5 (số lần render thumbnail/turntable trước khi chuyển DLQ; `EW_GAME_RENDER_MAX_ATTEMPTS`) | lần | code | [Confirmed — F078 2026-11-24] | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) §7 |
| `game_render_retry_backoff` | 60 (gốc backoff lũy thừa 2 giữa các lần render lỗi, tối đa 1 giờ; `EW_GAME_RENDER_BACKOFF_SECONDS`) | giây | code | [Confirmed — F078 2026-11-24] | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) §7 |
| `content_snapshot_cache_ttl` | 7 ngày, TTL trượt (snapshot bất biến `ew:<module>:cv:…`) | ngày | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0043) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) §5.4 |
| `media_max_bytes_by_kind` | ảnh ≤ 10 MB · âm thanh ≤ 20 MB, ≤ 10 phút · video ≤ 300 MB, ≤ 20 phút · mô hình 3D/scene ≤ 50 MB | — | platform | [Confirmed — Technical Architect 2026-11-16] (ADR-0044) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) §7 |
| `media_presign_ttl` | 15 (presigned PUT tải lên `ew-media-quarantine`) | phút | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0044) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) §7 |
| `media_signed_url_ttl` | 10 (signed URL CDN cho `ew-media-private`) | phút | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0044) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) §7 |
| `preview_token_ttl` | 30 (chỉ đọc draft được chỉ định; không tạo attempt, XP, learning event) | phút | code | [Confirmed — Technical Architect 2026-11-16] | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) §5 |
| `content_review_sla_days` | 3 (quá hạn → cảnh báo trong hàng đợi review) | ngày | platform | [Confirmed — Technical Architect 2026-11-16] | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) §4 |
| `content_review_bulk_max` | 50 item mỗi lần (mỗi item vẫn kiểm reviewer ≠ author) | item | code | [Confirmed — Technical Architect 2026-11-16] | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) §4 |
| `content_autosave_interval` | 10 (kèm `If-Match` chống ghi đè) | giây | code | [Confirmed — Technical Architect 2026-11-16] | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) §1.4 |
| `supported_locales` | `vi-VN`, `en-US` | — | platform | [Confirmed] (ADR-0056) | [GODOT_ARCHITECTURE §10.1](../08-game-client/GODOT_ARCHITECTURE.md#101-quốc-tế-hóa-i18n) |
| `tenant_default_locale` | `vi-VN` | — | tenant | [Confirmed] (ADR-0056) | [GODOT_ARCHITECTURE §10.1](../08-game-client/GODOT_ARCHITECTURE.md#101-quốc-tế-hóa-i18n) |
| `game_text_required_languages` | `vi`, `en` | — | platform | [Confirmed] (ADR-0056) | [CONTENT_MODEL §8](../02-learning/CONTENT_MODEL.md#8-ngôn-ngữ-nội-dung-và-bản-dịch-adr-0056) |
| `learning_translation_required` | false (chỉ bản `content_language`) | — | tenant | [Confirmed] (ADR-0056) | [CONTENT_MODEL §8](../02-learning/CONTENT_MODEL.md#8-ngôn-ngữ-nội-dung-và-bản-dịch-adr-0056) |
| `text_expansion_allowance` | 1,35 | hệ số | code | [Confirmed] | [DESIGN_SYSTEM §5.5](../05-ux/DESIGN_SYSTEM.md#55-font) |
| `bilingual_names_display` | bật ở khu Tiếng Anh, tắt nơi khác | — | platform (học sinh đổi được) | [Confirmed] | [WORLD_DESIGN §2.4](../01-game-design/WORLD_DESIGN.md#24-tên-song-ngữ-và-quy-tắc-hiển-thị-confirmed--adr-0056) |
| `dashboard_attention_thresholds` | ≥ 2 bài `missed` trong 14 ngày · ≥ 3 LO ở mức `beginning` · không vào game ≥ 7 ngày · tỷ lệ `rapid_guess` ≥ 30% trong 7 ngày | — | tenant | [Confirmed — Technical Architect 2026-11-15] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) §8.1 |
| `graded_assignment_release_default` | `feedback_timing = after_close`, `result_release = after_close` khi `mode = graded` | — | tenant | [Confirmed — Technical Architect 2026-11-15] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) §4.1 |
| `late_window_hours` | 0 (không nhận nộp muộn); tối đa 168 | giờ | tenant | [Confirmed — Technical Architect 2026-11-15] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) §4.1 |
| `assignment_remind_limit` | 1 lần/ngày/bài | lần | platform | [Confirmed — Technical Architect 2026-11-15] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) §4.1 |

## 15. An toàn trẻ em và sức khỏe

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `break_reminder` | 45 phút (g1_3: 30 phút); lần 2 sau 90 phút | — | platform + tenant | [Confirmed, OQ-17] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `teacher_mute_max` | 24 | giờ | tenant | Không dùng (ADR-0072: không có điều khiển chat lớp của giáo viên) | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `risk_signal_thresholds` | bắt nạt ≥ 3/24 h; hội đồng ≥ 3 người/1 h; loại trừ ≥ 5/7 ngày; ≥ 3 report/24 h | — | platform | [Confirmed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `evidence_context` | ±20 | tin | platform | [Confirmed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `chat_policy_cache` | 5 | phút | code | [Confirmed — Technical Architect 2026-11-16] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `platform_curfew` | 21:30–06:30 giờ Việt Nam (ICT) mỗi ngày: không đăng nhập game, không tạo game session, không gửi chat; trường chỉ được mở rộng khung giới nghiêm (siết thêm), không thu hẹp; áp như nhau cho mọi học sinh | — | platform (trường siết thêm) | [Confirmed — Technical Architect 2026-11-17] (ADR-0078) | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) §4.1 |
| `daily_play_limit_minutes` | tắt (không áp mặc định); `school_admin` bật cho cả trường với số phút do trường đặt (ví dụ 120); đạt giới hạn xử lý như giờ giới nghiêm tới 00:00 ICT hôm sau; khoảng hợp lệ 15–300 phút (owner chốt 2026-11-20, F015 T043) | phút/ngày | tenant (cấp trường) | [Confirmed — owner 2026-11-20, OQ-17] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) §4 |
| `curfew_warning_minutes` | 10 (cảnh báo trong game trước giờ giới nghiêm; tới giờ thì lưu tiến độ và kết thúc phiên) | phút | platform | [Confirmed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) §4.1 |

## 16. Identity, phiên, SIS, tài khoản

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `access_token_ttl` | 5 | phút | code | [Confirmed] (ADR-0008) | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `game_session_sso` | Client `eduworld-game-backend`: access token 5 phút; client session idle **7 ngày**, tối đa **30 ngày** (realm SSO Session Idle/Max nâng tương ứng vì Keycloak giới hạn client session ≤ realm); refresh token rotation (`revokeRefreshToken=true`, reuse 0) + phát hiện dùng lại (server giữ SHA-256 của 20 token đã xoay gần nhất), gắn `device_id`; không offline token, không Remember me. Refresh token lưu trong kho bảo mật OS trên thiết bị cá nhân (Windows DPAPI, Android Keystore, iOS Keychain); Web và máy `shared_device` chỉ giữ trong RAM. Còn hạn → mở game tự vào lại, không nhập mật khẩu | — | platform | [Confirmed — Technical Architect 2026-11-16] (ADR-0086) | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `game_mobile_background_relogin` | 30 | phút ở nền trước khi bắt đăng nhập lại | platform | Không dùng (ADR-0086 thay ADR-0064 mục 5: app mobile bị đóng hoặc ở nền lâu mở lại bằng refresh token đã lưu, hết hạn theo `game_session_sso`) | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `game_remember_username` | bật (tắt sẵn khi `device.cfg` `shared_device: true`); điền sẵn form S01 khi phiên hết hạn | — | thiết bị | [Confirmed] (ADR-0064, ADR-0086) | [GODOT_ARCHITECTURE §5.1](../08-game-client/GODOT_ARCHITECTURE.md#51-đăng-nhập-và-xác-định-tenant-adr-0063-adr-0064-adr-0065-adr-0068) |
| `shared_device_default` | Windows: `true` (máy dùng chung, không lưu refresh token) trừ khi bộ cài được chọn "máy cá nhân" (ghi `shared_device: false` vào `device.cfg`); Android, iOS: `false`; Web: không lưu refresh token bền vững | — | thiết bị (bộ cài) | [Confirmed — Technical Architect 2026-11-17] (ADR-0078, ADR-0086) | [GODOT_ARCHITECTURE §5.1](../08-game-client/GODOT_ARCHITECTURE.md#51-đăng-nhập-và-xác-định-tenant-adr-0063-adr-0064-adr-0065-adr-0068) |
| `resume_confirm_screen` | Bật trên mọi nền tảng: khi tự vào lại bằng refresh token, hiện "Tiếp tục là <nickname>?" kèm nút "Không phải tôi" (đăng xuất, xóa token) trước khi tạo game session | — | code | [Confirmed — Technical Architect 2026-11-17] (ADR-0086) | [STUDENT_GAME_UX_CORE](../05-ux/STUDENT_GAME_UX_CORE.md) S01 |
| `shared_device_idle_logout` | 30 | phút không hoạt động (máy `shared_device`: token chỉ trong RAM, thoát game = đăng xuất) | thiết bị | [Confirmed] (ADR-0064, ADR-0086 mục 2) | [KEYCLOAK_SSO §9](../09-identity/KEYCLOAK_SSO.md) |
| `login_capacity_target` | đỉnh 100 đăng nhập mật khẩu/giây; bão 5.000 lượt/10 phút; 500 refresh/giây (gồm refresh `mode=resume` khi mở game trên thiết bị cá nhân, ADR-0086); là mục tiêu năng lực đăng nhập duy nhất cho load test và capacity plan | — | platform | [Confirmed — Technical Architect 2026-11-16] (ADR-0045, ADR-0064 mục 6) | [KEYCLOAK_SSO §12](../09-identity/KEYCLOAK_SSO.md) |
| `portal_session_sso` | idle 30 phút / tối đa 10 giờ (client session `eduworld-portal`, realm SSO idle 7 ngày cho game); phiên `platform_staff` tối đa 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, refresh quá hạn → `401 REFRESH_TOKEN_INVALID`, ADR-0094); refresh token xoay vòng trong cookie `HttpOnly; Secure; SameSite=Strict` do module identity đặt, access token chỉ trong bộ nhớ SPA — tải lại trang gọi `POST /auth/portal/token:refresh`, không đăng nhập lại khi còn hạn (ADR-0086 mục 7, ADR-0091, ADR-0094) | — | platform | [Confirmed] thời hạn · [Confirmed] cách lưu (ADR-0091) | [KEYCLOAK_SSO §5.2](../09-identity/KEYCLOAK_SSO.md) |
| `portal_setup_ticket_ttl` | 10 phút, dùng một lần (`setup_ticket` khi `403 ACCOUNT_SETUP_REQUIRED`: đổi mật khẩu tạm; không còn đăng ký OTP — ADR-0094) | phút | platform | [Confirmed] (ADR-0091) | [KEYCLOAK_SSO §3.3](../09-identity/KEYCLOAK_SSO.md) |
| `portal_password_reset_ttl` | 30 phút, dùng một lần (link đặt lại mật khẩu nhân sự; cấp mới vô hiệu link cũ) | phút | platform | [Confirmed] (ADR-0091) | [KEYCLOAK_SSO §3.3](../09-identity/KEYCLOAK_SSO.md) |
| `quick_login` | **Không làm** (ADR-0063; ADR-0076 Rejected 2026-11-17): không QR lớp, thẻ QR, PIN, mật khẩu hình; đăng nhập chỉ username/password + giữ phiên bằng refresh token (ADR-0086); Device Grant chỉ khi có ADR mới (ADR-0063) | — | — | Không dùng | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `password_hash_budget` | ≤ 80 ms/lần | — | code | [Confirmed] | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `principal_cache` | 60 s (Valkey) + L1 15 s | — | code | [Confirmed] | [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) |
| `inactive_student_cleanup_days` | 180; báo trước 14 ngày; chạy 02:00 ICT; cần duyệt khi > 10% tenant | — | platform + tenant | [Confirmed] (ADR-0032) | [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) |
| `inactive_student_cleanup_low_days_warning` | Cảnh báo khi đặt số ngày dọn < 60 (dễ dọn nhầm qua kỳ nghỉ hè); vẫn cho lưu kèm lý do | ngày | platform | [Confirmed — Technical Architect 2026-11-16] (ADR-0032) | [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) |
| `common_student_batch_max` | 500 dòng mỗi request `createCommonTenantStudents` (tạo học sinh tenant chung) | dòng | code | [Confirmed] (Technical Architect 2026-11-17) | [ACCOUNT_PROVISIONING §3.4](../09-identity/ACCOUNT_PROVISIONING.md#34-tài-khoản-tenant-chung-do-platform-tạo-adr-0063) |
| `min_client_version` | Đặt theo mỗi release; một nguồn cấu hình triển khai `EW_GAME_MIN_CLIENT_VERSION` dùng chung module identity (`gameLogin` → `426 CLIENT_VERSION_UNSUPPORTED`) và module game (`client-config`) | phiên bản | code | [Confirmed — Technical Architect 2026-11-16] | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| `audit_log_retention` | 2 năm; hết hạn ẩn danh hóa, giữ partition (ADR-0070) | năm | platform | [Confirmed — Technical Architect 2026-11-16] | [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) §2 |
| `audit_query_max_range_days` | 93 mỗi lần truy vấn audit | ngày | code | [Confirmed — Technical Architect 2026-11-16] | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) |
| `login_events_retention` | `identity.login_events` (nhật ký xác thực) 90 ngày; `game.login_events` 1 năm rồi ẩn danh hóa `ip_hash`, `device_model` | — | platform | [Confirmed — Technical Architect 2026-11-16] | [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) §2 |
| `student_data_retention` | suốt thời gian học + 1 năm sau khi học sinh `deactivated`, sau đó ẩn danh hóa | — | platform | [Confirmed — Technical Architect 2026-11-16] | [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) §2 |
| `sis_sync_schedule` | full 01:00 ICT (`0 1 * * *`, ghi đè theo connection ở `sis_connections.schedule`); delta 15 phút trong giờ hành chính 07:00–17:59 thứ Hai–thứ Bảy (`*/15 7-17 * * 1-6`), chỉ khi facade khai báo `delta` | — | tenant | [Confirmed] (F017: code dùng mặc định tới khi đọc được tenant settings, OQ F017-7) | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) |
| `sis_staging_retention` | 30 | ngày | code | [Confirmed — Technical Architect 2026-11-16] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) |
| `sis_contract_violation_threshold` | > 10% bản ghi của một run vi phạm contract → dừng run, connection `contract_error`; > 10% lô apply lỗi → dừng run | % | code | [Confirmed — Technical Architect 2026-11-16] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) §2.5, §7 |
| `sis_client_rate` | 10 req/s mỗi connection `contract_v1`; timeout 15 s/request; tôn trọng `Retry-After` | — | tenant | [Confirmed — Technical Architect 2026-11-16] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) §4.2 |
| `sis_retry_backoff` | exponential + jitter 1 s, 2 s, 4 s…, tối đa 6 lần (timeout, 5xx, lỗi mạng) | — | code | [Confirmed — Technical Architect 2026-11-16] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) §7 |
| `sis_circuit_breaker` | 5 run liên tiếp thất bại → tạm dừng lịch, chỉ chạy lại khi admin kích hoạt | run | code | [Confirmed — Technical Architect 2026-11-16] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) §7 |
| `sis_webhook_skew` / `sis_webhook_nonce_ttl` | ±5 phút / 10 phút | — | code | [Confirmed] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) |
| `sis_webhook_retry` | lô webhook xử lý lỗi: giao lại sau 5 phút, tối đa 5 lần → `DLQ_IDENTITY` | — | code | [Confirmed — Technical Architect 2026-11-16] (giá trị §7; tên đăng ký F017) | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) §7 |
| `sis_run_lock_ttl` / `sis_run_stale_after` | lock một run/connection 10 phút (heartbeat 1/3 TTL) / run `queued`/`running` quá 1 giờ không chặn trigger mới | — | code | [Confirmed] (F017) | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) §5 |
| `sis_login_refresh_after_hours` | 24 | giờ | tenant | [Confirmed] (ADR-0061) | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| `sis_login_sync_timeout_ms` | 3000 (timeout duy nhất cho lookup SIS lúc đăng nhập, gồm on-demand lookup lần đầu của ADR-0046) | ms | code | [Confirmed — Technical Architect 2026-11-16] (ADR-0046, ADR-0061) | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| `sis_login_sync_concurrency` | 20 | lookup đồng thời / SIS connection | platform | [Confirmed] (ADR-0061) | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| `sis_login_sync_breaker` | 5 lookup lỗi liên tiếp → mở mạch 30 s mỗi SIS connection | — | code | [Confirmed] (F011) | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| `sis_missing_review_ratio` | 5 (> 5% bản ghi vắng trong hai lần full liên tiếp → `deactivation_review` cấp trường) | % | code | [Confirmed] (F011) | [SIS_INTEGRATION §5](../09-identity/SIS_INTEGRATION.md) |
| `sis_import_upload_ttl` / `sis_import_watch_interval` | 15 phút / 15 giây | — | code | [Confirmed] (F011) | [SIS_INTEGRATION §4.3](../09-identity/SIS_INTEGRATION.md) |
| `enabled_countries` | `VN` | ISO 3166-1 | platform | [Confirmed] (ADR-0060) | [DATABASE_DESIGN §2.2a](../07-backend/DATABASE_DESIGN.md) |

## 17. Rate limit và bảo mật

Bảng rate limit đầy đủ theo endpoint nằm ở [SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md) (nguồn duy nhất). Các giá trị chính:

| Tham số | Mặc định | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|
| `login_rate` | `/auth/game/login`: toàn cục 120 đăng nhập mật khẩu/giây (đỉnh thiết kế 100/s, ADR-0064 mục 6) → `429 LOGIN_QUEUE_BUSY`; 10/phút/thiết bị; 10/phút/username (ADR-0068) → `429 RATE_LIMITED` | platform | [Confirmed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `brute_force_lockout` | 5 lần sai → chờ 1–15 phút (theo tài khoản, không theo IP); thông điệp chung `INVALID_CREDENTIALS` | platform | [Confirmed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `portal_login_rate` | `/auth/portal/login`: 10/phút/username, 10/phút/`device_id`, toàn cục 20/giây; `password:forgot`: 3/giờ/identifier, 10/giờ/`device_id`; `setup:password`, `password:reset`: 10/phút/`device_id`; `token:refresh`: 30/phút/`device_id` → `429 RATE_LIMITED`; **không** giới hạn theo IP (R30) | platform | [Confirmed] (ADR-0091) | [SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md#14-rate-limiting-confirmed) |
| `game_login_ticket_ttl` | 2 phút, dùng một lần (chọn tenant khi thuộc nhiều tenant) | platform | [Confirmed] (ADR-0068) | [KEYCLOAK_SSO §6](../09-identity/KEYCLOAK_SSO.md) |
| `keycloak_token_call_timeout` | 3 giây (module identity → Keycloak token endpoint) | code | [Confirmed] | [BACKEND_DESIGN §6.1](../07-backend/BACKEND_DESIGN.md) |
| `game_login_body_logging` | **tắt** (APISIX, `core-api`, tracing, client) — không cấu hình bật được ở production | code | [Confirmed] (ADR-0068) | [SECURITY_ARCHITECTURE §3.1b](../10-security/SECURITY_ARCHITECTURE.md) |
| `certificate_pinning` | bật cho desktop/mobile, tắt Web (HSTS) | client | [Confirmed] (ADR-0068) | [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) |
| `feature_toggle_cache_ttl` | 60 giây, không vượt `principal_cache_ttl`; cache trạng thái bật/tắt menu/action hiệu lực theo tenant cho `featuregate`, làm mới ngay khi nhận `platform_settings.changed` (scope `feature_flag`) | code | [Confirmed] (ADR-0090) | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) |
| `principal_cache_ttl` | 60 giây (làm mới ngay khi `user_role.changed`/`role.updated`) | code | [Confirmed] (ADR-0066) | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) |
| `tenant_header_required` | bắt buộc `X-Tenant-Id` mọi route tenant-scoped; ngoại lệ: `/auth/game/login`, `/auth/game/tenant:select`, `/auth/session`, `/me/tenants`, `/game/client-config`, health | code | [Confirmed] (ADR-0065) | [API_DESIGN §2](../07-backend/API_DESIGN.md#2-quy-ước) |
| `identifier_uniqueness` | username và email duy nhất trên SSO (một người một tài khoản), không phân biệt hoa thường, do đó không trùng trong cùng tenant; email bắt buộc với nhân sự | platform | [Confirmed] (ADR-0067, ADR-0069) | [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) |
| `api_rate_default` | GET 300/phút; ghi 60/phút; refresh 30/phút theo thiết bị (`/auth/game/token:refresh`, cả `renew` lẫn `resume`); `auth/session` 10/phút | platform | [Confirmed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `answer_rate` | 60/phút, tối thiểu 1 s/câu; submit 10/phút | platform | [Confirmed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `ws_ticket_rate` / `ws_connections_per_user` | 10/phút / 1 (kết nối mới đóng kết nối cũ bằng `4409 replaced`) | platform | [Confirmed] rate · [Confirmed] 1 kết nối (ADR-0031) | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `request_body_max` / `csv_import_max` | 256 KB / 10 MB | code | [Confirmed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `secret_rotation` | 90 ngày | code | [Confirmed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `critical_vuln_fix` | ≤ 72 giờ | quy trình | [Confirmed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `module_call_deadline` | Deadline gọi facade giữa module trong tiến trình (`src/backend/pkg/modcall`): 800 ms đọc / 2 s ghi / 5 s bước saga; không retry, không circuit breaker giữa module (thay `internal_api_resilience`, đã bỏ) | code | [Confirmed] (ADR-0074) | [API_DESIGN §12.3](../07-backend/API_DESIGN.md) |
| `external_api_resilience` | Adapter tích hợp ngoài (Keycloak Admin, SIS, object storage): timeout theo adapter; retry 2 (chỉ thao tác idempotent); circuit breaker 50% / 20 request, nửa mở sau 10 s; bulkhead 50 | code | [Confirmed] (ADR-0074) | [API_DESIGN §12.3](../07-backend/API_DESIGN.md) |
| `idempotency_record_ttl` | 24 giờ | code | [Confirmed] | [API_DESIGN](../07-backend/API_DESIGN.md) |

## 17b. Hiệu năng client (máy tầm trung, ADR-0057)

Giá trị do [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) sở hữu; preset lưu theo thiết bị, không cấu hình theo tenant. Danh sách GPU/SoC ép renderer/preset (`device_profiles`) phát qua `client-config`.

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `device_tier_reference` | Tầm trung: PC i3 thế hệ 10/Ryzen 3 3200G + 8 GB + UHD 630/Vega 8; Android 4 GB Mali-G52/Adreno 610; iPhone 11; Web WebGL 2 | — | code | [Confirmed] yêu cầu · thông số [Confirmed] OQ-04 | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §2 |
| `target_fps` | Mobile/Web 30 (khóa); PC 60 (cho phép 30 ở cảnh đông); Tối thiểu ≥ 30 | FPS | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §3 |
| `renderer_default` | PC/Android/iOS: Mobile; Web + máy Tối thiểu: Compatibility; Forward+ chỉ preset Cao | — | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §4 |
| `quality_presets` | Thấp/Trung bình/Cao; render scale 0,75/0,9/1,0; nhân vật đầy đủ 10/20 (PC 30)/40 | — | thiết bị | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §5 |
| `dynamic_resolution` | Bước 0,05; giảm khi vượt mục tiêu 10% (2 s), tăng khi dưới 20% (10 s) | — | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §6 |
| `scene_budget_hub` | Mobile/Web: 150k tam giác, 100 draw call, 256 MB texture, RAM 1 GB · PC: 300k, 200, 512 MB, 1,5 GB | — | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §7 |
| `character_asset_budget` | LOD0/1/2: 8k/3k/1k tam giác; atlas 512²; ≤ 60 xương; ≤ 3 material | — | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §8 |
| `ui_cpu_budget` | ≤ 3 ms (mobile) · ≤ 2 ms (PC) mỗi khung; ≤ 150 Control trên HUD | ms | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §9 |
| `hub_sync_bandwidth` | ≤ 30 KB/s xuống, ≤ 3 KB/s lên | KB/s | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §10 |
| `load_time` | Khởi động lạnh → Hub ≤ 20 s; chuyển vùng ≤ 5 s; mở activity ≤ 2 s | s | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11 |
| `build_size` | Android/iOS ≤ 300 MB; Windows ≤ 1 GB; Web tải đầu ≤ 40 MB; gói vùng ≤ 40 MB; cache thiết bị ≤ 1,5 GB | MB | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11 |
| `battery_target` | ≤ 15%/giờ trên Android tầm trung; giảm render 3D còn 15 FPS khi màn đứng yên > 10 s | — | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §12 |
| `perf_budget_tolerance_ci` | Vượt ngân sách cảnh > 5% → chặn merge | % | code | [Confirmed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §15 |
| `classroom_reference_bandwidth` | 20 Mbps cho 40 máy dùng chung | Mbps | code | [Confirmed — Technical Architect 2026-11-15] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §10.1 |
| `offhours_prefetch_window` | 18:00–06:00 | giờ địa phương | thiết bị | [Confirmed — Technical Architect 2026-11-15] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §10.1 |
| `lan_cache_url` | Trống (tắt); đặt trong `device.cfg` khi trường có cache LAN | — | thiết bị | [Confirmed — Technical Architect 2026-11-15] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §10.1 |

## 17c. Game Analytics (ADR-0058)

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `analytics_raw_retention_days` | 90 (hết hạn: ẩn danh hóa, partition giữ lại — ADR-0070) | ngày | platform | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §9 |
| `analytics_player_days_retention_days` | 400 | ngày | platform | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §9 |
| `analytics_rollup_retention_years` | 2 (ngày) · 3 (tuần) · 5 (tháng) | năm | platform | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §9 |
| `analytics_player_key_rotation_months` | 24 | tháng | platform | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §2 |
| `analytics_min_cell_size` | 5 | học sinh | platform (module game: env `EW_GAME_ANALYTICS_MIN_CELL_SIZE` 1–100 cho `by_grade` của community goal — F077) | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §2 |
| `analytics_active_session_min_seconds` | 60 | giây | code | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §4 |
| `analytics_late_night_window` | 22:00–06:00 ICT (chỉ dùng cho chỉ số, không phải khung giờ truy cập) | — | platform | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §4.6 |
| `analytics_late_event_window` | 48 | giờ | code | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §5 |
| `analytics_perf_sample_interval` | 60 | giây/phiên | code | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §3.2 |
| `analytics_product_telemetry_enabled` | bật; tenant tắt được (perf/crash vẫn gửi) | — | platform + tenant | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §3.2 |
| `analytics_store_migration_threshold` | raw > 500 triệu dòng · dashboard p95 > 2 s · rollup ngày > 2 h · tăng p95 ghi DB > 20% (kéo dài ≥ 2 tuần) | — | — | [Confirmed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §10 |

## 17d. Dashboard (ADR-0062)

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `dashboard_max_range_days` | 730 | ngày | code | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_grain_daily_max_days` | 31 | ngày | code | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_grain_weekly_max_days` | 366 | ngày (lớn hơn → tháng) | code | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_statement_timeout` | 5 | giây | platform | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_query_p95_target` | 2 | giây | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §15 |
| `dashboard_cache_ttl_today` | 300 | giây | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §9 |
| `dashboard_cache_ttl_closed` | 86400 | giây | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §9 |
| `dashboard_max_concurrent_per_user` | 4 | truy vấn | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_max_concurrent_per_tenant` | 30 | truy vấn | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_db_pool_per_api_replica` | 10 | kết nối | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_page_size_max` | 100 | dòng | code | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_day_close_time` | 02:30 | giờ ICT | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §4 |
| `dashboard_late_window_hours` | 48 | giờ | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §5 |
| `dashboard_ingest_batch` | 1 s hoặc 500 event | — | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §4 |
| `dashboard_rollup_retention_daily_years / weekly / monthly` | 2 / 3 / 5 | năm | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §6 |
| `dashboard_replica_lag_alert_seconds` | 60 | giây | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_export_max_rows` | 1.000.000 | dòng | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |
| `dashboard_export_max_concurrent_per_user` | 2 | job | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |
| `dashboard_export_link_ttl_minutes` | 15 | phút | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |
| `dashboard_export_file_retention_days` | 7 | ngày | platform | [Confirmed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |

## 18. Hạ tầng, hiệu năng, vận hành

| Tham số | Mặc định | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|
| `target_ccu` | 10.000 | — | **[Confirmed]** (stakeholder, ADR-0045) | [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) |
| `login_slo` | đăng nhập mật khẩu → vào hub p95 ≤ 3 s; Keycloak ≤ 800 ms; refresh p95 ≤ 200 ms; `auth/game/login` p95 ≤ 400 ms (gồm Keycloak) + `game/sessions` p95 ≤ 150 ms | — | [Confirmed] | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `warm_up_windows` | 06:45–08:00, 13:15–14:30, 19:00–21:00 ICT; scale trước 15 phút | platform | [Confirmed] (ADR-0024) | [DEPLOYMENT](../11-operations/DEPLOYMENT.md) |
| `alert_thresholds` | NodeCapacityHigh 75%; KeycloakCpuHigh 75%; LoginLatencyHigh p95 > 3 s; EventConsumerLag 10.000; BackupMissing 26 giờ | code | [Confirmed] | [MONITORING](../11-operations/MONITORING.md) |
| `rpo_rto` | RPO ≤ 5 phút; RTO ≤ 4 giờ | — | [Confirmed] (NFR-AVAIL-002) | [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) |
| `object_storage_backend` | local/CI: floci; staging/production: **SeaweedFS tự host (3 node `storage`, replication 3 bản; staging 1 node) theo ADR-0093 — chuyển S3 managed tại Việt Nam khi có ngân sách, chỉ đổi cấu hình `EW_S3_*` + di chuyển dữ liệu** | — | [Confirmed — owner 2026-11-20, OQ-21] (ADR-0075, ADR-0093) | [DEPLOYMENT](../11-operations/DEPLOYMENT.md) §9 |
| `object_storage_backup` | staging/production: replication của backend + bản sao ngoài cụm hằng ngày; bucket riêng tư (`ew-media-private`; `ew-student-submissions` nếu sau này có) có RPO như dữ liệu học tập (`rpo_rto`); giữ 30 bản ngày + 12 bản tháng; diễn tập khôi phục hằng quý | — | [Confirmed] (ADR-0075) | [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) §6, §8 |
| `availability` | 99,5%/tháng trong giờ học 06:30–22:00 ICT (NFR-AVAIL-001) | — | [Confirmed — owner 2026-11-20, OQ-11] | [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) §7 |
| `maintenance_window` | Deploy/bảo trì production 22:00–06:00 ICT hoặc cuối tuần, có approval Release Manager; hotfix ngoại lệ | — | platform | [Confirmed — owner 2026-11-20, OQ-11] | [DEPLOYMENT](../11-operations/DEPLOYMENT.md) §10.3 |
| `event_stream_retention` / `outbox_retention` | 7 ngày / 30 ngày | code | [Confirmed] (ADR-0042) | [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md), [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) |
| `jetstream_duplicate_window` | 2 phút | code | [Confirmed] | [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) |
| `app_log_retention` | 14 ngày | code | [Confirmed] | [MONITORING](../11-operations/MONITORING.md) |

## 19. Mâu thuẫn đã xử lý (2026-11-15, 2026-11-16)

| # | Tham số | Quyết định | Tài liệu sở hữu |
|---|---|---|---|
| C-1 | `learning_block_max_items` | g1_3: 3 · g4_5: 5 · g6_9: 8; GAME_DESIGN L-2 chỉ link | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| C-2 | `abandon_penalty` | ≥ 3 trận/24 h → hạn chế ranked **15 phút** (mức nhẹ hơn cho trẻ em); ANTI_CHEAT chỉ link | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| C-3 | `party_invite_rate` | 10/phút, **50/ngày** mỗi học sinh; SECURITY_ARCHITECTURE §14 và REALTIME_ARCHITECTURE §8.1 chỉ tham chiếu | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| C-4 | `xp_below_grade_multiplier` | ×0,25 khi nội dung thấp hơn ≥ 2 khối; đã chuyển khỏi ANTI_CHEAT | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| C-5 | Giới hạn import SIS | Hai endpoint khác nhau, **có chủ ý**: `/schools/{id}/sis-imports` (School Admin của tenant) 10/giờ; `/sis/imports` (platform, gateway nội bộ) 5/giờ | [API_DESIGN](../07-backend/API_DESIGN.md), [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| C-6 | `hub_instance_capacity` (40), `battle_1v1_reconnect_grace` (30 s) | Cùng giá trị ở nhiều nơi; nơi sở hữu: capacity → [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md), grace theo chế độ → [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md); nơi khác chỉ được nhắc kèm link | xem cột trước |
| C-7 | `login_storm` và `login_capacity_target` | Một tham số `login_capacity_target` (100 đăng nhập/giây, 5.000 lượt/10 phút, 500 refresh/giây); `login_storm` không còn dùng | [KEYCLOAK_SSO §12](../09-identity/KEYCLOAK_SSO.md) |
| C-8 | `sis_lookup_timeout` và `sis_login_sync_timeout_ms` | Một tham số `sis_login_sync_timeout_ms` (3000 ms); `sis_lookup_timeout` không còn dùng | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| C-9 | `ws_connections_per_user` | 1 kết nối mỗi học sinh (ADR-0031); kết nối mới thay kết nối cũ | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md), [REALTIME_ARCHITECTURE §3](../06-architecture/REALTIME_ARCHITECTURE.md) |
| C-10 | Rate limit chat | Chỉ [CHAT_SYSTEM §3.1](../03-social-multiplayer/CHAT_SYSTEM.md) sở hữu (theo channel + 30 tin/phút/người); REALTIME_ARCHITECTURE §8.1 và API_DESIGN chỉ link; cache chat policy 5 phút | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| C-11 | Retention staging SIS và login events | Staging SIS 30 ngày; `identity.login_events` 90 ngày (tối thiểu hóa dữ liệu) | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md), [DATA_PRIVACY §2](../10-security/DATA_PRIVACY.md) |
| C-12 | `community_goal_progress_flush_seconds` | 30 s (không dùng 10 s) | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
