# Valkey Design

> Trạng thái: Draft · Phiên bản: 0.5 · Owner: Backend Architect + DevOps · Cập nhật: 2026-11-17 (ADR-0090: cache `featuregate` `ew:fg:*`; ADR-0091: `setup_ticket` và rate limit đăng nhập portal; ADR-0074: owner key theo module, ACL user theo module, binary mở một client cho mỗi module nạp) · 2026-11-16
> Liên quan: [ADR-0004](../adr/ADR-0004-valkey-chi-cho-trang-thai-tam-thoi-cache-va-phoi-hop-realtim.md) · [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · [ADR-0042](../adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) · [EVENT_DESIGN](EVENT_DESIGN.md) · [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) · [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md)

**Mục đích & phạm vi.** Nguồn chuẩn duy nhất cho key space Valkey (tên key, kiểu, TTL, module sở hữu), Pub/Sub channel, lệnh mẫu và hành vi khi Valkey suy giảm.

> **Thay đổi v0.3:** Valkey **không còn dùng Streams** cho domain event (chuyển sang NATS JetStream, ADR-0042). Mỗi key pattern thuộc **một module** (cột Owner = module · thành phần); module khác không đọc/ghi key của module khác (kể cả khi chạy chung binary `core-api`/`worker`), trừ ticket realtime (module game ghi, realtime đọc) và các kênh Pub/Sub `ew:rt:*`, `ew:chat:fanout:*` (module publish, binary `realtime` subscribe).

## 1. Nguyên tắc

1. **Valkey không phải system of record** (ADR-0004). Mọi key phải thuộc một trong hai loại:
   - **Rebuildable** — tái tạo được từ PostgreSQL (cache, leaderboard cache, party state).
   - **Ephemeral** — chấp nhận mất (presence, queue, ticket, rate limit); client/hệ thống tự phục hồi.
2. **Mọi key có TTL**, trừ vài key được quản lý tường minh (queue matchmaking, seq chat, node registry) — liệt kê trong §3.
3. **Không PII trong key**; value chỉ chứa ID, nickname, trạng thái. Không lưu tên thật, ngày sinh, nội dung chat đầy đủ ngoài buffer resume ngắn hạn.
4. **Prefix theo module:** tên key thật = `ew:<module>:<phần còn lại>` với `<module>` ∈ {`identity`, `learning`, `game`, `competition`, `chat`, `gateway`}. Bảng §3.1 (và các tài liệu domain) viết tên **rút gọn** `ew:<domain>:...`; key builder của từng module (`src/backend/pkg/`) chèn `<module>` theo cột Owner, ví dụ `ew:mm:queue:...` → `ew:competition:mm:queue:...`, `ew:rt:ticket:...` → `ew:game:rt:ticket:...`, `ew:chat:seq:...` → `ew:chat:seq:...` (không lặp). Riêng cache Principal (mỗi module tự giữ) viết đầy đủ `ew:<module>:principal:...`. Ngoại lệ: **ticket realtime** do module game ghi và realtime đọc (`ew:game:rt:ticket:*`, ACL gateway chỉ đọc + `GETDEL` prefix này). **Kênh Pub/Sub** (§3.2: `ew:rt:*`, `ew:chat:fanout:*`, `ew:chat:sensitive:version`…) là namespace kênh dùng chung, phân quyền bằng ACL kênh (`&...`). Môi trường tách bằng **instance/DB riêng**, không bằng prefix.
5. Mỗi key pattern có **một module owner** duy nhất được ghi; module khác lấy dữ liệu qua facade/event của module owner (ADR-0074/0042), không đọc key trực tiếp.

## 2. Triển khai

| Hạng mục | Local | Staging/Prod | Nhãn |
|---|---|---|---|
| Phiên bản | Valkey 8.x | Valkey 8.x | [Confirmed] |
| Topology | 1 instance | Primary + 2 replica + 3 Sentinel (pilot); Valkey Cluster khi pub/sub/throughput vượt ngưỡng (§8) | [Confirmed] |
| Persistence | Tắt | AOF `appendfsync everysec` + RDB snapshot 15'; **không** dùng để khôi phục dữ liệu nghiệp vụ, chỉ giảm thời gian warm-up | [Confirmed] |
| Memory | 256 MB | `maxmemory` = 70% RAM; `maxmemory-policy volatile-lru` (chỉ evict key có TTL) | [Confirmed] |
| Bảo mật | không | ACL user theo module (`identity`, `learning`, `game`, `competition`, `chat`, `realtime`, `apisix_ratelimit`; `core-api` và `worker` mở một client Valkey cho mỗi module nạp, dùng user của module đó — ADR-0074): mỗi user chỉ key `~ew:<module>:*` của mình (+ `realtime` đọc/`GETDEL` `~ew:game:rt:ticket:*`, ghi `~ew:gateway:*`) và kênh Pub/Sub cần dùng (`&ew:rt:*`, `&ew:chat:fanout:*`); TLS; tắt `KEYS`, `FLUSHALL`, `DEBUG`, `CONFIG` cho user ứng dụng | [Confirmed] |
| Client Go | `valkey-io/valkey-go` (client-side caching tắt mặc định) | | ADR-0025 |

Quy tắc code: cấm `KEYS`; dùng `SCAN` chỉ trong job bảo trì. Script Lua/Functions đặt trong `src/backend/modules/<module>/internal/infrastructure/cache/scripts/`, có test.

## 3. Key space

### 3.1. Tổng quan

| Pattern | Kiểu | TTL | Owner | Loại |
|---|---|---|---|---|
| `ew:rt:ticket:<sha256>` | STRING (JSON) | 60 s | `game · game_session` | Ephemeral |
| `ew:rt:node:<node_id>:alive` | STRING | 10 s (gia hạn 3 s) | `realtime` | Ephemeral |
| `ew:game:active_session:<student_id>` | HASH {game_session_id, device_id, platform, node_id, last_seen_ms} | 90 s (gia hạn theo heartbeat 15 s) | `game · game_session` | **Cache** — nguồn sự thật là `game.game_sessions` (unique 1 session active, ADR-0031). Dùng để realtime biết node đang giữ kết nối của thiết bị cũ khi takeover |
| `ew:game:login_lock:<student_id>` | STRING owner | 5 s | `game · game_session` | Ephemeral — tránh hai thiết bị tạo session cùng lúc ở tầng cache (PG vẫn tuần tự hóa bằng `FOR UPDATE`) |
| `ew:rt:nodes` | ZSET node → load | Không TTL; dọn bởi watcher khi `alive` mất | `realtime` | Ephemeral |
| `ew:rt:node:<node_id>:conns` | SET conn_id | 1 h (gia hạn) | `realtime` | Ephemeral |
| `ew:rt:conn:<user_id>` | HASH {node, conn_id, game_session_id, since} — một kết nối hiệu lực mỗi học sinh, ghi bằng Lua trả về chủ cũ (F090-AC-08) | 90 s (gia hạn theo heartbeat) | `realtime` | Ephemeral |
| `ew:rt:resume:<sha256(token)>` | LIST `<seq>|<server envelope>` của message không droppable (≤500) | 120 s | `realtime` | Ephemeral |
| `ew:rt:resume:<sha256(token)>:meta` | HASH {user_id, game_session_id, last_seq, trimmed_seq, subs (JSON ≤ 64 topic)} — resume ở node bất kỳ (F090 research R3) | 120 s | `realtime` | Ephemeral |
| `ew:rt:cmd:<user_id>:<cmd_id>` | STRING kết quả `system.command.acked`/`failed` đã trả (hoặc `pending`) — chống chuyển tiếp lệnh trùng `id` (F090-AC-16) | 120 s | `realtime` | Ephemeral |
| `ew:rt:room:<room_id>` | HASH {owner_node, type, tenant_id (hoặc tenant_ids cho room liên tenant), school_id, capacity, members, state_version} | 2 h (gia hạn) | `competition · room` | Ephemeral |
| `ew:rt:room:<room_id>:lease` | STRING owner_node | 10 s | `competition · room` | Ephemeral |
| `ew:rt:room:<room_id>:ckpt` | STRING (JSON) | 1 h | `competition · battle/coop` | Ephemeral (kết quả cuối ghi PG) |
| `ew:rt:area:<area_id>:<tenant_id>:<school_id>:<grade_band>:instances` | ZSET room_id → số người | 2 h | `competition · room` | Ephemeral |
| `ew:rt:room:<room_id>:members` | HASH user_id → JSON {eid, PublicPlayer, school_id, grade_band, game_session_id, node} — chỗ ngồi của room hub (gồm chỗ đang giữ khi mất kết nối), owner dựng lại khi failover (F091) | 2 h (gia hạn) | `competition · room` | Ephemeral |
| `ew:rt:room:<room_id>:pos:<user_id>` | STRING [x,y,z] vị trí hợp lệ gần nhất (ghi ~1 Hz) — failover và facade khoảng cách của F071 | 5 s | `competition · room` | Ephemeral |
| `ew:rt:rooms` | ZSET room_id → lần gia hạn lease cuối (ms) — watcher failover tìm room mất owner | Phần tử xóa khi room đóng | `competition · room` | Ephemeral |
| `ew:rt:user:<user_id>` (module competition) | HASH {room_id, area_id} — hub instance học sinh đang giữ chỗ (vào lại, Tham gia bạn, đổi trang phục) | 2 h | `competition · room` | Ephemeral |
| `ew:presence:user:<user_id>` | HASH {hidden (0/1, ẩn trạng thái — F091), status (enum presence theo [MULTIPLAYER_GAMEPLAY §5](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#5-presence), dùng chung cho AsyncAPI và `PlayerCard`), area_id, room_id, activity, updated_at} | 90 s | `realtime · presence` (area/activity do module gửi qua `ew:rt:user`) | Ephemeral |
| `ew:presence:tenant:<tenant_id>:online` | ZSET user_id → last_seen_ms (lọc theo trường bằng presence user) | Tự dọn phần tử > 90 s | `realtime · presence` | Ephemeral |
| `ew:chat:social:presence:<user_id>` | HASH {status, activity, area_id, hidden, ts} — presence gần nhất module chat nhận từ tín hiệu `ew.rt.presence.<tenant_id>` (gateway phát lại `refresh` tối đa mỗi `presence_ttl`/3 khi kết nối còn sống); ghi bằng Lua chỉ khi `ts` mới hơn | `presence_ttl` (90 s) | `chat · social` (F095) | Ephemeral — hết hạn = `offline` trong `listFriends` |
| `ew:chat:social:friends:<user_id>` | SET user_id bạn bè (không chặn nhau) + phần tử `-` cho danh sách rỗng — fan-out `presence.friend.updated` | 10 phút; xóa khi đổi quan hệ (kết bạn, hủy, chặn) | `chat · social` (F095) | Rebuildable từ `social.friendships` |
| `ew:party:<party_id>` | HASH (state realtime) | 24 h (gia hạn khi hoạt động) | `competition · party` | Rebuildable |
| `ew:party:<party_id>:members` | HASH user_id → JSON {role, ready, status, joined_at} | 24 h | `competition · party` | Rebuildable |
| `ew:party:<party_id>:lock` | STRING owner (`SET NX`) — tuần tự hóa lệnh lobby (bắt đầu hoạt động, chuyển leader) | 5 s | `competition · party` | Ephemeral — PG `SELECT … FOR UPDATE` vẫn là chốt chặn |
| `ew:party:<party_id>:leader_grace` | STRING leader_id | `party_leader_grace` | `competition · party` | Ephemeral — hết hạn thì worker tự chuyển leader ([PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md)) |
| `ew:party:active` | ZSET party_id → last_online_ms | Không TTL; worker `party-presence-sweeper` dọn | `competition · party` | Rebuildable từ `multiplayer.parties` |
| `ew:party:user:<user_id>` | STRING party_id | 24 h | `competition · party` | Rebuildable |
| `ew:party:invite:<invitation_id>` | HASH | = `expires_at` (mặc định 5 phút) | `competition · party` | Rebuildable |
| `ew:party:invites:user:<user_id>` | ZSET invitation_id → expires_ms | 10 phút | `competition · party` | Rebuildable |
| `ew:lobby:<room_id>` | HASH {match_id?, state, deadline_ms} | 30 phút | `competition · lobby` | Ephemeral |
| `ew:lobby:<room_id>:ready` | HASH user_id → 0/1 | 30 phút | `competition · lobby` | Ephemeral |
| `ew:mm:queue:<mode>:<scope>:<grade_key>:<subject>[:<topic>]` (`<scope>` = phạm vi ghép theo [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md), vd. `school:<school_id>`, `tenant:<tenant_id>`) | ZSET ticket_id → enqueue_ms | Không TTL key; phần tử dọn bởi matchmaker (ticket TTL) | `competition · matchmaking` | Ephemeral |
| `ew:mm:ticket:<ticket_id>` | HASH {queue_key, user_ids, party_id, grade_level, tenant_id, school_id, rating, rd, constraints, status} | 10 phút | `competition · matchmaking` | Ephemeral |
| `ew:mm:user:<user_id>` | STRING ticket_id | 10 phút | `competition · matchmaking` | Ephemeral |
| `ew:mm:proposal:<proposal_id>` | HASH {ticket_ids, accepted, deadline_ms} | 30 s | `competition · matchmaking` | Ephemeral |
| `ew:mm:lock:<queue_key>` | STRING worker_id | 5 s (gia hạn) | `competition · matchmaking` | Ephemeral |
| `ew:mm:queues` | SET queue_key đang có ticket | Không TTL; matchmaker dọn khi queue rỗng | `competition · matchmaking` | Ephemeral |
| `ew:mm:cooldown:<student_id>` | STRING | Theo `mm_ticket_rate` | `competition · matchmaking` | Ephemeral — chặn tạo ticket dồn dập |
| `ew:rl:<scope>:<subject>:<window>` | STRING counter / ZSET | = window | `mỗi module (src/backend/pkg/httpx)` | Ephemeral |
| `ew:idem:<principal_id>:<key>` | STRING (JSON status + response ref) | 24 h | `mỗi module (src/backend/pkg/httpx)` | Ephemeral cache — bản ghi chuẩn ở `infra.idempotency_records` |
| `ew:<module>:principal:<user_id>:<tenant_id>` (tên đầy đủ) | STRING (JSON Principal của phiên tenant + `version`; phiên platform dùng `<tenant_id>` = `platform`) | `principal_cache_ttl` | mỗi module (`src/backend/pkg/authz`, cache kết quả facade `principals:resolve` của module identity) | Rebuildable — xóa khi `user_role.changed`, `role.updated` (mọi user có role đó; xóa theo version role [Confirmed]), `tenant_membership.changed`, `enrollment.changed`, `user.access_revoked`, `tenant.updated`. Principal chứa permission = hợp các UserRole hiệu lực (ADR-0066). Khóa phụ cùng TTL (F002): `ew:<module>:principal:sub:<sha256(iss,sub)[:16]>` → `user_id` (token không mang `user_id`) và `ew:<module>:principal:idx:<user_id>` (SET phần tenant) cho invalidate toàn user; L1 trong tiến trình 15 s; `ClientID`/`sid`/MFA không cache |
| `ew:id:login_ticket:<sha256(ticket)>` | STRING (JSON `{user_id, device_id, choices[], token_ref}` — token được mã hóa bằng khóa của module identity, **không** chứa mật khẩu) | 2 phút, `GETDEL` một lần | `identity · identity` | Ephemeral — chọn tenant sau `POST /auth/game/login` khi học sinh thuộc nhiều tenant (ADR-0068); mất key = học sinh đăng nhập lại |
| `ew:id:login_rl:<sha256(username)>` | STRING counter | 60 s | `identity · identity` | Ephemeral — đếm lần đăng nhập game theo username (bổ sung cho APISIX, 10/phút, ADR-0068) |
| `ew:fg:<tenant_id>` / `ew:fg:platform` (tên đầy đủ `ew:identity:fg:…`) | HASH `{catalog_version, disabled_menus[], disabled_actions[]}` — trạng thái bật/tắt menu/action **hiệu lực** của Admin Portal theo tenant (`platform` = mặc định platform, dùng cho System Admin liên tenant không gửi `X-Tenant-Id`) | `feature_toggle_cache_ttl` (60 s, ≤ `principal_cache_ttl`); xóa ngay khi nhận `platform_settings.changed` scope `feature_flag` | `identity · featuregate.Source` (ADR-0090) | Rebuildable — nguồn `platform.feature_flags` + `platform.portal_menu_catalog`; miss + DB lỗi → `featuregate` trả `503 UNAVAILABLE` (fail-closed) |
| `ew:portal:setup:<sha256(setup_ticket)>` (tên đầy đủ `ew:identity:portal:setup:…`) | STRING (JSON `{user_id, required_steps[], otp_email_sent_at?}`, **không** chứa mật khẩu/token) | `portal_setup_ticket_ttl` (10 phút), `GETDEL` một lần | `identity · auth_portal` (ADR-0091) | Ephemeral — mất thì người dùng đăng nhập lại để nhận ticket mới |
| `ew:id:portal_login_rl:<kind>:<sha256(value)>` (`kind` = `user`/`device`/`global`/`forgot`) | STRING counter | 60 s (`forgot`: 1 giờ) | `identity · auth_portal` | Ephemeral — đếm theo `portal_login_rate` (bổ sung APISIX; không key theo IP, R30) |
| `ew:id:memberships:<user_id>` | SET tenant_id (membership theo DB, ADR-0026) | 60 s | `identity · identity` | Rebuildable |
| `ew:tenant:alias:<alias>` | STRING (JSON {tenant_id, tenant_type, status}) | 5 phút | `identity · tenancy` | Rebuildable — xóa khi `tenant.updated` |
| `ew:tenant:assign:<tenant_id>:<resource_type>` | SET resource_id được gán (ADR-0029) | 10 phút | `identity · tenancy` | Rebuildable — xóa khi `tenant_data.assigned`/`unassigned` |
| `ew:cache:content:qv:<question_version_id>` | STRING (render payload, **không** answer key) | 24 h | `learning · question_bank` | Rebuildable |
| `ew:<module>:cv:<item_type>:<item_id>:<version_no>` | STRING (snapshot đã dựng của một version, JSON nén + sha256; **không** answer key) — `<module>` ∈ {learning, game, chat} | TTL trượt 7 ngày (gia hạn khi đọc) | module sở hữu nội dung · snapshot worker | Rebuildable từ `*_versions` — **bất biến**, không bao giờ invalidate (ADR-0043) |
| `ew:<module>:current:<tenant_id>:<item_type>:<item_id>` | STRING version_id hiện hành (con trỏ release); nội dung platform không gán tenant cụ thể dùng `<tenant_id>` = `_platform` | 1 h; ghi đè khi `content.published`/`rolled_back`/`unpublished` | module sở hữu nội dung | Rebuildable từ `content_items.current_version_id` |
| `ew:game:manifest:<tenant_id>` | HASH {manifest_version, etag, body (JSON nén)} | 1 h; ghi đè khi `content.manifest_updated` | `game · content` | Rebuildable từ `game.content_manifests` — phục vụ `GET /game/content-manifest` (304 theo etag) |
| `ew:game:level_session:<level_session_id>` | STRING (content_versions ghim) | 6 h | `game · level` | Cache — nguồn sự thật `game.level_sessions` |
| `ew:learning:key:<question_version_id>` | STRING (answer key + scoring rule, mã hóa) | 1 h | `learning · question_bank` | Rebuildable — chỉ ACL user `learning`; không bao giờ trả client, không lên CDN |
| `ew:cache:content:qv:key:<question_version_id>` | STRING (answer key mã hóa) | 1 h | `learning · question_bank` | Rebuildable — chỉ ACL user `learning` (chấm điểm chỉ ở module learning, kể cả cho trận đấu) |
| `ew:cache:policy:chat:<tenant_id>:<school_id>` | STRING (ChatPolicy đã resolve) | 5 phút | `chat · chat` | Rebuildable |
| `ew:cache:config:<name>:<version>` | STRING | 10 phút | module tương ứng | Rebuildable |
| `ew:lb:<tenant_id>:<leaderboard_id>:<scope_id>:<period_key>` | ZSET student_id → score (điểm × 1e6 − thời điểm đạt, để phá hòa) | Theo kỳ + 7 ngày | `competition · leaderboard` | Rebuildable |
| `ew:lb:<tenant_id>:<leaderboard_id>:<scope_id>:<period_key>:meta` | HASH {rebuilt_at, source_version, checksum} | Như trên | `competition · leaderboard` | Rebuildable |
| `ew:chat:unread:<user_id>` | HASH channel_id → count | 7 ngày | `chat · chat` | Rebuildable (từ `last_read_seq`) |
| `ew:chat:seq:<channel_id>` | STRING counter — cấp `seq` cho mọi tin trước khi publish NATS (ADR-0035); script `seq_next.lua` (§4.9) | Không TTL; `EXPIRE` 30 ngày sau khi channel đóng | `chat · chat` | Rebuildable — khởi tạo lại từ `MAX(seq)` PG; unique `(channel_id, seq)` ở PG là chốt chặn |
| `ew:chat:recent:<channel_id>` | LIST (envelope JSON đã kiểm duyệt, mới nhất trước), `LTRIM` 200 | 10 phút (gia hạn mỗi lần ghi) | `chat · chat` | Ephemeral — che độ trễ `chat-persister` cho lịch sử/`chat.sync` |
| `ew:chat:cmid:<sender_id>:<client_message_id>` | STRING message_id (`SET NX`) | 10 phút | `chat · chat` | Ephemeral — idempotency lệnh gửi |
| `ew:chat:typing:<channel_id>` | HASH user_id → expires_at | 5 s | `chat · chat` | Ephemeral — không lưu PG |
| `ew:chat:ntf:unread:<user_id>` | STRING counter thông báo chưa đọc (`social.notifications`) | 7 ngày | `chat · notification` | Rebuildable — mất thì đếm lại từ PG |
| `ew:chat:mentions:<user_id>` | HASH channel_id → unread_mentions | 7 ngày | `chat · chat` | Rebuildable (từ `channel_members.unread_mentions`) |
| `ew:chat:cursor_buf:<channel_id>:<user_id>` | HASH {delivered_seq, read_seq, dirty_at} | 1 phút | `chat · chat` | Ephemeral — gộp receipt trước khi publish JetStream (≤ 1 lần/2 s) |
| `ew:chat:sensitive:compiled:<scope>:<version>` | STRING (bản biên dịch danh mục từ nhạy cảm đã tuần tự hóa + checksum; `scope` = `platform` hoặc `tenant:<tenant_id>`) | 7 ngày | `chat · moderation` | Rebuildable từ PG (`chat.sensitive_*`); chỉ node backend đọc, không bao giờ trả client (ADR-0030) |
| `ew:chat:sensitive:current` | HASH scope → catalog_version hiện hành | Không TTL | `chat · moderation` | Rebuildable từ `chat.sensitive_catalog_versions` |
| `ew:chat:switches:<tenant_id>` | HASH {enabled_all, steps (json), expires_at, snapshot_id} — giá trị **hiệu lực** đã kết hợp platform + tenant; `platform` dùng key `ew:chat:switches:platform` | 10 phút (refresh khi đổi) | `chat · moderation` | Rebuildable từ `chat.moderation_switches`; mất cache → đọc PG (không mặc định tắt) |
| `ew:gam:cap:<student_id>:<rule_id>:<yyyymmdd>` | STRING counter | 48 h | `game · gamification` | Ephemeral cache; cap chuẩn kiểm tra lại trong PG khi trao |
| `ew:lock:<name>` | STRING owner | ≤ 30 s | `mỗi module` | Ephemeral — gồm `ew:lock:job:student-inactive-cleanup:<tenant_id>`, `ew:lock:job:gameplay-chat-purge`, `ew:lock:job:game-session-timeout` |
| `ew:settings:<key>:<scope>` | STRING (JSON giá trị hiệu lực, vd. `student_cleanup`, `chat_retention`; `scope` = `platform` hoặc `tenant:<id>`) | 10 phút; xóa khi nhận `platform_settings.changed` | `identity · tenancy` | Rebuildable từ `platform.platform_settings` + `platform.tenants.settings` |
| `ew:<module>:dash:ver:<tenant_id>` (svc = `learning`/`game`) | STRING counter — phiên bản dữ liệu dashboard của tenant, tăng khi chốt ngày/backfill | Không TTL | `learning · analytics` / `game · analytics` | Rebuildable — mất thì bắt đầu lại từ 1 (cache cũ tự hết hạn) |
| `ew:<module>:dash:res:<tenant_id>:<ver>:<hash>` | STRING (kết quả truy vấn dashboard đã serialize, nén) | 5 phút (khoảng chứa hôm nay) / 24 h (đã chốt) | như trên | Cache — nguồn sự thật bảng rollup ([DASHBOARD_DATA §9](DASHBOARD_DATA.md#9-cache)) |
| `ew:<module>:dash:hll:<metric>:<tenant_id>:<scope>:<yyyymmdd>` | HyperLogLog (PFADD/PFCOUNT) người dùng hoạt động hôm nay | 3 ngày | như trên | Ephemeral — chỉ số tạm tính; số chốt tính lại từ PostgreSQL |
| `ew:<module>:dash:inflight:<user_id>` | STRING counter truy vấn dashboard đang chạy (giới hạn 4/người) | 30 s | như trên | Ephemeral |
| `ew:<module>:dash:lock:<hash>` | STRING (SET NX) chống cache stampede | 5 s | như trên | Ephemeral |

**Mở rộng sau (ADR-0063):** khóa `ew:tenant:code:<login_code>` (tra tenant trước đăng nhập, `/auth/tenants:resolve`) và `ew:identity:class_login_code:<sha256(code)>` (mã lớp + PIN) **không dùng trong Release 1.0** — đăng nhập không chọn tenant trước và chỉ bằng username + mật khẩu (ADR-0063, ADR-0068, ADR-0069). Vé chọn tenant sau đăng nhập game dùng khóa `ew:id:login_ticket:<sha256(ticket)>` ở bảng trên (`/auth/game/tenant:select`).

`<grade_key>`: `g<n>` (ghép theo đúng khối, mặc định lớp 1–5) hoặc `gb:<grade_band>` (lớp 6–9 cho phép ±1 trong band) — xem [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md).

### 3.2. Pub/Sub channels

| Channel | Publisher | Subscriber |
|---|---|---|
| `ew:rt:user:<user_id>` | mọi module (`core-api`/`worker`) — sự kiện cho một người | `realtime` node đang giữ kết nối user |
| `ew:chat:fanout:<channel_id>` | module chat (sau khi tin đã qua moderation **và** NATS JetStream ack — ADR-0035); cũng mang `chat.message.edited/recalled`, `chat.reaction.updated`, `chat.pin.updated`, `chat.typing.updated`, `chat.receipt.updated` | `realtime` node có member online |
| `ew:rt:party:<party_id>` | module competition · party | `realtime` node có member online |
| `ew:rt:room:<room_id>:out` | module competition (instance giữ lease room) | `realtime` node có member trong room |
| `ew:rt:node:<node_id>:in` | realtime node bất kỳ (forward tín hiệu kết nối, vd. takeover) | Node `<node_id>` |
| `ew:rt:broadcast` | module identity (thông báo bảo trì, admin) | Tất cả node gateway |
| `ew:chat:sensitive:version` | module chat worker (sau khi publish danh mục) | Mọi instance module chat (api): biên dịch bản mới ở nền rồi hoán đổi nguyên tử; payload `{scope, tenant_id?, version}` |
| `ew:chat:switches:changed` | module chat api (công tắc kiểm duyệt) / worker (tự bật lại khi hết hạn) | Mọi instance module chat: xóa cache `ew:chat:switches:<tenant_id>` |

Lệnh từ client đi theo chiều ngược lại **không** qua Valkey: gateway gửi NATS request `ew.cmd.<module>.<domain>.<action>` tới handler của module trong `core-api` (ADR-0074). Khi chuyển sang Valkey Cluster: dùng **sharded pub/sub** (`SSUBSCRIBE`/`SPUBLISH`) cho các channel theo ID.

### 3.3. Streams

Không dùng. Domain event đi qua outbox → NATS JetStream (ADR-0042, [EVENT_DESIGN §3](EVENT_DESIGN.md)); tin nhắn chat đi qua NATS JetStream `CHAT_MESSAGES` (ADR-0035).

## 4. Ví dụ lệnh

### 4.1. Realtime ticket

```text
# module game cấp ticket
SET ew:rt:ticket:9f2c…e1 '{"user_id":"…","game_session_id":"…","tenant_id":"…","school_id":"…","iat":1763190000}' EX 60 NX
# realtime tiêu thụ (dùng 1 lần)
GETDEL ew:rt:ticket:9f2c…e1
```

### 4.2. Presence

```text
HSET ew:presence:user:<uid> status online area_id main_hub room_id <rid> activity idle updated_at 1763190000123
EXPIRE ew:presence:user:<uid> 90
ZADD ew:presence:tenant:<tid>:online 1763190000123 <uid>
# Fan-out tới bạn bè: lấy danh sách bạn (cache) → PUBLISH ew:rt:user:<friend_id> {...presence.friend.updated...}
```

Presence của học sinh chỉ gửi cho bạn bè/party/lớp theo policy, không công khai toàn trường.

### 4.3. Party (atomic join — Lua)

```lua
-- KEYS[1]=ew:party:<pid>  KEYS[2]=ew:party:<pid>:members  KEYS[3]=ew:party:user:<uid>
-- ARGV[1]=uid ARGV[2]=member_json ARGV[3]=ttl_s
if redis.call('EXISTS', KEYS[3]) == 1 then return {err='ALREADY_IN_PARTY'} end
local max = tonumber(redis.call('HGET', KEYS[1], 'max_members'))
if redis.call('HLEN', KEYS[2]) >= max then return {err='PARTY_FULL'} end
redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
redis.call('SET', KEYS[3], string.match(KEYS[1], 'ew:party:(.+)'), 'EX', ARGV[3])
redis.call('HINCRBY', KEYS[1], 'state_version', 1)
return redis.call('HGET', KEYS[1], 'state_version')
```

Thứ tự chuẩn: command `JoinParty` của module competition ghi `eduworld_competition` trước (`multiplayer.party_members`, unique active membership), **rồi** cập nhật Valkey bằng script trên và publish `party.member.joined`. Nếu Valkey lỗi → state realtime được rebuild từ PG ở lần đọc sau. Script chỉ là lớp bảo vệ race tốc độ cao; ràng buộc unique ở PG là nguồn sự thật.

### 4.4. Matchmaking queue

```text
HSET ew:mm:ticket:<tid> queue_key battle_1v1:school:<sid>:g3:math user_ids <uid> grade_level 3 tenant_id <tenant> school_id <sid> rating 1500 rd 200 status queued
EXPIRE ew:mm:ticket:<tid> 600
ZADD ew:mm:queue:battle_1v1:school:<sid>:g3:math 1763190000123 <tid>
SET ew:mm:user:<uid> <tid> EX 600 NX          # chống 2 ticket cùng lúc
# Matchmaker (worker module competition giữ lock):
SET ew:mm:lock:battle_1v1:school:<sid>:g3:math <worker_id> NX PX 5000
ZRANGE ew:mm:queue:battle_1v1:school:<sid>:g3:math 0 199   # lấy 200 ticket cũ nhất → ghép theo rating window mở rộng theo thời gian chờ
ZREM ew:mm:queue:battle_1v1:school:<sid>:g3:math <tid1> <tid2>   # trong MULTI/Lua cùng tạo proposal
```

### 4.5. Rate limit (sliding window bằng ZSET, cho chat)

```text
ZREMRANGEBYSCORE ew:rl:chat:<uid>:60s 0 <now-60000>
ZADD ew:rl:chat:<uid>:60s <now> <msg_id>
ZCARD ew:rl:chat:<uid>:60s         # so với limit từ ChatPolicy
EXPIRE ew:rl:chat:<uid>:60s 60
```

Fixed-window cho API đơn giản: `INCR ew:rl:api:<sub>:<route>:<minute>` + `EXPIRE 60`. APISIX hiện thực bằng plugin Lua `ew-api-rate` (F004): khóa `ew:rl:api:<sub>:<profile>:<read|write>:<cửa sổ>` (`profile` = `api`/`session` trong `deploy/apisix/routes/routes.yaml`), user ACL `apisix_ratelimit` chỉ prefix `~ew:rl:*` và lệnh `INCR`, `EXPIRE`; mật khẩu từ biến môi trường gateway `EW_APISIX_VALKEY_PASSWORD`. Valkey không phản hồi → cho qua và ghi cảnh báo (allow_degradation).

### 4.6. Idempotency

```text
SET ew:idem:<uid>:<key> '{"status":"in_progress"}' EX 86400 NX   # nếu NX thất bại → đọc trạng thái
# hoàn tất:
SET ew:idem:<uid>:<key> '{"status":"done","http":200,"resource":"attempt/…"}' EX 86400 XX
```

Bản ghi chuẩn ghi trong `infra.idempotency_records` **cùng transaction** nghiệp vụ; Valkey chỉ để trả lời nhanh request lặp. Nếu Valkey mất → kiểm tra PG.

### 4.7. Leaderboard cache

```text
# key luôn bắt đầu bằng tenant: ew:lb:<tenant_id>:<leaderboard_id>:<scope_id>:<period_key>
ZINCRBY ew:lb:<tenant_id>:<leaderboard_id>:<sid>:2026W46 25 <student_id>
ZREVRANGE ew:lb:<tenant_id>:<leaderboard_id>:<sid>:2026W46 0 49 WITHSCORES
ZREVRANK ew:lb:<tenant_id>:<leaderboard_id>:<sid>:2026W46 <student_id>
```

Nguồn chuẩn: `competition.match_results` / `competition.leaderboard_snapshots` trong `eduworld_competition`; `<leaderboard_id>` là ID định nghĩa bảng xếp hạng ([DATABASE_DESIGN §2.10](DATABASE_DESIGN.md)), mang mode, môn và loại kỳ. Job worker module competition rebuild toàn bộ ZSET khi `meta.source_version` lệch hoặc key mất.

### 4.9. Cấp seq chat (Lua, ADR-0035)

```lua
-- KEYS[1] = ew:chat:seq:<channel_id>; ARGV[1] = pg_max_seq (module chat đọc từ channels.last_seq khi GET trả nil)
if redis.call('EXISTS', KEYS[1]) == 0 then
  redis.call('SET', KEYS[1], ARGV[1], 'NX')
end
return redis.call('INCR', KEYS[1])
```

module chat chỉ đọc `eduworld_chat` khi key không tồn tại (lần đầu hoặc sau khi Valkey mất). Vì persister có thể chưa ghi kịp, giá trị khởi tạo dùng `GREATEST(channels.last_seq, max seq trong ew:chat:recent)`; trùng hiếm gặp bị unique PG chặn và persister gán seq mới + phát `chat.message.resequenced` [Confirmed].

## 5. Dữ liệu cấm chỉ lưu trong Valkey (ADR-0004)

| Dữ liệu | Nguồn chuẩn PostgreSQL |
|---|---|
| Learning records, learning events | `learning.learning_events` |
| Attempt, câu trả lời, điểm, kết quả thi | `assessment.*` |
| Match results, rating | `competition.match_results`, `competition.ratings` |
| Tournament bracket | `competition.tournaments`, `competition.bracket_nodes` |
| Student/class/enrollment | `org.*` |
| Nội dung đã publish | `content.*`, `game.*_versions` |
| Party membership (bền vững), invitation | `multiplayer.party_members`, `multiplayer.party_invitations` |
| Tin nhắn chat, report, moderation case | `chat.*` |
| Game session active, lịch sử đăng nhập | `game.game_sessions`, `game.login_events` (`eduworld_game`), `identity.users.last_login_at` (`eduworld_identity`) |
| XP, reward grant, inventory | `gamification.*`, `game.inventory_items` |
| Idempotency (thao tác quan trọng) | `infra.idempotency_records` (database của từng module) |

Code review checklist: mọi PR thêm key Valkey mới phải cập nhật bảng §3.1 (loại, TTL, owner).

## 6. Khi Valkey mất / suy giảm

| Chức năng | Ảnh hưởng | Hành vi hệ thống |
|---|---|---|
| Realtime ticket | Không mở kết nối WS mới | module game trả `503 UNAVAILABLE`; game vẫn dùng REST (làm bài, quest đơn) — **chế độ suy giảm** |
| Pub/sub | Chat/party/notification không fan-out xuyên node | Kết nối hiện hữu chỉ nhận message cùng node; client poll REST mỗi 10 s khi nhận `system.service.degraded`. Chat vẫn **bền vững** vì đã nằm trong NATS JetStream → PG; client bù bằng lịch sử khi thấy seq gap |
| Cấp `seq` chat | Không cấp được seq | module chat từ chối gửi `CHAT_TEMPORARILY_UNAVAILABLE`; khi phục hồi, `seq_next.lua` khởi tạo lại từ PG |
| Presence | Bạn bè hiển thị offline | Chấp nhận |
| Matchmaking | Queue mất | Worker phát `matchmaking.ticket.cancelled{reason:"system"}` khi phục hồi; client tự re-queue |
| Party realtime state | Mất | Rebuild từ PG khi truy cập (`party.RebuildState`) |
| Battle đang chạy | Room vẫn chạy in-memory ở instance module competition giữ lease (không cần Valkey cho tick); không giao được sự kiện tới gateway khác node → client nhận lại snapshot khi phục hồi; mất checkpoint + instance chết → match `aborted`, không tính rating | |
| Rate limit | APISIX `limit-count` fallback: **fail-open có giới hạn** bằng `limit-req` local per node | Ghi cảnh báo |
| Idempotency cache | Kiểm tra PG | Latency tăng |
| Cache nội dung/principal | Đọc PG | Latency tăng, theo dõi DB load |
| Leaderboard | Trả từ PG (chậm hơn) hoặc `503` cho leaderboard global | Rebuild job sau phục hồi |

Mọi client Valkey dùng timeout (dial 1 s, read/write 500 ms) và circuit breaker; lỗi Valkey **không** làm hỏng transaction PG đã commit.

Readiness của `realtime` fail khi Valkey không kết nối được (không nhận kết nối mới); `core-api` vẫn ready (chế độ suy giảm). Domain event **không** bị ảnh hưởng vì đi qua NATS JetStream.

## 7. Metrics & cảnh báo

| Metric | Ngưỡng cảnh báo [Confirmed] |
|---|---|
| `used_memory / maxmemory` | > 75% |
| `evicted_keys` tăng | > 0 trong 5 phút (key không TTL không bị evict, nên evict = thiếu RAM) |
| Replication lag | > 5 s |
| Độ dài `ew:mm:queue:*` | Theo dõi phân phối; ticket chờ > 120 s |
| Pub/sub msg/s | > 70% năng lực đo được khi load test |
| Latency lệnh p99 | > 5 ms |

## 8. Khi nào cần Valkey Cluster / tách instance

- Pub/sub > ~100k msg/s hoặc CPU primary > 60% kéo dài → tách **instance riêng cho realtime (pub/sub, presence, room)** và **instance cho cache/rate limit**; có thể tách thêm instance theo module khi một module chiếm phần lớn tải.
- Dữ liệu > 70% RAM của node lớn nhất hợp lý → Valkey Cluster; key liên quan dùng hash tag `{party:<pid>}` để script Lua chạy trên cùng slot (vd. `ew:party:{<pid>}`, `ew:party:{<pid>}:members`). Để đổi một chỗ, **mọi key phải được tạo qua key builder tập trung** (`src/backend/pkg/` key builder + `internal/infrastructure/cache/keys.go` của từng module), cấm nối chuỗi key rải rác; script Lua chỉ nhận key qua `KEYS[]` (không tự dựng key bên trong script — ví dụ §4.3 tách `party_id` từ key chỉ để minh họa, bản thật truyền qua `ARGV`).
