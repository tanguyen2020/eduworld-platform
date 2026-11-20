# Feature Specification: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

**Feature Branch**: `117-gameplay-chat-purge` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` · **Mốc**: M4b · **Phụ thuộc**: F101, F105, F116
**Input**: Khi tạo party, vào trận 1v1 hoặc trận đội, module chat tự tạo channel `party`/`match`/`team` và tự thêm người chơi; ba loại channel này luôn bật, không cấp tenant/trường/lớp nào tắt được; đóng read-only khi trận kết thúc/party giải tán; lịch sử, unread trong channel gameplay; job `gameplay-chat-purge` xóa vật lý chat gameplay sau thời hạn lưu (ngoại lệ duy nhất ADR-0033), giữ bằng chứng trước khi xóa. FR nguồn: FR-CHAT-001, FR-CHAT-003, FR-CHAT-009, FR-CHAT-011, FR-PTY-001.

## Clarifications

### Session 2026-11-16

- Q: Event đóng channel `match` là `match.ended` (CHAT_SYSTEM §4) hay `match.completed`? → A: `match.completed` và `match.aborted` theo EVENT_DESIGN §4.8 và REALTIME_ARCHITECTURE §6 (RB-507, CHAT_SYSTEM §4 đã sửa).
- Q: "Tạo channel cùng transaction với trận/party" (CHAT_SYSTEM §4.1) khi hai module khác database? → A: module competition ghi outbox cùng transaction (đảm bảo event được phát); module chat tạo channel khi tiêu thụ event, idempotent. `channel_id` là UUIDv5 tất định từ loại + `party_id`/`match_id`/`team_id` (hàm thuần trong `src/backend/pkg/`) để competition trả `chat_channel_id` ngay mà không gọi đồng bộ (research R1).
- Q: Purge cần kiểm report/case/sanction, nhưng report/case do F121 (M5) làm sau F117 (M4b)? → A: Job purge **fail-closed**: chưa có module moderation thì không xóa gì, chỉ cảnh báo; dữ liệu giữ lâu hơn hạn tạm thời là chấp nhận được, mất bằng chứng thì không (research R3) — [Confirmed, RB-001].
- Q: Party chat văn bản tự do P5 (FR-PTY-001) hay P4 (FR-CHAT-009)? → A: Theo FR-CHAT-009 (Confirmed, ADR-0020 mục 6): đủ text/emoji/quick ngay từ P4; ghi mâu thuẫn PRD.
- Q: Reaction trong FR-CHAT-003? → A: Reaction (≤ `chat_reactions_per_user`) làm ở F118; F117 làm text/emoji/quick, lịch sử, unread, badge.

### Session 2026-11-17

- Q: Preset "chỉ quick message + emote" thu hẹp loại tin của channel always-on (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Chat có ngay khi vào party hoặc trận (Priority: P1)

Học sinh tạo party hoặc được ghép trận: khung chat đã sẵn, không có bước "bật chat"; người vào party sau tự là thành viên; trận đội có kênh riêng cho đội và kênh chung cả trận.

**Why this priority**: ADR-0020 mục 6 bắt buộc chat luôn có khi chơi cùng nhau (AC-CHAT-008).

**Independent Test**: Phát `party.created` (3 thành viên) + `party.member_joined` + `match.started` (2 đội × 2) lên NATS test → channel và membership đúng; thành viên đội A gọi lịch sử `team` đội B → `404`; `party.disbanded` → gửi tin trả `CHAT_CHANNEL_CLOSED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F117-AC-01 | KHI nhận `party.created`, module chat PHẢI tạo channel `party` (id tất định theo `party_id`), thêm mọi thành viên; KHI nhận `party.member_joined` PHẢI thêm thành viên (đọc lịch sử từ lúc vào); KHI nhận `party.member_left`/`party.member_kicked` PHẢI gỡ quyền gửi và đọc tin mới | FR-CHAT-009 · FR-PTY-001 · AC-CHAT-008 | TC-CHAT-017 |
| F117-AC-02 | KHI nhận `match.started`, module chat PHẢI tạo một channel `match` gồm mọi người chơi của trận; với trận đội PHẢI tạo thêm một channel `team` cho từng đội chỉ gồm thành viên đội đó, và thành viên đội khác PHẢI nhận `404` khi truy cập channel `team` không phải của mình | FR-CHAT-009 · FR-CHAT-001 · AC-CHAT-008 | TC-CHAT-017 |
| F117-AC-03 | HỆ THỐNG PHẢI xử lý event party/trận idempotent và chịu được thứ tự lệch (thành viên đến trước `party.created`, event giao hai lần) mà không tạo channel/thành viên trùng | FR-CHAT-009 · [EVENT_DESIGN §7](../../docs/07-backend/EVENT_DESIGN.md) | TC-SVC-003 |
| F117-AC-04 | KHI tạo hoặc đổi membership channel gameplay, module chat PHẢI phát `chat.channel_opened` (`channel_id`, `channel_type`, `scope_ref`, `member_ids`) để realtime tự đăng ký nhận tin cho thành viên đang online | FR-CHAT-009 · [EVENT_DESIGN §4.9](../../docs/07-backend/EVENT_DESIGN.md) | TC-CHAT-017 |
| F117-AC-05 | KHI nhận `match.completed`/`match.aborted`/`party.disbanded`, module chat PHẢI đóng channel (read-only); NẾU gửi tin vào channel đã đóng, THÌ PHẢI trả `409 CHAT_CHANNEL_CLOSED`, còn lịch sử PHẢI xem được đến khi purge | FR-CHAT-009 · [CHAT_SYSTEM §4.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-020 |
| F117-AC-06 | KHI người chơi reconnect hoặc đổi thiết bị giữa trận, module chat PHẢI giữ nguyên membership và trả các tin bỏ lỡ qua `chat.channel.sync` | FR-CHAT-009 · ADR-0031 | TC-CHAT-020 |
| F117-AC-07 | NẾU event mang `tenant_id` khác tenant của người chơi trong projection, hoặc người tham gia không phải học sinh, THÌ module chat PHẢI bỏ qua người đó, ghi log/metric (không PII) và không bao giờ thêm nhân sự vào channel | AGENTS R8 · ADR-0072 · AC-CHAT-007 | TC-TEN-012 |

---

### User Story 2 — Không cấp nào tắt được chat gameplay (Priority: P1)

Trường hay lớp có tắt các kênh khác thì chat trong trận/party vẫn hoạt động; chỉ người bị Safety Team tạm khóa chat mới không gửi được, và mỗi học sinh tự tắt tiếng/chặn người khác được.

**Why this priority**: Quy tắc always-on (R15, ADR-0020 mục 6) là bất biến an toàn và trải nghiệm.

**Independent Test**: Seed rule cấp trường đặt `enabled:false` cho `party` → ghi bị từ chối `CHAT_POLICY_ALWAYS_ON`; effective policy của `party` vẫn `enabled`, đủ `text/emoji/quick/sticker/gif`; học sinh bị `mute` vẫn trong channel, nhận tin, gửi bị `CHAT_MUTED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F117-AC-08 | KHI resolve effective policy cho `match`/`team`/`party`, module chat PHẢI luôn trả `enabled = true`, `allowed_kinds` gồm `text`, `emoji`, `quick`, `sticker`, `gif` (ngoại lệ duy nhất: preset trường `quick_message_emote_only`, F117-AC-24), mọi tính năng "luôn" của CHAT_SYSTEM §7.1 bật, không `allowed_hours`; cấp dưới chỉ siết được `rate`, `slow_mode_seconds` (≤ 10 s) và `validation.strictness` | FR-CHAT-009 · AC-CHAT-009 · AGENTS R15 | TC-CHAT-018 |
| F117-AC-09 | NẾU một rule cấp tenant/school/class cho `match`/`team`/`party` có `enabled:false`, thiếu loại tin bắt buộc, có `allowed_hours` hoặc `slow_mode_seconds > 10`, THÌ mọi đường ghi chat policy PHẢI từ chối với `422 CHAT_POLICY_ALWAYS_ON` | FR-CHAT-009 · AC-CHAT-009 | TC-CHAT-018 |
| F117-AC-10 | HỆ THỐNG PHẢI không có cờ, feature flag, cài đặt tenant/trường/lớp hay biến môi trường nào tắt được chat `match`/`team`/`party` (kiểm bằng test tĩnh và test cấu hình) | FR-CHAT-009 · AGENTS R15 | TC-CHAT-018 |
| F117-AC-11 | TRONG KHI học sinh có sanction `mute`, module chat PHẢI giữ học sinh trong channel gameplay và vẫn giao tin cho học sinh đó, chỉ từ chối gửi (`CHAT_MUTED`); người khác chat bình thường | FR-CHAT-009 · [CHAT_SYSTEM §4.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-019 |
| F117-AC-12 | KHI A chặn B, module chat PHẢI ẩn tin hai chiều giữa A và B trong channel gameplay chung (fan-out và lịch sử) mà không đưa ai ra khỏi trận/party; KHI A tắt tiếng B, PHẢI chỉ ẩn tin của B với A | FR-CHAT-009 · AC-CHAT-005 | TC-SOC-003 |
| F117-AC-24 | KHI resolve effective policy cho `match`/`team`/`party` của học sinh thuộc trường có `chat_policy_preset = quick_message_emote_only` (hoặc mặc định pilot tương đương), module chat PHẢI vẫn trả `enabled = true` và tự tạo/join channel, nhưng `allowed_kinds` PHẢI chỉ còn `quick` + `emoji`; đây PHẢI là đường duy nhất thu hẹp loại tin của channel always-on — rule tenant/school/class bỏ loại tin vẫn bị `422 CHAT_POLICY_ALWAYS_ON` (F117-AC-09) | FR-CHAT-009 · AC-CHAT-015 · [CHILD_SAFETY §3.5](../../docs/10-security/CHILD_SAFETY.md#35-preset-chỉ-quick-message--emote-adr-0078) · ADR-0078 · ADR-0020 | TC-CHAT-048 |

---

### User Story 3 — Lịch sử, chưa đọc và thời hạn lưu trong trận/party (Priority: P2)

Học sinh thấy số tin chưa đọc trên nút chat, kéo xem lại tin trong trận, và biết tin trong trận được giữ bao nhiêu ngày.

**Why this priority**: FR-CHAT-003 ở P4 cho channel gameplay; minh bạch thời hạn lưu là yêu cầu FR-CHAT-011.

**Independent Test**: Bình đọc tới `seq = 10` trên thiết bị 1 → thiết bị 2 nhận `chat.unread.updated` với số chưa đọc giảm; `listMyChannels` trả channel trận đã đóng kèm `status = closed`, `purge_after`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F117-AC-13 | KHI thành viên gửi `chat.receipt.read` (`channel_id`, `seq`), module chat PHẢI gộp con trỏ (≤ 1 lần/2 giây/thành viên), ghi `last_read_seq` không lùi qua đường bền vững và phát `chat.unread.updated` tới mọi thiết bị của người đó | FR-CHAT-003 · [CHAT_SYSTEM §8.2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-039 |
| F117-AC-14 | KHI học sinh gọi `listMyChannels`, module chat PHẢI trả channel gameplay đang mở và đã đóng chưa purge của mình, với `unread_count = last_seq − last_read_seq`, `status`, `purge_after` để client hiển thị thời hạn lưu | FR-CHAT-003 · FR-CHAT-011 | TC-CHAT-004 |

---

### User Story 4 — Purge chat gameplay đúng hạn, giữ bằng chứng (Priority: P1)

Tin trong trận/party tự xóa hẳn sau thời hạn lưu để không tích dữ liệu trẻ em; tin đã bị báo cáo vẫn còn làm bằng chứng; channel có vụ việc đang mở chưa bị xóa.

**Why this priority**: ADR-0033 là ngoại lệ hard-delete duy nhất (R21); sai một dòng là mất bằng chứng hoặc xóa nhầm dữ liệu bền vững.

**Independent Test**: Channel `match` đóng ngày D (đồng hồ giả lập), `gameplay_chat_retention_days = N` → chạy job ngày D+N−1: còn; D+N: đã xóa, API `404`; tin có report được sao sang bằng chứng; channel có case mở còn nguyên; channel `class`/`direct` không mất dòng nào.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F117-AC-15 | KHI đóng channel gameplay, module chat PHẢI đặt `closed_at` và `purge_after = closed_at + gameplay_chat_retention_days` hiệu lực của tenant, phát `chat.channel_closed`; NẾU giá trị hiệu lực nhỏ hơn mức tối thiểu 3 ngày, THÌ PHẢI dùng mức tối thiểu và cảnh báo | FR-CHAT-011 · ADR-0033 | TC-CHAT-024 |
| F117-AC-16 | KHI job `gameplay-chat-purge` chạy, module chat PHẢI với mỗi channel `purge_after < now()` không bị hoãn: rollup thống kê ngày → sao tin và revision thuộc report/case/sanction sang bằng chứng → xóa theo lô (1.000 dòng) tin, revision, reaction, pin, thành viên, channel → đánh dấu partition → `DROP` partition ngày chỉ khi mọi channel có tin trong ngày đó đã purge | FR-CHAT-011 · ADR-0033 · [DATABASE_DESIGN §3.8b](../../docs/07-backend/DATABASE_DESIGN.md) | TC-CHAT-021 |
| F117-AC-17 | KHI tin trong channel gameplay bị tham chiếu bởi report, moderation case hoặc sanction, job PHẢI sao nội dung gốc, người gửi, thời điểm, `channel_type`, `match_id`/`party_id`, `catalog_version` và revision sang bằng chứng trước khi xóa, và bằng chứng PHẢI còn xem được sau purge | FR-CHAT-011 · AC-CHAT-010 | TC-CHAT-022 |
| F117-AC-18 | TRONG KHI channel còn report chưa phân loại hoặc case `open`/`in_review`/`escalated`/`reopened`, job PHẢI hoãn purge, ghi lý do hoãn; case đóng thì purge ở lần chạy kế tiếp | FR-CHAT-011 · AC-CHAT-010 | TC-CHAT-023 |
| F117-AC-19 | NẾU module moderation chưa sẵn sàng (chưa có nguồn report/case) hoặc sao bằng chứng của một channel thất bại, THÌ job PHẢI không xóa channel đó (fail-closed), tăng metric và cảnh báo vận hành | FR-CHAT-011 · AGENTS R21 | TC-CHAT-022 |
| F117-AC-20 | HỆ THỐNG PHẢI không bao giờ xóa vật lý dữ liệu của channel `class`, `study_group`, `team` cố định, `direct`; role `chat_purger` PHẢI chỉ có `DELETE` trên danh sách bảng gameplay và các role khác PHẢI không có `DELETE`/`TRUNCATE` | FR-CHAT-011 · AC-CHAT-010 · AGENTS R21 | TC-CHAT-024 |
| F117-AC-21 | KHI tin `world`/`area` quá `ephemeral_chat_retention`, job PHẢI sao tin thuộc report/case sang bằng chứng rồi `DETACH` + `DROP` partition ngày của bảng tin ephemeral | FR-CHAT-011 · ADR-0033 | TC-CHAT-021 |
| F117-AC-22 | KHI một lần chạy kết thúc, job PHẢI ghi bản ghi lượt chạy (số channel, số tin, partition đã drop, số hoãn, số bằng chứng) và audit tổng hợp không nội dung, phát `chat.gameplay_purged`, tăng `eduworld_chat_purge_total{tenant,kind}`; tồn đọng quá 2 ngày PHẢI cảnh báo | FR-CHAT-011 · ADR-0033 | TC-CHAT-021 |
| F117-AC-23 | HỆ THỐNG PHẢI không có API hay use case nào ngoài job `gameplay-chat-purge` thực hiện xóa vật lý; sau purge, mọi API chat với channel đó PHẢI trả `404` | FR-CHAT-011 · AGENTS R21 | TC-CHAT-021 |

### Edge Cases

- NẾU `match.aborted` đến khi channel chưa có tin, THÌ vẫn đóng channel và purge bình thường (không có partition để drop).
- NẾU party sống lâu nhiều ngày, THÌ partition các ngày đó giữ đến khi party purge (F117-AC-16).
- NẾU `party.member_joined` đến sau `party.disbanded` (lệch thứ tự), THÌ bỏ qua thêm thành viên vào channel đã đóng.
- NẾU job chạy song song trên hai worker, THÌ lock `ew:lock:job:gameplay-chat-purge` bảo đảm một instance; xóa lô idempotent.
- NẾU Safety Team mở case sau khi channel đã purge, THÌ chỉ thấy bằng chứng đã lưu (MODERATION §10).
- NẾU trận liên tenant do platform tổ chức (MATCHMAKING H3 `platform`), THÌ channel dùng `tenant_ids` [Confirmed]; triển khai cùng F107.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-001 | Loại channel `match`, `party`, `team` theo trận (các loại còn lại → F120) | [CHAT_SYSTEM §2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-003 | Channel gameplay: text, emoji, quick message, lịch sử, unread, badge in-app (reaction → F118) | [CHAT_SYSTEM §8](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-009 | Tự tạo/tự join, always-on, không tắt bằng cài đặt, chỉ mute/block/sanction ngăn gửi | [CHAT_SYSTEM §4.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · ADR-0020 |
| FR-CHAT-011 | Purge gameplay + World/Area, giữ bằng chứng, hoãn khi case mở, thông báo thời hạn (dữ liệu `purge_after`) | [CHAT_SYSTEM §6.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · ADR-0033 |
| FR-PTY-001 | Party Chat (channel `party` theo vòng đời party) | [PARTY_SYSTEM](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |

### Key Entities

- **GameplayChannel**: channel `match`/`party`/`team` gắn một trận/party, vòng đời Open → Closed → Purged.
- **PartyProjection**: bản sao party và thành viên từ module competition.
- **ModerationEvidence**: bản sao bất biến của tin bị tham chiếu bởi report/case/sanction.
- **PurgeRun**: bản ghi tổng hợp mỗi lượt purge (không nội dung).
- **GameplayDailyStats**: thống kê tổng hợp trước khi purge.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `gameplay_chat_retention_days` | Hạn purge channel gameplay từ `closed_at` (tối thiểu 3) |
| `ephemeral_chat_retention` | Hạn drop partition World/Area |
| `evidence_context` | Số tin ngữ cảnh khi sao bằng chứng |
| `chat_policy_cache` | Cache effective policy (always-on áp sau cache) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Purge chỉ bằng job worker, role DB `chat_purger`, không API | AGENTS R21 · ADR-0033 · ADR-0070 |
| Không cờ tắt chat `match`/`team`/`party` ở mọi cấp | AGENTS R15 · ADR-0020 |
| Không gọi đồng bộ module competition; phản ứng bằng event | AGENTS R29 · ADR-0042 |
| Audit purge không nội dung; metric không label user | AGENTS R14 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 99% người chơi gửi được tin trong channel trận/party trong ≤ 2 giây sau khi trận bắt đầu hoặc party được tạo.
- **SC-002**: 0 dòng bị xóa ngoài bảng gameplay/ephemeral trong toàn bộ test purge; 0 tin bị tham chiếu bởi report/case mất bằng chứng.
- **SC-003**: Channel hết hạn được purge trong ≤ 2 giờ sau `purge_after` (job hằng giờ) khi không bị hoãn.
- **SC-004**: 0 cấu hình chính sách nào làm chat trận/party ngừng hoạt động trong bộ test always-on.

## Ngoài phạm vi

- Report, moderation case, xem bằng chứng của Safety Team, `async_analysis` → F121; API cài đặt `gameplay_chat_retention_days` (identity, `putChatRetentionSettings`) → F015/F057.
- API ghi chat policy và Portal chính sách trường → F120/F122 (dùng validator always-on của feature này).
- Reaction, sticker/GIF, receipts delivered/từng người, mention → F118; UI Godot (panel, tab Đội/Cả trận, thông báo thời hạn) → F119.
- Ghi tin `world`/`area` (ephemeral) → F120; F117 chỉ purge bảng đó.
- Phát `chat_channel_id` trong payload party/trận phía competition → F101, F105.

## Assumptions

- module competition (F101/F105) bổ sung payload `match.started` (`participants[]` gồm `student_id`, `team_id?`, `school_id`) và dùng cùng hàm id tất định để trả `chat_channel_id` (contract delta ở contracts §6).
- `gameplay_chat_retention_days` hiệu lực đọc từ `ref.tenants.settings_subset` (cập nhật từ event identity; dự phòng facade `identity` `getTenant`).
- Lịch chạy job hằng giờ, lô 1.000 dòng, ngưỡng cảnh báo tồn đọng 2 ngày lấy từ DATABASE_DESIGN/ADR-0033 [Confirmed]; chưa có tên trong CONFIG_DEFAULTS.
- Bảng `chat.reports`, `chat.moderation_cases` do F121 tạo; trước đó job ở chế độ fail-closed (F117-AC-19).
