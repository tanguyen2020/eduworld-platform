# CONFIG_DEFAULTS — Sổ đăng ký tham số mặc định

> Trạng thái: Draft · Phiên bản: 1.0 · Owner: Technical PM + Solution Architect · Cập nhật: 2026-11-15
> Liên quan: [docs/README](../README.md) · [PRODUCT_REQUIREMENTS](PRODUCT_REQUIREMENTS.md) · [GLOSSARY](GLOSSARY.md) · [OPEN_QUESTIONS](../12-project/OPEN_QUESTIONS.md)

## 1. Mục đích và quy tắc

Một chỉ mục duy nhất cho **mọi tham số mặc định cấu hình được** của EduWorld (thời gian, giới hạn, ngưỡng, tỷ lệ), để tránh mỗi tài liệu tự ghi một con số.

| Quy tắc | Nội dung |
|---|---|
| R1 — Một nơi sở hữu | Mỗi tham số có **đúng một tài liệu sở hữu** (cột "Sở hữu"). Tài liệu sở hữu là nguồn sự thật; tài liệu khác chỉ link, không chép lại con số |
| R2 — Đổi cùng lúc | Đổi giá trị phải sửa **tài liệu sở hữu và dòng ở đây trong cùng một merge request**; nếu là [Confirmed] cần quyết định stakeholder/ADR |
| R3 — Nhãn | Mặc định mọi giá trị là **[Proposed]**; chỉ ghi [Confirmed] khi đã có quyết định stakeholder hoặc Master Prompt |
| R4 — Phạm vi | `platform` = chỉ đội platform cấu hình (Game Operations Console/Platform Administration); `tenant` = tenant được ghi đè trong giới hạn platform cho phép; `code` = hằng số trong mã/hạ tầng, đổi bằng release |
| R5 — Tên | Tên `snake_case`, dùng làm khóa cấu hình trong hệ thống (`platform_settings`, `tenant_settings`, cấu hình game có version) |

Quy ước sở hữu đã chốt cho các vùng chồng lấn: trần và giảm dần XP → [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md); trần Xu → [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md); [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) chỉ sở hữu ngưỡng phát hiện và xử lý gian lận; nhắc nghỉ → [CHILD_SAFETY](../10-security/CHILD_SAFETY.md); lời mời party → [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md); SLA báo cáo → [MODERATION](../03-social-multiplayer/MODERATION.md); giới hạn độ dài chat → [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md); rate limit API → [SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md).

## 2. Thế giới và phiên chơi

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `session_target_minutes` | 15–30 | phút | code | [Proposed] | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) |
| `learning_block_max_items` | g1_3: 3 · g4_5: 5 · g6_9: 8 | câu liên tiếp trước một nhịp game | platform | [Proposed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `hub_instance_capacity` | 40 | người | platform | [Proposed] (ADR-0012) | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `hub_instance_idle_close` | 5 | phút | code | [Proposed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `private_instance_capacity` | ≤ 8 | người | platform | [Proposed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `battle_room_size` | 2–8 | người | platform | [Proposed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `area_transition_max` | < 5 | giây | code (NFR) | [Proposed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `discovery_report_rate` | ≤ 30 | lần/phút | code | [Proposed] | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| `party_seat_hold` | 30 | giây | code | [Proposed] | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |

## 3. Progression và XP

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `xp_curve` | `power_linear`: base 100, exponent 1,35, linear 20 | — | platform (version hóa) | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `max_level` | 60 | level | platform | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `level_titles_at` | 1 / 10 / 20 / 35 / 50 | level | platform | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `level_floor_protection` | true | — | platform | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_repeat_multipliers` | 100% / 50% / 25% / 0% trong 7 ngày (theo content version) | — | platform | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_daily_cap` | 1.000 | XP/ngày | platform | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_competition_matches_per_day` | 10 | trận/ngày | platform | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| `xp_below_grade_multiplier` | ×0,25 khi nội dung thấp hơn ≥ 2 khối | — | platform | [Proposed] | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |

## 4. Quest

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `daily_quests_per_day` | 3 | quest | platform | [Proposed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `weekly_quests_per_week` | 3 | quest | platform | [Proposed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `quest_reset_time` | 04:00 ICT; weekly: thứ Hai 04:00 | — | platform | [Proposed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `max_active_quests` | 10 | quest | platform | [Proposed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `quest_pass_rule` | tỷ lệ đúng 0,7; tối thiểu 8 câu; 5 lần/ngày | — | platform (theo quest) | [Proposed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| `coop_quest_reconnect_grace` | 120 | giây | platform | [Proposed] | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |

## 5. Challenge và boss

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `difficulty_levels` | Easy, Normal, Hard, Expert, Legendary (bảng tham số theo mức) | — | platform | [Confirmed] tên mức · [Proposed] tham số | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `g1_3_challenge_rules` | thời gian ×1,5; chỉ Easy–Hard | — | platform | [Proposed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `challenge_attempts_per_day` | 5 | lượt | platform | [Proposed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `challenge_fail_cooldown` | 60 (từ mức Hard) | giây | platform | [Proposed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| `boss_reconnect_grace` | 120 | giây | platform | [Proposed] | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |

## 6. Kinh tế, vật phẩm, sưu tầm

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `xu_daily_cap` | 300 | Xu/ngày | platform | [Proposed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `duplicate_item_xu` | 5 / 10 / 25 / 50 / 100 theo độ hiếm | Xu | platform | [Proposed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `shop_purchase_limit_default` | 1 | lần/item | platform (theo item) | [Proposed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `badges_displayed_max` | 3 | huy hiệu | platform | [Proposed] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `trading_enabled` | false | — | platform | [Confirmed] (tắt đến khi được review) | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| `real_money_purchase` | không có | — | — | [Proposed, OQ-14] | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |

## 7. Nhân vật

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `walk_speed` / `run_speed` | 3,0 / 5,5 | m/s | code | [Proposed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `jump_height` | 1,0 | m | code | [Proposed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `camera_distance` | 4–8 | m | code | [Proposed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `emote_rate` | 1 lần/2 s | — | code | [Proposed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `emote_wheel_slots` | 6 | ô | code | [Proposed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| `avatar_creation_presets` / `saved_outfits_max` | 6 / 3 | bộ | platform | [Proposed] | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |

## 8. Thi đấu, ranking, leaderboard, tournament

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `battle_1v1_format` | 7 câu × 20 s (g1_3: 30 s); 100 điểm + bonus tốc độ ≤ 50; câu cuối ×2 | — | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `team_battle_format` | 10 câu; lượt hỗ trợ 30% điểm | — | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `grade_window_g6_9` | ±1 khối sau 30 s chờ (g1_5 luôn cùng khối) | — | platform | [Confirmed] nguyên tắc · [Proposed] số | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `glicko2_params` | r0 1500, RD 350, σ 0,06, τ 0,5; RD 50–350 | — | code | [Proposed] (ADR-0018) | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `rating_provisional` | < 10 trận hoặc RD > 110 | — | code | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `season_length` | 8 | tuần | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `season_soft_reset` | hệ số 0,5; RD sau reset ≥ 150 | — | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `tiers` | 6 tier; hysteresis 50; ẩn số rating thô với mọi học sinh | — | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `competition_points_daily_cap` | 300; quá 20 trận/ngày không cộng | điểm | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `cheat_fast_answer_flag` | < 400 ms (g1_5: < 600 ms) liên tiếp ≥ 3 câu | — | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `leaderboard_visibility` | g1_3: không xếp hạng cá nhân; g4_5: top 10; g6_9: top 50 + ±3 quanh mình | — | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| `tournament_timers` | check-in 15 phút; no-show 3 phút; disconnect grace 60 s × 2; khiếu nại 10 phút | — | platform | [Proposed] | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |

### 8.1. Sự kiện và season track

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `event_claim_grace_hours` | 72 | giờ | platform (theo sự kiện) | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_festival_min_duration_days` | 7 | ngày | platform | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_extra_daily_quests_max` | 2 | quest/ngày | platform | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_points_daily_cap` | 200 | điểm sự kiện/ngày | platform (theo sự kiện) | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_upcoming_visible_days` | 7 | ngày | platform | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_rule_change_effective` | 04:00 ICT hôm sau | — | code | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `community_goal_contribution_daily_cap` | 50 | đơn vị/học sinh/ngày | platform (theo goal) | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `community_goal_progress_flush_seconds` | 30 | giây | platform | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_track_tiers` / `season_track_points_per_tier` | 30 cấp / 100 điểm | — | platform (theo track) | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_points_daily_cap` | 100 | điểm mùa/ngày | platform (theo track) | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_point_sources` | daily quest 20 · weekly quest 50 · hoạt động học tập 5 · hoạt động sự kiện 10 · co-op 10 · trận 5 | điểm | platform (theo track) | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `season_unclaimed_rewards` | tự cấp khi hết mùa | — | code | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `school_event_max_duration_days` | 28 | ngày | platform | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `school_events_concurrent_per_tenant` | 3 | sự kiện | platform | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| `event_decoration_bundle_max_mb` | 20 | MB (mobile) | platform | [Proposed] | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |

## 9. Party, bạn bè, club

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `party_max_members` | 4 (cấu hình 2–8) | người | platform + tenant | [Confirmed] cấu hình được · [Proposed] số | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_invite_ttl` | 5 | phút | platform | [Proposed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_invite_rate` | 10/phút, 50/giờ | lời mời | platform | [Proposed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_leader_grace` | 60 | giây | platform | [Proposed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_dormant_timeout` | 30 | phút | platform | [Proposed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `party_kick_reinvite_block` | 10 | phút | platform | [Proposed] | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `friend_limits` | ≤ 10 lời mời đang chờ; ≤ 20 lời mời/ngày; ≤ 100 bạn | — | platform | [Proposed] | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |
| `friend_request_ttl` / `friend_code_ttl` | 7 ngày / 10 phút | — | platform | [Proposed] | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |
| `club_max_members` / `clubs_per_student_max` | 50 / 5 | — | platform + tenant | [Proposed] (ADR-0050) | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |

## 10. Matchmaking

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `mm_rating_window` | 100 / 200 / 350 / ∞ tại 0 / 10 / 20 / 40 s | — | platform | [Proposed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_ready_check` | 15 | giây | platform | [Proposed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_bot_after` | 90 (bot luyện tập unranked) | giây | platform | [Proposed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_ticket_ttl` / `mm_tickets_per_user` | 5 phút / 1 | — | platform | [Proposed] | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| `mm_ticket_rate` | 30/giờ; cooldown 2 / 10 phút | — | platform | [Proposed] | [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) |

## 11. Multiplayer và realtime

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `disconnect_grace` | hub 30 s; quest co-op 120 s; boss 120 s; lobby 10 s; 1v1 30 s; đội 45 s; giải 60 s (tối đa 2 lần) | — | platform | [Proposed] | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| `abandon_penalty` | ≥ 3 trận/24 h → hạn chế ranked 15 phút | — | platform | [Proposed] | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| `realtime_ticket_ttl` | 60 s, dùng 1 lần | — | code | [Proposed] (ADR-0008) | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `ws_heartbeat` / `ws_timeout` / `ws_ping` | 15 / 45 / 20 | giây | code | [Proposed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `ws_max_frame` | 16 | KB | code | [Proposed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `resume_window` | 120 s, ≤ 500 message | — | code | [Proposed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `nats_request_timeout` | 2 | giây | code | [Proposed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `hub_sync_rate` | 10 | Hz | code | [Proposed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `ws_per_gateway_node` | 4.000 | kết nối | code | [Proposed] | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) |
| `client_reconnect_backoff` | 0,5–15 s, jitter ±20% | — | code | [Proposed] | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| `client_interpolation` | 100 ms (tối đa 200) | — | code | [Proposed] | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| `client_input_rate` / `client_command_timeout` | ≤ 10 Hz / 10 s | — | code | [Proposed] | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| `presence_ttl` / `active_session_ttl` | 90 | giây | code | [Proposed] | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) |

## 12. Chat và kiểm duyệt

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `chat_max_length` | 120 / 200 / 300 theo loại channel (trần DB 1.000) | ký tự | platform | [Proposed] (ADR-0041) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_edit_window` / `chat_recall_window` | 15 phút / 24 giờ | — | platform | [Proposed] (ADR-0034) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_pins_max` / `chat_mentions_max` / `chat_reactions_per_user` | 3 / 5 / 3 | — | platform | [Proposed] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_forward_targets_max` | 5 | channel | platform | [Proposed] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `gameplay_chat_retention_days` | 7 (tối thiểu 3) | ngày | platform + tenant | [Proposed] (ADR-0033) | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `ephemeral_chat_retention` | 24 (World/Area) | giờ | platform | [Proposed] | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat_recent_buffer` | 200 tin / 10 phút | — | code | [Proposed] (ADR-0035) | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) |
| `chat_rate_per_user` | ≤ 30 | tin/phút | platform | [Proposed] | [SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md) |
| `moderation_switches` | bật tất cả bước | — | platform + tenant (chỉ System Admin) | [Confirmed] cơ chế (ADR-0030) | [MODERATION](../03-social-multiplayer/MODERATION.md) |
| `moderation_pipeline_latency` | p95 < 20 ms (trần 50 ms) | — | code | [Proposed] | [MODERATION](../03-social-multiplayer/MODERATION.md) |
| `report_sla` | p1: phản hồi 1 giờ / xong 24 giờ; p2: 1 / 3 ngày học; p3: 3 / 7 ngày học | — | platform | [Proposed] | [MODERATION](../03-social-multiplayer/MODERATION.md) |
| `auto_mute_spam` | 10 phút → 1 giờ khi tái phạm | — | platform | [Proposed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `filter_quality_gate` | recall ≥ 95%; false positive ≤ 3% | — | release gate | [Proposed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |

## 13. Học tập

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `attempt_idle_timeout` | 5 | phút | platform | [Proposed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `attempt_resume_window` | 24 giờ (practice/retry/quest/challenge) | — | platform | [Proposed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `deadline_grace` | 2 | giây | code | [Proposed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `suspicious_speed_flag` | < 1,5 s/câu trên ≥ 10 câu | — | platform | [Proposed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `score_scale` | thang 10, làm tròn 1 chữ số | — | tenant | [Proposed] | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| `attempts_per_question` | g1_3: 3; khối khác: 2 | lượt | platform | [Proposed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `retry_wrong_schedule` | 1–3–7 ngày | — | platform | [Proposed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `question_font_px` | 32 / 28 / 24 / 22 (g1_3 / g4_5 / g6_7 / g8_9) | px | code | [Proposed] | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| `mastery_model` | EWMA có trọng số theo ngữ cảnh, prior 0,30, ngưỡng mức | — | platform (version hóa) | [Proposed] (ADR-0017) | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) |
| `latex_max_length` / `latex_max_depth` / `math_svg_max` | 2.000 ký tự / 20 / 256 KB | — | code | [Proposed] (ADR-0055) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `publish_to_visible_max` | ≤ 30 | giây | code (SLO) | [Proposed] (ADR-0043) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `video_completion` | xem thật ≥ 90% + trả lời câu chèn bắt buộc; heartbeat 10 s | — | platform | [Proposed] | [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) |
| `video_recommended_length` | ≤ 8 phút (g1_5); ≤ 15 phút (g6_9) | — | hướng dẫn biên tập | [Proposed] | [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) |
| `recording_limits` | 30 s (g1_5) / 90 s (g6_9); 3 lần ghi lại; xóa 90 ngày sau khi chấm | — | platform | [Proposed] | [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md) |
| `written_autosave` / `grading_deadline` | 15 giây / 7 ngày | — | tenant | [Proposed] | [WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md) |
| `attempt_start_rate` | 20/giờ | lượt | platform | [Proposed] | [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) |

## 14. LMS và Admin Portal

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `lesson_completion_heartbeat` | ≥ 50% thời lượng | — | platform | [Proposed] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `teacher_dashboard_refresh` | 15 phút; trang chủ p95 < 1,5 s | — | code | [Proposed] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `report_link_ttl` | 15 | phút | code | [Proposed] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `former_teacher_access` | 1 năm dữ liệu lớp cũ | — | tenant | [Proposed] | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `scheduled_publish_min_lead` | ≥ 5 | phút | platform | [Proposed] | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `assets_per_version_max` | 50 | asset | platform | [Proposed] (ADR-0044) | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| `supported_locales` | `vi-VN`, `en-US` | — | platform | [Confirmed] (ADR-0056) | [GODOT_ARCHITECTURE §10.1](../08-game-client/GODOT_ARCHITECTURE.md#101-quốc-tế-hóa-i18n) |
| `tenant_default_locale` | `vi-VN` | — | tenant | [Confirmed] (ADR-0056) | [GODOT_ARCHITECTURE §10.1](../08-game-client/GODOT_ARCHITECTURE.md#101-quốc-tế-hóa-i18n) |
| `game_text_required_languages` | `vi`, `en` | — | platform | [Confirmed] (ADR-0056) | [CONTENT_MODEL §8](../02-learning/CONTENT_MODEL.md#8-ngôn-ngữ-nội-dung-và-bản-dịch-adr-0056) |
| `learning_translation_required` | false (chỉ bản `content_language`) | — | tenant | [Proposed] (ADR-0056) | [CONTENT_MODEL §8](../02-learning/CONTENT_MODEL.md#8-ngôn-ngữ-nội-dung-và-bản-dịch-adr-0056) |
| `text_expansion_allowance` | 1,35 | hệ số | code | [Proposed] | [DESIGN_SYSTEM §5.5](../05-ux/DESIGN_SYSTEM.md#55-font) |
| `bilingual_names_display` | bật ở khu Tiếng Anh, tắt nơi khác | — | platform (học sinh đổi được) | [Proposed] | [WORLD_DESIGN §2.4](../01-game-design/WORLD_DESIGN.md#24-tên-song-ngữ-và-quy-tắc-hiển-thị-confirmed--adr-0056) |

## 15. An toàn trẻ em và sức khỏe

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `break_reminder` | 45 phút (g1_3: 30 phút); lần 2 sau 90 phút | — | platform + tenant | [Proposed, OQ-17] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `teacher_mute_max` | 24 | giờ | tenant | [Proposed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `risk_signal_thresholds` | bắt nạt ≥ 3/24 h; hội đồng ≥ 3 người/1 h; loại trừ ≥ 5/7 ngày; ≥ 3 report/24 h | — | platform | [Proposed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `evidence_context` | ±20 | tin | platform | [Proposed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| `chat_policy_cache` | 5 | phút | code | [Proposed] | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |

## 16. Identity, phiên, SIS, tài khoản

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `access_token_ttl` | 5 | phút | code | [Proposed] (ADR-0008) | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `game_session_sso` | idle 30 phút / tối đa 10 giờ; không Remember me; không offline token; token chỉ trong RAM; mỗi lần mở game nhập mật khẩu vào form trong game (ADR-0068) | — | platform | [Proposed] tham số / [Confirmed] chính sách (ADR-0064) | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `game_mobile_background_relogin` | 30 | phút ở nền trước khi bắt đăng nhập lại | platform | [Proposed] (ADR-0064) | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `game_remember_username` | bật (tắt sẵn khi `device.cfg` `shared_device: true`) | — | thiết bị | [Proposed] (ADR-0064) | [GODOT_ARCHITECTURE §5.1](../08-game-client/GODOT_ARCHITECTURE.md#51-đăng-nhập-và-xác-định-tenant-adr-0063-adr-0064-adr-0065-adr-0068) |
| `shared_device_idle_logout` | 30 | phút không hoạt động | thiết bị | [Proposed] (ADR-0064) | [KEYCLOAK_SSO §9](../09-identity/KEYCLOAK_SSO.md) |
| `login_capacity_target` | đỉnh 100 đăng nhập mật khẩu/giây; bão 5.000 lượt/10 phút; 500 refresh/giây | — | platform | [Proposed] (ADR-0064) | [KEYCLOAK_SSO §12](../09-identity/KEYCLOAK_SSO.md) |
| `portal_session_sso` | idle 30 phút / tối đa 10 giờ; platform `auth_time` ≤ 8 giờ | — | platform | [Proposed] | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `quick_login` | **Ngoài Release 1.0** (ADR-0063) — khi bật sau: QR TTL 60 s, mở tối đa 15 phút + PIN 4 số; PIN 4–6 số; mật khẩu hình ảnh 3–6 hình; device code 10 phút, polling 5 s | — | tenant | Mở rộng sau | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `password_hash_budget` | ≤ 80 ms/lần | — | code | [Proposed] | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `principal_cache` | 60 s (Valkey) + L1 15 s | — | code | [Proposed] | [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) |
| `inactive_student_cleanup_days` | 180; báo trước 14 ngày; chạy 02:00 ICT; cần duyệt khi > 10% tenant | — | platform + tenant | [Proposed] (ADR-0032) | [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) |
| `sis_sync_schedule` | full 01:00 ICT; delta 15 phút | — | tenant | [Proposed] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) |
| `sis_lookup_timeout` | 3 | giây | code | [Proposed] (ADR-0046) | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) |
| `sis_staging_retention` | 30 | ngày | code | [Proposed] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) |
| `sis_webhook_skew` / `sis_webhook_nonce_ttl` | ±5 phút / 10 phút | — | code | [Proposed] | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) |
| `sis_login_refresh_after_hours` | 24 | giờ | tenant | [Proposed] (ADR-0061) | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| `sis_login_sync_timeout_ms` | 3000 | ms | code | [Proposed] (ADR-0061) | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| `sis_login_sync_concurrency` | 20 | lookup đồng thời / SIS connection | platform | [Proposed] (ADR-0061) | [SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md) |
| `enabled_countries` | `VN` | ISO 3166-1 | platform | [Confirmed] (ADR-0060) | [DATABASE_DESIGN §2.2a](../07-backend/DATABASE_DESIGN.md) |

## 17. Rate limit và bảo mật

Bảng rate limit đầy đủ theo endpoint nằm ở [SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md) (nguồn duy nhất). Các giá trị chính:

| Tham số | Mặc định | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|
| `login_rate` | `/auth/game/login`: toàn cục 120 đăng nhập mật khẩu/giây (đỉnh thiết kế 100/s, ADR-0064) → `429 LOGIN_QUEUE_BUSY`; 10/phút/thiết bị; 10/phút/username (ADR-0068) → `429 RATE_LIMITED` | platform | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `brute_force_lockout` | 5 lần sai → chờ 1–15 phút (theo tài khoản, không theo IP); thông điệp chung `INVALID_CREDENTIALS` | platform | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `game_login_ticket_ttl` | 2 phút, dùng một lần (chọn tenant khi thuộc nhiều tenant) | platform | [Confirmed] (ADR-0068) | [KEYCLOAK_SSO §6](../09-identity/KEYCLOAK_SSO.md) |
| `keycloak_token_call_timeout` | 3 giây (identity-service → Keycloak token endpoint) | code | [Proposed] | [BACKEND_DESIGN §6.1](../07-backend/BACKEND_DESIGN.md) |
| `game_login_body_logging` | **tắt** (APISIX, service, tracing, client) — không cấu hình bật được ở production | code | [Confirmed] (ADR-0068) | [SECURITY_ARCHITECTURE §3.1b](../10-security/SECURITY_ARCHITECTURE.md) |
| `certificate_pinning` | bật cho desktop/mobile, tắt Web (HSTS) | client | [Proposed] (ADR-0068) | [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) |
| `principal_cache_ttl` | 60 giây (làm mới ngay khi `user_role.changed`/`role.updated`) | code | [Proposed] (ADR-0066) | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) |
| `tenant_header_required` | bắt buộc `X-Tenant-Id` mọi route tenant-scoped; ngoại lệ: `/auth/game/login`, `/auth/game/tenant:select`, `/auth/session`, `/me/tenants`, `/game/client-config`, health | code | [Confirmed] (ADR-0065) | [API_DESIGN §2](../07-backend/API_DESIGN.md#2-quy-ước) |
| `identifier_uniqueness` | username và email duy nhất trên SSO (một người một tài khoản), không phân biệt hoa thường, do đó không trùng trong cùng tenant; email bắt buộc với nhân sự | platform | [Confirmed] (ADR-0067, ADR-0069) | [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) |
| `api_rate_default` | GET 300/phút; ghi 60/phút; refresh 30/phút (`/auth/game/token:refresh`); `auth/session` 10/phút | platform | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `answer_rate` | 60/phút, tối thiểu 1 s/câu; submit 10/phút | platform | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `ws_ticket_rate` / `ws_connections_per_user` | 10/phút / 2 | platform | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `request_body_max` / `csv_import_max` | 256 KB / 10 MB | code | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `secret_rotation` | 90 ngày | code | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `critical_vuln_fix` | ≤ 72 giờ | quy trình | [Proposed] | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| `internal_api_resilience` | timeout 800 ms đọc / 2 s ghi / 5 s saga; retry 2 (chỉ idempotent); circuit breaker 50% / 20 request, nửa mở sau 10 s; bulkhead 50 | code | [Proposed] (ADR-0040) | [API_DESIGN](../07-backend/API_DESIGN.md) |
| `idempotency_record_ttl` | 24 giờ | code | [Proposed] | [API_DESIGN](../07-backend/API_DESIGN.md) |

## 17b. Hiệu năng client (máy tầm trung, ADR-0057)

Giá trị do [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) sở hữu; preset lưu theo thiết bị, không cấu hình theo tenant. Danh sách GPU/SoC ép renderer/preset (`device_profiles`) phát qua `client-config`.

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `device_tier_reference` | Tầm trung: PC i3 thế hệ 10/Ryzen 3 3200G + 8 GB + UHD 630/Vega 8; Android 4 GB Mali-G52/Adreno 610; iPhone 11; Web WebGL 2 | — | code | [Confirmed] yêu cầu · thông số [Proposed] OQ-04 | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §2 |
| `target_fps` | Mobile/Web 30 (khóa); PC 60 (cho phép 30 ở cảnh đông); Tối thiểu ≥ 30 | FPS | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §3 |
| `renderer_default` | PC/Android/iOS: Mobile; Web + máy Tối thiểu: Compatibility; Forward+ chỉ preset Cao | — | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §4 |
| `quality_presets` | Thấp/Trung bình/Cao; render scale 0,75/0,9/1,0; nhân vật đầy đủ 10/20 (PC 30)/40 | — | thiết bị | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §5 |
| `dynamic_resolution` | Bước 0,05; giảm khi vượt mục tiêu 10% (2 s), tăng khi dưới 20% (10 s) | — | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §6 |
| `scene_budget_hub` | Mobile/Web: 150k tam giác, 100 draw call, 256 MB texture, RAM 1 GB · PC: 300k, 200, 512 MB, 1,5 GB | — | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §7 |
| `character_asset_budget` | LOD0/1/2: 8k/3k/1k tam giác; atlas 512²; ≤ 60 xương; ≤ 3 material | — | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §8 |
| `ui_cpu_budget` | ≤ 3 ms (mobile) · ≤ 2 ms (PC) mỗi khung; ≤ 150 Control trên HUD | ms | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §9 |
| `hub_sync_bandwidth` | ≤ 30 KB/s xuống, ≤ 3 KB/s lên | KB/s | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §10 |
| `load_time` | Khởi động lạnh → Hub ≤ 20 s; chuyển vùng ≤ 5 s; mở activity ≤ 2 s | s | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11 |
| `build_size` | Android/iOS ≤ 300 MB; Windows ≤ 1 GB; Web tải đầu ≤ 40 MB; gói vùng ≤ 40 MB; cache thiết bị ≤ 1,5 GB | MB | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11 |
| `battery_target` | ≤ 15%/giờ trên Android tầm trung; giảm render 3D còn 15 FPS khi màn đứng yên > 10 s | — | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §12 |
| `perf_budget_tolerance_ci` | Vượt ngân sách cảnh > 5% → chặn merge | % | code | [Proposed] | [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §15 |

## 17c. Game Analytics (ADR-0058)

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `analytics_raw_retention_days` | 90 (hết hạn: ẩn danh hóa, partition giữ lại — ADR-0070) | ngày | platform | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §9 |
| `analytics_player_days_retention_days` | 400 | ngày | platform | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §9 |
| `analytics_rollup_retention_years` | 2 (ngày) · 3 (tuần) · 5 (tháng) | năm | platform | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §9 |
| `analytics_player_key_rotation_months` | 24 | tháng | platform | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §2 |
| `analytics_min_cell_size` | 5 | học sinh | platform | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §2 |
| `analytics_active_session_min_seconds` | 60 | giây | code | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §4 |
| `analytics_late_night_window` | 22:00–06:00 ICT (chỉ dùng cho chỉ số, không phải khung giờ truy cập) | — | platform | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §4.6 |
| `analytics_late_event_window` | 48 | giờ | code | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §5 |
| `analytics_perf_sample_interval` | 60 | giây/phiên | code | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §3.2 |
| `analytics_product_telemetry_enabled` | bật; tenant tắt được (perf/crash vẫn gửi) | — | platform + tenant | [Proposed, OQ-12] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §3.2 |
| `analytics_store_migration_threshold` | raw > 500 triệu dòng · dashboard p95 > 5 s · rollup ngày > 2 h · tăng p95 ghi DB > 20% (kéo dài ≥ 2 tuần) | — | — | [Proposed] | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) §10 |

## 17d. Dashboard (ADR-0062)

| Tham số | Mặc định | Đơn vị | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|---|
| `dashboard_max_range_days` | 730 | ngày | code | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_grain_daily_max_days` | 31 | ngày | code | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_grain_weekly_max_days` | 366 | ngày (lớn hơn → tháng) | code | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_statement_timeout` | 5 | giây | platform | [Confirmed — ADR-0062] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_query_p95_target` | 2 | giây | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §15 |
| `dashboard_cache_ttl_today` | 300 | giây | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §9 |
| `dashboard_cache_ttl_closed` | 86400 | giây | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §9 |
| `dashboard_max_concurrent_per_user` | 4 | truy vấn | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_max_concurrent_per_tenant` | 30 | truy vấn | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_db_pool_per_api_replica` | 10 | kết nối | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_page_size_max` | 100 | dòng | code | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §8 |
| `dashboard_day_close_time` | 02:30 | giờ ICT | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §4 |
| `dashboard_late_window_hours` | 48 | giờ | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §5 |
| `dashboard_ingest_batch` | 1 s hoặc 500 event | — | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §4 |
| `dashboard_rollup_retention_daily_years / weekly / monthly` | 2 / 3 / 5 | năm | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §6 |
| `dashboard_replica_lag_alert_seconds` | 60 | giây | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §7 |
| `dashboard_export_max_rows` | 1.000.000 | dòng | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |
| `dashboard_export_max_concurrent_per_user` | 2 | job | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |
| `dashboard_export_link_ttl_minutes` | 15 | phút | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |
| `dashboard_export_file_retention_days` | 7 | ngày | platform | [Proposed] | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) §10 |

## 18. Hạ tầng, hiệu năng, vận hành

| Tham số | Mặc định | Phạm vi | Nhãn | Sở hữu |
|---|---|---|---|---|
| `target_ccu` | 10.000 | — | **[Confirmed]** (stakeholder, ADR-0045) | [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) |
| `login_storm` | 3.000 lượt/10 phút; đỉnh 50 đăng nhập mật khẩu/s; 500 refresh/s | — | [Proposed] (ADR-0045) | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `login_slo` | đăng nhập mật khẩu → vào hub p95 ≤ 3 s; Keycloak ≤ 800 ms; refresh p95 ≤ 200 ms; `auth/game/login` p95 ≤ 400 ms (gồm Keycloak) + `game/sessions` p95 ≤ 150 ms | — | [Proposed] | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| `warm_up_windows` | 06:45–08:00, 13:15–14:30, 19:00–21:00 ICT; scale trước 15 phút | platform | [Proposed] (ADR-0024) | [DEPLOYMENT](../11-operations/DEPLOYMENT.md) |
| `alert_thresholds` | NodeCapacityHigh 75%; KeycloakCpuHigh 75%; LoginLatencyHigh p95 > 3 s; EventConsumerLag 10.000; BackupMissing 26 giờ | code | [Proposed] | [MONITORING](../11-operations/MONITORING.md) |
| `rpo_rto` | RPO ≤ 5 phút; RTO ≤ 4 giờ | — | [Proposed] (NFR-AVAIL-002) | [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) |
| `object_storage_backup` | floci: backup ngoài máy hằng ngày; giữ 30 bản ngày + 12 bản tháng; diễn tập khôi phục hằng quý | — | [Proposed] (ADR-0071) | [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) §6, §8 |
| `availability` | 99,5% trong giờ học | — | [TBD, OQ-11] | [PRODUCT_REQUIREMENTS](PRODUCT_REQUIREMENTS.md) |
| `event_stream_retention` / `outbox_retention` | 7 ngày / 30 ngày | code | [Proposed] (ADR-0042) | [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md), [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) |
| `jetstream_duplicate_window` | 2 phút | code | [Proposed] | [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) |
| `app_log_retention` | 14 ngày | code | [Proposed] | [MONITORING](../11-operations/MONITORING.md) |

## 19. Mâu thuẫn đã xử lý (2026-11-15)

| # | Tham số | Quyết định | Tài liệu sở hữu |
|---|---|---|---|
| C-1 | `learning_block_max_items` | g1_3: 3 · g4_5: 5 · g6_9: 8; GAME_DESIGN L-2 chỉ link | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| C-2 | `abandon_penalty` | ≥ 3 trận/24 h → hạn chế ranked **15 phút** (mức nhẹ hơn cho trẻ em); ANTI_CHEAT chỉ link | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| C-3 | `party_invite_rate` | 10/phút, **50/giờ**; SECURITY_ARCHITECTURE §14 chỉ tham chiếu | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| C-4 | `xp_below_grade_multiplier` | ×0,25 khi nội dung thấp hơn ≥ 2 khối; đã chuyển khỏi ANTI_CHEAT | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| C-5 | Giới hạn import SIS | Hai endpoint khác nhau, **có chủ ý**: `/schools/{id}/sis-imports` (School Admin của tenant) 10/giờ; `/sis/imports` (platform, gateway nội bộ) 5/giờ | [API_DESIGN](../07-backend/API_DESIGN.md), [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) |
| C-6 | `hub_instance_capacity` (40), `battle_1v1_reconnect_grace` (30 s) | Cùng giá trị ở nhiều nơi; nơi sở hữu: capacity → [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md), grace theo chế độ → [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md); nơi khác chỉ được nhắc kèm link | xem cột trước |
