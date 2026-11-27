# Feature Specification: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Feature Branch**: `115-chat-core` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` · **Mốc**: M4a · **Phụ thuộc**: F090, F095
**Input**: Lõi chat của module chat: mô hình channel và thành viên, resolve `ChatPolicy` 4 cấp (không có rule = deny, không điều kiện theo khối), đường gửi duy nhất (WS qua `realtime` và REST fallback) đi qua giới hạn kỹ thuật + pipeline kiểm duyệt, cấp `seq`, ghi bền vững vào NATS JetStream trước khi xác nhận "Sent", fan-out qua Valkey Pub/Sub, `chat-persister` ghi PostgreSQL, lịch sử và bù tin theo `seq`. FR nguồn: FR-CHAT-002, FR-CHAT-008, FR-CHAT-017, NFR-SAFE-001.

## Clarifications

### Session 2026-11-16

- Q: F115 có tự tạo channel không? → A: Có command nội bộ `OpenChannel`/`AddMember`/`RemoveMember` (không có API public); consumer tạo channel gameplay ở F117, channel bền vững ở F120. Test của F115 gọi command qua fixture.
- Q: Các bước kiểm tra nội dung (danh mục, PII, spam, bắt nạt) có ở F115 không? → A: Không. F115 định nghĩa port `ContentValidator` và bắt buộc mọi tin gọi port đó; F116 hiện thực. module chat **không khởi động** ở môi trường không phải `test` nếu chưa nối `ContentValidator` thật (R17, NFR-SAFE-001).
- Q: Sanction (mute) kiểm tra ở đâu? → A: Bảng `chat.sanctions` và cổng kiểm tra sanction thuộc F116; F115 chỉ chừa vị trí trong chuỗi "giới hạn kỹ thuật luôn chạy" ([MODERATION §3](../../docs/03-social-multiplayer/MODERATION.md)).
- Q: Rate limit theo `ChatPolicy` (CHAT_SYSTEM §3.1) hay "1 msg/2 s, 20/phút" (REALTIME_ARCHITECTURE §8)? → A: Theo `rate` của effective policy + trần `chat_rate_per_user` (CHAT_SYSTEM sở hữu giới hạn chat, CONFIG_DEFAULTS); REALTIME_ARCHITECTURE cần sửa.
- Q: Trường id idempotency REST là `client_message_id` (OpenAPI) hay `client_msg_id` (AsyncAPI)? → A: Thay bởi Session 2026-11-17 (một tên).

### Session 2026-11-17

- Q: Tên khóa idempotency tin nhắn sau đối soát (RB-508)? → A: Một tên `client_message_id` ở REST, realtime và DB ([CHAT_SYSTEM §5](../../docs/03-social-multiplayer/CHAT_SYSTEM.md)); AsyncAPI đổi `client_msg_id` → `client_message_id` (delta contract). Tiêu chí chi tiết của CHAT_SYSTEM dùng mã `AC-CHS-NNN`; cột Nguồn của feature giữ mã PRD `AC-CHAT-NNN` (RB-505).
- Q: Từ chối gửi chat trong giờ giới nghiêm và áp preset "chỉ quick message + emote" ở đường gửi (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-20 (owner chốt — việc nhận từ OQ F016-2/F045-3)

- Facade vòng đời học sinh và gộp tài khoản của module chat → **Chốt:** F115 hiện thực `cleanupStudentData`/`restoreStudentData`/`anonymizeUserData` (theo chính sách tin nhắn của CHAT_SYSTEM, không xóa vật lý — R21) và `mergeUserData` (cập nhật tham chiếu thành viên channel), idempotent theo `merge_id`.

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

Quyết định khi triển khai (đề xuất bảo thủ nhất, phù hợp ADR; owner đã duyệt trước các mặc định đề xuất):

- Bảng tin phân vùng `chat.messages` / `chat.gameplay_messages` / `chat.gameplay_partition_channels` → **Chốt:** DDL viết tay theo DATABASE_DESIGN §3.8/§3.8b (khóa chính phức hợp của bảng phân vùng ent không biểu diễn được; tiền lệ `competition.integrity_signals`), partition `DEFAULT` cho tới khi có job partition của bảng nghiệp vụ; ent chỉ có `Channel`, `ChannelMember`, `ChatPolicy`. `chat.channel_members` dùng khóa đại diện `channel_member_id` + unique `(channel_id, user_id) WHERE deleted_at IS NULL` (ent cần khóa một cột).
- Unique `(channel_id, seq)` (F115-AC-20) → **Chốt:** khóa unique của bảng phân vùng buộc chứa cột phân vùng nên persister giữ `pg_advisory_xact_lock` theo channel và kiểm `(channel_id, seq)` trước khi ghi; tin trùng `seq` (mất key Valkey) vào DLQ + metric, chưa gán lại `seq` (`chat.message.resequenced` chưa có tiêu chí — để sau).
- `quick_message_id` → **Chốt:** `varchar(64)` ở DB, REST và AsyncAPI (contracts §6). Tin `kind = emoji` lưu mã emoji ở `body`/`body_display`.
- `unaccent` → **Chốt:** cài trong schema `chat` (`chat.unaccent`), không trong `public` (role ứng dụng chỉ có USAGE trên `chat`, đặc quyền tối thiểu).
- Chưa có bộ kiểm tra nội dung thật (F116) → **Chốt:** `core-api` và `worker` profile `chat`/`all` **từ chối khởi động** ngoài `EW_ENV=test` (kể cả `local`) cho tới khi F116 nối `pipeline.Validator`; `EW_ENV=test` dùng test double có nhãn (`TestDoubleValidator`, quyết định `unchecked`). Profile `events`/`jobs` không có đường gửi nên không bị chặn.
- Giới nghiêm và preset → **Chốt:** module chat chiếu `ref.schools` (`settings_subset.curfew_window`) và `ref.platform_settings` (`platform_curfew`) qua consumer `chat.projection_identity` (`school.synced`, `school.updated`, `platform_settings.changed`); mặc định `EW_PLATFORM_CURFEW` cho tới khi có bản chiếu. `chat_policy_preset` nằm trong tài liệu policy cấp `school` (`chat.chat_policies.rules`; thêm thuộc tính tùy chọn vào `chat-policy.schema.json`, F120 ghi). Không có cờ "mặc định pilot" trong code.
- Gửi khi PostgreSQL ngừng (F115-AC-16) → **Chốt:** facts của đường gửi (channel + thành viên + thẻ + block, effective policy, giới nghiêm) cache Valkey `ew:chat:facts:*` (tươi `chat_facts_cache` 30 s, policy `chat_policy_cache`, giới nghiêm 60 s) và phục vụ bản cũ tới `chat_facts_stale_ttl` (1 giờ) khi không đọc được PostgreSQL; không có giá trị nào → `CHAT_TEMPORARILY_UNAVAILABLE` (fail closed). Persister giữ lô (`InProgress`) khi PostgreSQL ngừng, không tốn lần giao, không đưa vào DLQ.
- Phản hồi WS → **Chốt:** `chat.message.acked`, `chat.message.held`, `chat.history.loaded` đi qua `ew:rt:user:<id>` với `target.conn_id` (envelope Pub/Sub không mang `ref`, nên `ChatHistoryLoaded` bỏ `required: [ref]`); `system.command.acked` của lệnh gốc mang `result` (payload acked, hoặc `{channel_id, count, has_more}`). `ChatMessage.moderation_state` thêm `held` (chỉ trong lịch sử của chính người gửi).
- Fan-out → **Chốt:** `target.user_ids` = thành viên đang hoạt động trừ người có block hai chiều với người gửi (thành viên đã bị gỡ không nhận tin dù gateway chưa hủy subscribe); mute để F117 (research R2). Lịch sử cũng ẩn tin của người có quan hệ block.
- Giới hạn tốc độ → **Chốt:** `rate.messages_per_minute` theo channel + `min(global_rate, chat_rate_per_user)` theo người, cửa sổ trượt Valkey `ew:chat:rl:*` theo đồng hồ của use case; `burst` và `slow_mode_seconds` để TechnicalGate của F116.
- `chat.sync` → **Chốt:** `chat.channel.sync` (T043, OQ F090-1); `client_message_id` bắt buộc trong `chat.message.send`.
- Envelope JetStream → **Chốt:** schema ở `specs/events/chat-envelope.v1.schema.json` (không phải payload outbox, `check-cross-features` chỉ nhận `payloads/<module>/<event_type>`); DLQ dùng stream `DLQ_CHAT` subject `ew.dlq.chat.persister` (EVENT_DESIGN §4.10); `infra.dead_letters` chỉ giữ id, không nội dung (R14).

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Channel `class`, `study_group` và cấp policy `class`? → A: bỏ (ADR-0095 mục 5, FR-CHAT-001, FR-MOD-004); không tạo kênh khối (R15 không đổi). Còn 6 loại channel `world`, `area`, `match`, `party`, `team`, `direct`; policy 3 cấp platform → tenant → school; khóa cache `chat_policy_cache` theo `(tenant, school, channel_type)`. `channel.New` từ chối `class`/`study_group` (`ErrType`); seed policy platform **version 2** không còn rule `class`/`study_group` (migration `20261231000002_adr0095_noclass.sql`, `chat-policy.schema.json` v5 sửa tại chỗ vì chỉ seed platform từng mang các giá trị này). Channel `class`/`study_group` còn sót (nếu có) bị soft-delete giữ tin làm bằng chứng; dòng policy cấp `class` bị soft-delete; giá trị cũ trong CHECK bỏ ở WP-R khi 0 dòng. Enum `channel_type` của `chat-envelope`, `chat.message_created`, `chat.message_held` thu hẹp tại chỗ (v1, tương thích ngược). AC giữ mã; F115-AC-08, AC-14, AC-22 sửa chữ/test.

### Session 2026-11-28 (delta server F112-6 — [Confirmed], owner "chốt hết")

- Q: `CHAT_NOT_ALLOWED` khi gửi ngoài `allowed_hours` của channel bền vững có cho biết lúc nào mở lại (câu hỏi mở F112-6 của client F112)? → A: [Confirmed] Có, additive: `reopen_at` = phút đầu tiên mà khung giờ của mọi cấp mở lại và không nằm trong giờ giới nghiêm của học sinh (quét tối đa 7 ngày, không có → vắng), giờ server UTC. REST: trường `reopen_at` của Problem (như `CURFEW_ACTIVE`); WS: `system.command.failed.details.reopen_at` (RFC 3339) — `natscmd.failure` chuyển cả `reopen_at` của `CURFEW_ACTIVE` vào `details` (F115-AC-22 không đổi mã). `CHAT_NOT_ALLOWED` vì policy tắt, không là bạn bè hay chặn không mang `reopen_at` và không nêu lý do. Owner tài liệu: [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#3-policy-resolution). Test: `TestF112ReopenAt`, `TestFailureDetails`, `TestChatPolicyPutLevels` (20:31 → `2026-11-24T00:00:00Z`).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Gửi và nhận tin realtime, bền vững trước khi "Sent" (Priority: P1)

Học sinh trong một channel gõ tin và bấm Gửi; tin hiện "đã gửi" chỉ khi hệ thống đã ghi bền vững; bạn cùng channel thấy tin ngay theo đúng thứ tự.

**Why this priority**: Là đường truyền duy nhất cho mọi tính năng chat sau (F116–F120); sai thứ tự hoặc mất tin phá vỡ toàn bộ chat.

**Independent Test**: Hai client WS là member cùng channel (tạo bằng fixture `OpenChannel`); A gửi 20 tin → A nhận `chat.message.acked` cho từng tin sau JetStream ack, B nhận `chat.message.created` đúng thứ tự `seq`; dừng NATS → A nhận `CHAT_TEMPORARILY_UNAVAILABLE`, B không nhận gì.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F115-AC-01 | KHI member gửi `chat.message.send` (`kind` `text`/`emoji`) hợp lệ, module chat PHẢI chạy giới hạn kỹ thuật rồi pipeline kiểm duyệt, cấp `seq`, publish vào stream bền vững với khóa khử trùng = `message_id`, và chỉ sau khi nhận ack mới trả `chat.message.acked` (`client_message_id`, `message_id`, `channel_id`, `seq`, `moderation_state`) | FR-CHAT-017 · FR-CHAT-008 · [CHAT_SYSTEM §5.2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-025 |
| F115-AC-02 | KHI tin đã được ack với decision `allow`/`unchecked`/`mask`, module chat PHẢI phát tin tới mọi member online (`chat.message.created`, nội dung đã che) và client nhận theo thứ tự `seq`; với `hold_for_review`/`block_and_alert` PHẢI chỉ gửi `chat.message.held` cho người gửi | FR-CHAT-017 · [CHAT_SYSTEM §4.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-004 |
| F115-AC-03 | NẾU stream bền vững lỗi hoặc không ack trong 2 giây, THÌ module chat PHẢI trả `system.command.failed` mã `CHAT_TEMPORARILY_UNAVAILABLE` (REST `503` + `Retry-After`), không phát tin cho bất kỳ ai và không ghi tin | FR-CHAT-017 · ADR-0035 · AGENTS R25 | TC-CHAT-026 |
| F115-AC-04 | KHI gửi lại cùng `client_message_id` (WS) hoặc cùng `Idempotency-Key` (REST) trong thời gian giữ khóa, module chat PHẢI trả lại đúng `message_id` và `seq` cũ, không tạo tin thứ hai ở stream lẫn PostgreSQL | FR-CHAT-008 · AGENTS R10 | TC-CHAT-026 |
| F115-AC-05 | KHI học sinh gọi `sendChannelMessage`, module chat PHẢI dùng cùng use case với lệnh WS (cùng kiểm tra, cùng pipeline, cùng thứ tự ack); NẾU thiếu `Idempotency-Key`, THÌ PHẢI trả `428 IDEMPOTENCY_KEY_REQUIRED` | FR-CHAT-008 · NFR-SAFE-001 | — |
| F115-AC-06 | HỆ THỐNG PHẢI không có đường nào ghi hoặc phát tin chat mà bỏ qua pipeline kiểm duyệt: mọi `kind` đi qua cùng use case `SendMessage`; module chat PHẢI từ chối khởi động khi chưa nối bộ kiểm tra nội dung thật (ngoài môi trường test) | NFR-SAFE-001 · AGENTS R15 · AGENTS R17 | TC-CHAT-003 |
| F115-AC-07 | NẾU `body` vượt `max_text_length` của effective policy hoặc người gửi vượt `rate`/trần `chat_rate_per_user`, THÌ module chat PHẢI từ chối (`VALIDATION_FAILED` / `CHAT_RATE_LIMITED`) trước khi cấp `seq`; giới hạn này luôn chạy kể cả khi công tắc kiểm duyệt tắt | FR-CHAT-002 · AC-MOD-021 · AGENTS R28 | TC-TEN-024 |

---

### User Story 2 — Quyền vào channel theo quan hệ và policy, mặc định từ chối (Priority: P1)

Chỉ member hợp lệ mới gửi/đọc/subscribe được; channel loại nào không có rule ở mọi cấp thì không ai dùng được; quy tắc như nhau cho mọi lớp.

**Why this priority**: Chặn đọc trộm chat giữa trường/tenant là yêu cầu an toàn trẻ em bắt buộc (R8, R15).

**Independent Test**: Học sinh không phải member `chat.channel.join`, `listChannelMessages`, gửi tin → bị từ chối, không lộ sự tồn tại; xóa rule `party` ở cấp platform trong fixture → mọi gửi vào channel `party` bị `CHAT_NOT_ALLOWED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F115-AC-08 | KHI resolve effective policy, module chat PHẢI giao các cấp platform → tenant → school (giá trị chặt hơn thắng; không còn cấp class, không có channel `class`/`study_group` — ADR-0095) và NẾU không có rule cho `channel_type` ở bất kỳ cấp nào, THÌ PHẢI coi là deny (`CHAT_NOT_ALLOWED`) | FR-CHAT-002 · [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-002 |
| F115-AC-09 | HỆ THỐNG PHẢI cho cùng một tin ở cùng loại channel cùng quyết định với học sinh lớp 1–3 và lớp 6–9; mã nguồn policy/pipeline chat PHẢI không đọc `grade_level`/`grade_band` (kiểm bằng test tĩnh) | FR-CHAT-002 · AC-CHAT-003 · AGENTS R15 | TC-CHAT-010 |
| F115-AC-10 | NẾU người gọi không phải member của channel (đoán `channel_id`, subscribe WS, đọc lịch sử, gửi tin), THÌ module chat PHẢI trả `404 NOT_FOUND` (WS `NOT_FOUND`), không lộ channel có tồn tại | FR-CHAT-002 · AC-CHAT-007 | TC-CHAT-008 |
| F115-AC-11 | NẾU học sinh hoặc nhân sự tenant A truy cập channel tenant B (qua API, WS ticket hoặc repository bỏ điều kiện tenant), THÌ module chat PHẢI từ chối ở tầng ứng dụng và RLS PHẢI trả 0 dòng | AC-CHAT-007 · NFR-SEC-006 · AGENTS R8 | TC-CHAT-009 |
| F115-AC-12 | KHI `X-Tenant-Id` (REST) khác tenant của phiên, module chat PHẢI trả `403 TENANT_MISMATCH`; header tenant của lệnh WS chỉ lấy từ gateway (`Ew-Tenant-Id`) | NFR-SEC-006 · ADR-0065 | TC-TEN-011 |
| F115-AC-13 | HỆ THỐNG PHẢI cho mọi route `/chat/*` của feature trả `403 GAME_ACCESS_STUDENT_ONLY` với token nhân sự (`school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin`); nhân sự không là member channel nào | AGENTS R23 · ADR-0072 | TC-AUTH-014 |
| F115-AC-14 | KHI channel có cờ liên tenant hoặc liên trường, module chat PHẢI chỉ trả nickname/avatar của người gửi (`PlayerCard`), không tên thật, trường, khối | AGENTS R16 · [CHAT_SYSTEM §2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | — |
| F115-AC-22 | TRONG KHI thời điểm server nằm trong giới nghiêm hiệu lực của người gửi, module chat PHẢI từ chối mọi lệnh gửi (REST và WS `chat.message.send`) ở mọi channel kể cả `match`/`team`/`party` bằng `403 CURFEW_ACTIVE` (WS: `system.command.failed` cùng mã) trước khi cấp `seq`; channel không bị đóng hay xóa | NFR-SAFE-005 · AC-CHAT-015 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-CHAT-046 |
| F115-AC-23 | TRONG KHI trường của người gửi có `chat_policy_preset = quick_message_emote_only` (hoặc mặc định pilot tương đương), module chat PHẢI chỉ nhận `kind` `quick` và `emoji` (emote/reaction) ở mọi channel của trường kể cả `party`/`match`/`team`, từ chối `text`, `sticker`, `gif`, forward bằng `403 CHAT_KIND_NOT_ALLOWED`, như nhau cho mọi khối; channel gameplay vẫn tự tạo | AC-CHAT-015 · [CHILD_SAFETY §3.5](../../docs/10-security/CHILD_SAFETY.md#35-preset-chỉ-quick-message--emote-adr-0078) · ADR-0078 · AGENTS R15 | TC-CHAT-048 |

---

### User Story 3 — Lưu bền vững, lịch sử và bù tin theo `seq` (Priority: P1)

Mở channel thấy lịch sử; mất mạng hay mất gói fan-out thì tự bù đủ tin; PostgreSQL tạm ngừng vẫn chat được và lịch sử đầy đủ khi DB trở lại.

**Why this priority**: Không mất tin là cam kết ADR-0035; lịch sử là nền cho unread (F117) và bằng chứng (F121).

**Independent Test**: Gửi 500 tin, dừng PostgreSQL 10 phút giữa chừng → sau khi DB trở lại, `chat.messages`/`chat.gameplay_messages` có đủ 500 dòng, không trùng `(channel_id, seq)`; drop gói fan-out → client gọi `chat.channel.sync` nhận đủ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F115-AC-15 | KHI tin nằm trong stream, `chat-persister` PHẢI ghi theo lô vào PostgreSQL idempotent theo `message_id`, cập nhật `last_seq` của channel không lùi, chỉ ack stream sau commit và phát domain event `chat.message_created` (không nội dung) qua outbox | FR-CHAT-017 · AGENTS R25 · [EVENT_DESIGN §4.9](../../docs/07-backend/EVENT_DESIGN.md) | TC-CHAT-025 |
| F115-AC-16 | TRONG KHI PostgreSQL không sẵn sàng, module chat PHẢI vẫn gửi/nhận realtime; khi DB trở lại, persister PHẢI ghi bù đủ, không trùng, đúng `seq` | FR-CHAT-017 · ADR-0035 | TC-CHAT-027 |
| F115-AC-17 | NẾU một sự kiện lỗi quá số lần giao tối đa, THÌ persister PHẢI chuyển vào hàng DLQ, ghi `infra.dead_letters` và phát cảnh báo, không chặn các tin khác | FR-CHAT-017 · [EVENT_DESIGN §7](../../docs/07-backend/EVENT_DESIGN.md) | — |
| F115-AC-18 | KHI member gọi `listChannelMessages` (`before_seq`/`after_seq`/`limit`) hoặc `chat.history.fetch`, module chat PHẢI trả tin sắp theo `seq`, hợp nhất PostgreSQL với bộ đệm tin gần nhất, chỉ nội dung đã che, lọc `tenant_id` và `deleted_at IS NULL`, chỉ tin từ `joined_at` của member | FR-CHAT-008 · [CHAT_SYSTEM §8.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-004 |
| F115-AC-19 | KHI client gửi `chat.channel.sync` hoặc `chat.channel.join` có `since_seq`, module chat PHẢI trả tin thiếu qua `chat.history.loaded` (tối đa `chat_recent_buffer`), nhiều hơn thì báo `has_more` để client tải REST | FR-CHAT-008 · ADR-0035 | TC-CHAT-028 |
| F115-AC-20 | NẾU key cấp `seq` trong Valkey bị mất, THÌ module chat PHẢI khởi tạo lại từ `GREATEST(channels.last_seq, seq lớn nhất trong bộ đệm)` bằng script nguyên tử; ràng buộc unique `(channel_id, seq)` ở PostgreSQL PHẢI chặn trùng | FR-CHAT-017 · [VALKEY_DESIGN §4.9](../../docs/07-backend/VALKEY_DESIGN.md) | TC-CHAT-028 |
| F115-AC-21 | HỆ THỐNG PHẢI không để game client hay portal kết nối NATS/Valkey; mọi tin vào hệ thống qua `/realtime/v1` hoặc `/api/v1/chat/*`, và chỉ `chat-persister` ghi tin vào PostgreSQL | AGENTS R25 · ADR-0035 | — |

### Edge Cases

- NẾU Valkey Pub/Sub mất gói, THÌ client thấy khoảng trống `seq` và gọi `chat.channel.sync` (F115-AC-19); khoảng trống do gửi lỗi (seq đã cấp nhưng publish thất bại) là bình thường, không retry vô hạn.
- NẾU không cấp được `seq` (Valkey lỗi), THÌ trả `CHAT_TEMPORARILY_UNAVAILABLE` (F115-AC-03).
- NẾU member bị gỡ khỏi channel giữa chừng, THÌ lệnh gửi kế tiếp trả `NOT_FOUND`; gateway hủy subscribe `ew:chat:fanout:<channel_id>` của phiên đó.
- NẾU tin `block`, THÌ không cấp `seq`, không publish, không lưu nội dung; client nhận `CHAT_CONTENT_BLOCKED` (quyết định do F116).
- NẾU học sinh bị thu hồi truy cập/khóa (F016) hoặc WS đóng (F090), THÌ lệnh đang chờ không được fan-out sau khi phiên mất.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-002 | Resolve policy 4 cấp, deny-by-default, quyền theo membership/quan hệ, không điều kiện theo khối | [CHAT_SYSTEM §3–§4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-008 | Gửi qua WS (`chat.message.send`) hoặc REST (`sendChannelMessage`) với idempotency; giao realtime qua `/realtime/v1`; lịch sử, bù tin | [CHAT_SYSTEM §5, §8.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-017 | JetStream trước "Sent", Valkey Pub/Sub fan-out, `chat-persister` ghi PostgreSQL, độ tin cậy | [ADR-0035](../../docs/adr/ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md) |
| NFR-SAFE-001 | Một use case gửi duy nhất gọi pipeline; khởi động thất bại nếu thiếu bộ kiểm tra thật | [MODERATION §3](../../docs/03-social-multiplayer/MODERATION.md) |

### Key Entities

- **Channel**: không gian hội thoại có loại (`world`…`direct`), phạm vi (`scope_ref`), tenant, trường, trạng thái mở/đóng, `last_seq`.
- **ChannelMember**: quan hệ học sinh–channel (vai trò, thời điểm vào/rời, con trỏ đọc).
- **Message**: tin có `seq` theo channel, `kind`, nội dung gốc (bằng chứng) và nội dung hiển thị (đã che), trạng thái kiểm duyệt.
- **ChatPolicy / EffectivePolicy**: rule theo cấp platform/tenant/school (ADR-0095) và kết quả giao cho một người trong một channel.
- **ChatEnvelope**: bản ghi tin trong stream bền vững (`message_id`, `seq`, `tenant_id`, nội dung đã kiểm duyệt, quyết định kiểm duyệt).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `chat_max_length` | Trần độ dài text theo loại channel (giá trị hiệu lực lấy từ policy) |
| `chat_rate_per_user` | Trần tổng tin/phút mỗi người trên mọi channel |
| `chat_recent_buffer` | Bộ đệm tin gần nhất và số tin tối đa trả qua `chat.channel.sync` |
| `chat_policy_cache` | Thời gian cache effective policy |
| `idempotency_record_ttl` | Giữ kết quả `sendChannelMessage` theo `Idempotency-Key` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không fan-out tin chưa ack; không ghi PG từ đường realtime; client không nối NATS/Valkey | AGENTS R25 · ADR-0035 |
| Send latency (`acked`) p95 < 300 ms, delivery p95 < 250 ms, persist lag p95 < 500 ms | [CHAT_SYSTEM §5.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · TC-PERF-010 |
| Mọi bảng `chat.*` tenant-scoped có RLS; chỉ soft-delete | AGENTS R8, R21, R27 |
| Không log nội dung chat thô ở INFO; metric không gắn label user/nội dung | AGENTS R14 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 tin tới người nhận mà không có trong stream bền vững trong 10.000 lần gửi có chèn lỗi NATS ngẫu nhiên.
- **SC-002**: Sau 10 phút PostgreSQL ngừng, 100% tin trong khoảng đó có trong lịch sử, 0 bản trùng.
- **SC-003**: 0 lượt đọc/gửi thành công vào channel không phải member trong bộ test duyệt mọi route chat (tenant khác, trường khác, ID đoán).
- **SC-004**: Ở tải TC-PERF-010, 95% tin được xác nhận "đã gửi" trong dưới 300 ms và tới người nhận trong dưới 250 ms.

## Ngoài phạm vi

- Bước kiểm tra nội dung, danh mục, công tắc, sanction, quick message → F116.
- Tạo/đóng channel gameplay, unread count, purge → F117; channel bền vững (world/area/team cố định/DM/club; không còn class/study group — ADR-0095), API ghi chat policy, tìm kiếm, retention ẩn danh hóa → F120.
- Sửa/thu hồi/reply/forward/reaction/pin/mention/typing/receipts/sticker/GIF → F118; UI Godot → F119.
- Report, moderation case, đọc chat của Safety Team → F121; Portal chat policy → F122; ticket WS, định tuyến lệnh → F090.

## Assumptions

- F095 đã dựng skeleton module chat (cmd, cấu hình, outbox relay, projection `ref.tenants`, `ref.schools`, `ref.students`, `ref.player_profiles`; `ref.classes`/`ref.enrollments` không còn dùng — ADR-0095) và bảng `social.blocks`/`social.mutes`; F115 không tạo lại.
- F090 đã định tuyến `chat.*` → NATS request `ew.cmd.chat.<type>` kèm header `Ew-Tenant-Id`, `Ew-User-Id` và subscribe `ew:chat:fanout:<channel_id>` khi reply có `subscribe`.
- Seed platform `chat_policies` (8 loại channel, mặc định CHAT_SYSTEM §3.1) được nạp bằng `src/backend/modules/chat/seed/` [Confirmed — giá trị số].
- Timeout ack 2 s, lô persister (500 sự kiện/50 ms), `max_deliver` 5 chưa có tên trong CONFIG_DEFAULTS; giữ là hằng số cấu hình code, task Polish đề xuất đăng ký [Confirmed].
