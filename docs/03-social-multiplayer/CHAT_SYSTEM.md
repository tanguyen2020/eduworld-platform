# Chat System

> Trạng thái: Draft · Phiên bản: 0.6 · Owner: Multiplayer Engineer + Identity & Security Engineer · Cập nhật: 2026-11-24 (ADR-0101: bỏ Windows, client chỉ Web/Android/iOS) · 2026-11-21 (F115: `chat.sync` → `chat.channel.sync`, facts đường gửi khi PostgreSQL ngừng §5.4); 2026-11-17 (ADR-0078: PII/grooming luôn chạy, giờ giới nghiêm platform, preset quick message + emote, tenant `common`; 2026-11-16: channel `study_group` theo event module learning — ADR-0084)
> Liên quan: [ADR-0020](../adr/ADR-0020-chat-thong-nhat-cho-moi-lop-moi-tin-nhan-phai-qua-kiem-tra-h.md) · [ADR-0030](../adr/ADR-0030-chinh-sach-tu-nhay-cam-trong-chat-danh-muc-quan-ly-duoc-va-c.md) · [ADR-0078](../adr/ADR-0078-mac-dinh-an-toan-cho-tre-nho.md) · [ADR-0033](../adr/ADR-0033-purge-nhom-chat-gameplay-ngoai-le-co-kiem-soat-cua-adr-0028.md) · [ADR-0034](../adr/ADR-0034-bo-tinh-nang-chat-day-du.md) · [ADR-0035](../adr/ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md) · [MODERATION](MODERATION.md) · [SOCIAL_SYSTEM](SOCIAL_SYSTEM.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) · [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) · [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml)

> [!IMPORTANT]
> **Bỏ khái niệm lớp — [ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) (owner chốt 2026-11-21).** Không có channel `class`, `study_group` và **không** tạo kênh theo khối (đã gỡ khỏi bảng channel §2, bảng mặc định, bảng membership); chat policy còn 3 cấp `platform → tenant → school` (§3). Các nhắc lớp/nhóm học tập còn lại (pin, receipts, block, `notification_level`, bảng tính năng, purge, AC-CHS-002/027/040) và code/contract (`ChannelType`, `chat-policy.schema.json`, AsyncAPI, seed policy platform version 2) đã sửa ở WP-C1 (2026-11-21) cùng [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) (R15) — [CODE_PLAN §4.2](../12-project/CODE_PLAN_CMS_LMS_FIRST.md#42-kế-hoạch-bỏ-khái-niệm-lớp-adr-0095); khi mâu thuẫn, ADR-0095 thắng.

## 1. Phạm vi & nguyên tắc

Tài liệu **duy nhất** về chat: channel, policy, tính năng tin nhắn, truyền tải, lưu trữ/purge và **trải nghiệm chat của học sinh** (§4.3). Kiểm duyệt nội dung (pipeline, danh mục từ nhạy cảm, công tắc, report, sanction) thuộc [MODERATION](MODERATION.md); quan hệ bạn bè/block/club thuộc [SOCIAL_SYSTEM](SOCIAL_SYSTEM.md).

Chat là **domain riêng** trong **module chat** (ADR-0074, thay ADR-0036), database `eduworld_chat`, schema `chat` **[Confirmed]**. Lệnh gửi tin chạy trong `core-api` (nhận từ `realtime` qua NATS `ew.cmd.chat.*`); mọi consumer và job nền của chat (`chat-persister`, `chat-notifier`, `chat-moderation-async`, `chat-stats`, `gameplay-chat-purge`, `chat-partition-maintainer`) chạy trong Swarm service `worker-chat` (`src/backend/cmd/worker --profile=chat`), scale riêng khi chat tăng đột biến ([DEPLOYMENT §3.1](../11-operations/DEPLOYMENT.md#31-binary-và-swarm-service-backend-adr-0074)).

1. **Một bộ quy tắc cho mọi lớp 1–9** (ADR-0020, Accepted): `ChatPolicy` **không** có điều kiện theo `grade_level`/`grade_band`. Mọi học sinh có cùng loại channel, cùng loại tin nhắn (`text`, `emoji`, `quick`, `sticker`, `gif`, reaction) và cùng bộ tính năng (§7) và cùng rate limit **[Confirmed]**.
2. **Mọi tin nhắn phải hợp lệ trước khi phát**: tất cả nội dung mới (text, **bản sửa**, **tin forward**, đoạn **quote**, quick message có tham số, tên nhóm/channel do người dùng đặt) qua pipeline kiểm tra hợp lệ đồng bộ phía server ([MODERATION](MODERATION.md) §3) → `allow | mask | block | hold_for_review | block_and_alert`. Chỉ `allow`/`mask` được phát; `block` không lưu nội dung **[Confirmed]**. Từ nhạy cảm so khớp theo **danh mục quản lý trong CMS** ([MODERATION](MODERATION.md) §11). Pipeline **bật mặc định**; System Admin có thể tắt từng bước hoặc toàn bộ theo platform/tenant ([MODERATION](MODERATION.md) §12, ADR-0030). Khi tắt vẫn giữ độ dài tối đa, rate limit, quyền channel, sanction, block/mute/report **[Confirmed]**.
3. **Không mở tự do vô điều kiện**: quyền gửi vào một channel xác định theo **quan hệ** (thành viên trường, instance, nhóm, bạn bè) và `ChatPolicy`; không có rule = deny **[Confirmed]**.
4. **Tenant isolation**: tin nhắn chỉ đọc được bởi member channel; Platform Safety Team (`safety_moderator`, ADR-0072) chỉ đọc tin trong ngữ cảnh một moderation case, có audit ([MODERATION §6.1](MODERATION.md#61-vai-trò-và-phạm-vi)); không ai đọc toàn bộ chat của mọi trường **[Confirmed]**.
5. **Data minimization**: chỉ lưu những gì cần cho lịch sử + an toàn, có retention ([DATA_PRIVACY](../10-security/DATA_PRIVACY.md)) **[Confirmed]**.
6. Voice chat: **không làm trong Release 1.0** ([ADR-0070](../adr/ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)); §11 chỉ giữ điểm mở rộng ở mức thiết kế **[Confirmed]**.
7. **Chat gameplay luôn có** (ADR-0020 mục 6): khi vào trận **1v1**, **chơi theo đội** hoặc **tạo nhóm chơi (party)**, hệ thống tự tạo channel `match`/`team`/`party` và tự cho người chơi tham gia (§4.1). Ba loại channel này là **always-on**: cài đặt tenant/trường **không tắt được**, không thu hẹp loại tin, không áp khung giờ. Chỉ mute/block của chính người chơi hoặc sanction của Safety Team mới ngăn được một người gửi. Kiểm tra nội dung vẫn theo công tắc ADR-0030 **[Confirmed]**.

## 2. Channel types

| `channel_type` | Phạm vi (`scope_ref`) | Tạo bởi | Membership | Nội dung cho phép (mọi lớp) | Retention [Confirmed] |
|---|---|---|---|---|---|
| `world` | School (mặc định); toàn tenant nếu tenant `unit` bật mở rộng | Hệ thống | Ngầm định — người online cùng trường (hoặc cùng tenant) | `text`, `emoji`, `quick`, `sticker`, `gif`, reaction — qua kiểm tra hợp lệ | Ephemeral, **purge sau 24 h** (ADR-0033) |
| `area` | `room_id` (hub instance) | Hệ thống khi tạo instance | Ngầm định — người trong instance | `text`, `emoji`, `quick`, `sticker`, `gif`, reaction | Ephemeral, **purge sau 24 h** (ADR-0033) |
| `match` **(always-on)** | `match_id` | Tự động khi trận 1v1 / team battle / trận tournament bắt đầu (`match.started`) | Mọi người chơi của trận (cả hai đội) | `text`, `emoji`, `quick`, `sticker`, `gif`, reaction | **Purge** sau `gameplay_chat_retention_days` (mặc định 7, tối thiểu 3) kể từ `closed_at` (ADR-0033) |
| `party` **(always-on; văn bản tự do từ P4 — FR-CHAT-009)** | `party_id` | Tự động khi module chat tiêu thụ `party.created` | Member party | `text`, `emoji`, `quick`, `sticker`, `gif`, reaction | **Purge** sau `gameplay_chat_retention_days` kể từ khi party giải tán (ADR-0033) |
| `team` **(always-on)** | `team_id` / đội trong trận | Tự động khi trận đội bắt đầu (mỗi đội một channel) hoặc khi tạo club — team cố định là channel của club ([SOCIAL_SYSTEM §6](SOCIAL_SYSTEM.md#6-club--guild-nhóm-bền-vững--adr-0050-confirmed)) | Member đội / thành viên club | `text`, `emoji`, `quick`, `sticker`, `gif`, reaction | Team theo trận: **purge** như `match` (ADR-0033). Team cố định (club): `persistent_chat_retention_days`, không purge |
| `direct` | cặp `(user_a, user_b)` | Khi gửi tin đầu tiên | 2 người **đã là bạn bè** và không block nhau | `text`, `emoji`, `quick`, `sticker`, `gif`, reaction | `persistent_chat_retention_days` |

- Người ngoài trường (World mở rộng toàn tenant, team liên trường/liên tenant) chỉ thấy nickname và avatar, không thấy tên thật/trường/lớp.
- Channel bền vững (`team` cố định/club, `direct`) lưu theo `persistent_chat_retention_days` (180 ngày, OQ-09) **[Confirmed — Technical Architect 2026-11-16]** rồi ẩn danh hóa (§6); tin bị report giữ theo case.
- Nhân sự (portal) không là thành viên channel chat nào và không có DM với học sinh; chat chỉ giữa học sinh với nhau (ADR-0031, ADR-0072).

## 3. Policy resolution

`ChatPolicy` có 3 cấp: `platform → tenant → school` (cấp `class` bỏ theo [ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md)). Cấp `school` do `school_admin` đặt (ADR-0072). Cấp dưới **chỉ được siết chặt hơn** cấp trên (ADR-0020), trừ khi cấp trên đặt `allow_override_relax=true` cho một rule cụ thể. Rule **không có** trường `grade_band`/`grade_level`; schema từ chối rule chứa điều kiện theo lớp.

Cấu trúc rule (JSON Schema ở [`specs/schemas/chat-policy.schema.json`](../../specs/schemas/chat-policy.schema.json)):

```json
{
  "channel_type": "party",
  "enabled": true,
  "allowed_kinds": ["text", "emoji", "quick", "sticker", "gif"],
  "reactions": true,
  "features": { "edit": true, "recall": true, "reply": true, "quote": true, "forward": true, "sticker": true, "gif": true, "reaction": true, "mention": true, "pin": true, "typing": true, "receipts": true, "search": true },
  "max_text_length": 200,
  "rate": { "messages_per_minute": 12, "burst": 3 },
  "slow_mode_seconds": 0,
  "allowed_hours": [{ "days": "mon-sun", "from": "06:30", "to": "21:30" }],
  "validation": {
    "strictness": "strict",
    "unknown_link": "block",
    "pii": "block",
    "profanity": "mask",
    "bullying_suspected": "hold_for_review"
  },
  "sensitive_term_policy": {
    "categories": ["profanity", "insult", "violence", "sexual", "drugs", "self_harm", "grooming_scam"],
    "action_overrides": { "insult": "block" }
  }
}
```

Thuật toán `EffectivePolicy(user, channel)`:

```mermaid
flowchart TD
  A[platform rule cho channel_type] --> B[∩ tenant rule]
  B --> C[∩ school rule]
  C --> E["∩ sanction của user: mute, restrict..."]
  E --> F["∩ quan hệ: member channel / bạn bè cho direct / cùng trường cho world"]
  F --> G[Effective policy — cache Valkey chat_policy_cache 5 phút, khóa cache chứa thế hệ policy của platform/tenant]
```

- Phép `∩` = lấy giá trị chặt hơn: `allowed_kinds` giao nhau, số lượng lấy min, giờ lấy giao, boolean `enabled` lấy AND, `strictness` lấy mức cao hơn.
- **Ngoại lệ always-on** (`match`, `team`, `party`): `enabled` luôn `true`, `allowed_kinds` luôn gồm `text`, `emoji`, `quick`, `sticker`, `gif`, mọi `features` bắt buộc ở §7.1 luôn `true`, không có `allowed_hours` ở mọi cấp. Cấp dưới chỉ được siết `rate`, `slow_mode_seconds` (tối đa 10 s) và `validation.strictness`. Schema v5 từ chối rule vi phạm (`enabled:false`, thiếu loại tin, có `allowed_hours`, slow mode > 10 s) và API trả `CHAT_POLICY_ALWAYS_ON`.
- Effective policy giống nhau cho mọi thành viên của cùng channel trong cùng trường/lớp: **không** có quy tắc "grade band thấp nhất" hay ngoại lệ theo khối.
- Không có rule ở bất kỳ cấp nào = **deny**.
- Ghi một cấp (F120): `putChatPolicy` (`tenant`: `tenant.settings.write`; `school`: `school.settings.write` trong trường đó), `putPlatformChatPolicy` (System Admin, gateway nội bộ) — kiểm always-on → schema v5 (không điều kiện theo khối) → chỉ siết chặt so với các cấp trên và `allowed_hours` nằm ngoài giờ giới nghiêm hiệu lực; version + `If-Match`, audit before/after, `chat_policy.updated`. Sau commit tăng bộ đếm thế hệ Valkey (`ew:chat:policygen:platform|<tenant>`): mọi instance dùng khóa cache mới ngay. `allowed_hours` của các cấp được giao nhau và kiểm khi gửi ở channel bền vững (`world`, `area`, `direct`); ngoài khung → `CHAT_NOT_ALLOWED` (giờ giới nghiêm kiểm trước → `CURFEW_ACTIVE`).
- **Công tắc kiểm duyệt** (`moderation_switches`: `enabled_all`, `steps.{sensitive_terms,spam,bullying,async_analysis}`, `expires_at` bắt buộc ≤ 24 giờ) nằm ở cấp platform và tenant, không nằm trong rule channel; chỉ System Admin thay đổi, tắt cần System Admin thứ hai duyệt ([MODERATION](MODERATION.md) §12). Bước PII và phát hiện grooming **luôn chạy**, không có công tắc ([ADR-0078](../adr/ADR-0078-mac-dinh-an-toan-cho-tre-nho.md)). `validation.*` và `sensitive_term_policy` chỉ có hiệu lực với các bước đang bật (`validation.pii` luôn hiệu lực).
- **Giờ giới nghiêm platform** (`platform_curfew`, [CHILD_SAFETY §4.1](../10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078)) không phải rule channel: module chat từ chối mọi lệnh gửi trong giờ giới nghiêm ở mọi channel, kể cả gameplay (khi đó game session cũng đã kết thúc nên không có trận/party đang chạy). Điều này không làm trái ngoại lệ always-on vì channel vẫn tồn tại và tự tạo khi được chơi [Confirmed — Technical Architect 2026-11-17].
- **Preset cấp trường** `chat_policy_preset = quick_message_emote_only` ([CHILD_SAFETY §3.5](../10-security/CHILD_SAFETY.md#35-preset-chỉ-quick-message--emote-adr-0078)): `allowed_kinds` của mọi channel trong trường chỉ còn `quick` + `emoji` (emote/reaction). Đây là **ngoại lệ duy nhất** được phép thu hẹp `allowed_kinds` của `match`/`team`/`party`; channel vẫn always-on (`enabled` luôn `true`), preset áp cho cả trường, không theo khối (R15) [Confirmed — Technical Architect 2026-11-17].
- **Tenant `common`** ([CHILD_SAFETY §3.6](../10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078)): rule mặc định của tenant tắt `world` và `direct`, `area` chỉ `quick` + `emoji`; chỉ System Admin bật lại [Confirmed — Technical Architect 2026-11-17]. Triển khai (F120): một cấp dựng sẵn giữa platform và tenant, gỡ từng phần bằng `putCommonTenantSocialOverride` (bảng `chat.common_tenant_social_overrides`, `reason`, audit).

### 3.1. Mặc định platform [Confirmed — giá trị số; cấu trúc Confirmed theo ADR-0020]

Một bảng duy nhất theo channel, áp dụng cho mọi học sinh lớp 1–9:

| channel_type | enabled | allowed_kinds | max_text_length | rate (msg/phút, burst) | slow mode | validation strictness |
|---|---|---|---|---|---|---|
| `world` | bật (phạm vi trường) | text, emoji, quick, sticker, gif | 120 | 6, 2 | 5 s | `strict` |
| `area` | bật | text, emoji, quick, sticker, gif | 120 | 10, 3 | 0 | `strict` |
| `match` | **luôn bật** (always-on) | text, emoji, quick, sticker, gif | 200 | 12, 3 | 0 | `strict` |
| `party` | **luôn bật** (always-on) | text, emoji, quick, sticker, gif | 200 | 12, 3 | 0 | `strict` |
| `team` | **luôn bật** (always-on) | text, emoji, quick, sticker, gif | 200 | 12, 3 | 0 | `strict` |
| `direct` | bật (chỉ bạn bè) | text, emoji, quick, sticker, gif | 300 | 15, 4 | 0 | `strict` |

- Giới hạn tổng mỗi người (`chat_rate_per_user`): 30 tin/phút trên mọi channel **[Confirmed — Technical Architect 2026-11-16]**; cột `rate` ở bảng trên là giới hạn theo channel. module chat đếm ở Valkey xuyên node. Mục này là **nguồn duy nhất** của rate limit chat; [REALTIME_ARCHITECTURE §8.1](../06-architecture/REALTIME_ARCHITECTURE.md) và API_DESIGN chỉ tham chiếu.
- Sticker/GIF tính chung `chat_sticker_gif_rate` (4/phút/channel, §7.5); typing theo `chat_typing_throttle` (§7.10).
- Giờ chat (`chat_allowed_hours`, [Confirmed — owner 2026-11-20, OQ-09]): ngoài giờ giới nghiêm `platform_curfew` ([CHILD_SAFETY §4.1](../10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078)); `allowed_hours` của channel bền vững mặc định 06:30–21:30, trường có thể thu hẹp thêm; channel `match`/`team`/`party` không có khung giờ riêng.
- Phạm vi World (`world_chat_scope`, [Confirmed — owner 2026-11-20, OQ-09]): mặc định trong trường; tenant `unit` có thể mở rộng toàn tenant; **không** có World chat toàn organization hay liên tenant (ADR-0054).
- Link: chỉ cho phép link nội bộ EduWorld đã whitelist (ví dụ mời vào quest); link ngoài bị chặn. Ảnh/file do người dùng upload: **không** hỗ trợ; sticker/GIF chỉ từ thư viện nội bộ đã duyệt (§7.5).
- `features` (§7.1) chỉ cấp platform được đổi; cấp dưới không nới.
- Trường/lớp có thể tắt channel, thu hẹp giờ, bật slow mode, nâng strictness — áp dụng như nhau cho mọi học sinh trong phạm vi đó. **Trừ** `match`/`party`/`team`: không tắt, không thu hẹp giờ; chỉ siết rate/slow mode (≤ 10 s)/strictness.
- `allowed_hours` không áp cho channel gameplay vì việc vào chơi đã bị giới hạn bởi giờ giới nghiêm platform và giờ truy cập game của trường (CHILD_SAFETY §4).

## 4. Membership

| Channel | Nguồn membership | Đồng bộ |
|---|---|---|
| `match` | `participants[]` trong payload `match.started` (module chat không đọc database của module competition) | Consumer `match.started` (tạo channel + thêm mọi participant), `match.completed`/`match.aborted` (đóng channel) |
| `party`/`team` | Party/team events, đội trong trận | `party.created` (tạo), `party.member_joined/left`, `party.disbanded` (đóng); `match.started` tạo channel `team` cho từng đội |
| `direct` | 2 người (bạn bè, không block) | Mở bằng `openDirectChannel` (id tất định theo cặp, tạo một lần — F120). Consumer `chat.membership`: `user.blocked` → `frozen` (gửi → `CHAT_NOT_ALLOWED`), `user.unblocked` → `open` khi còn là bạn bè; bạn bè trở lại → mở lại khi gọi `openDirectChannel` |
| `team` cố định (club) | Thành viên club ([SOCIAL_SYSTEM §6](SOCIAL_SYSTEM.md#6-club--guild-nhóm-bền-vững--adr-0050-confirmed)) | Cùng transaction với lệnh club (vào/rời/bị gỡ — bởi chủ/cán sự hoặc nhân sự trường qua `removeSchoolClubMember`, F122 T061: `joined_at`/`left_at`); lưu trữ club → `read_only` |
| `world`/`area` | Không lưu `channel_members`; quyền kiểm tra động: World theo trường (`ref.students`) hoặc tenant (`world_scope = tenant`, chỉ tenant `unit`), Area theo phòng gateway báo trong tín hiệu presence (`room_id`) | Id tất định; `listMyChannels` liệt kê; hàng `chat.channels` tạo lười ở lần join/gửi đầu tiên; lịch sử chỉ từ bộ đệm gần nhất (F120). **Lọc block/mute phía server** (F120 T061, CHILD_SAFETY §3.4): fan-out sau JetStream ack mang `target.exclude_user_ids` = học sinh có block hai chiều với người gửi hoặc đã tắt tiếng người gửi (gateway không giao cho họ; đọc quan hệ không được → không phát, fail closed); lịch sử từ bộ đệm bỏ tin của người gửi mà người xem đã block/bị block/tắt tiếng. Quan hệ của học sinh cache `chat_facts_cache`, xóa ngay khi block/mute đổi; client không tự ẩn |

`chat.channel_members(channel_id, user_id, role, joined_at, left_at, muted_until, last_read_seq, notification_level)`.

### 4.1. Vòng đời channel gameplay (always-on)

```mermaid
stateDiagram-v2
  [*] --> Open: match.started / party.created
  Open --> Open: member join/leave, reconnect, đổi thiết bị (ADR-0031)
  Open --> Closed: match.completed / match.aborted / party.disbanded (ghi closed_at)
  Closed --> Closed: còn report / moderation case mở → hoãn purge
  Closed --> Purged: closed_at + gameplay_chat_retention_days (job gameplay-chat-purge, ADR-0033)
  Purged --> [*]
```

| Chế độ | Channel tự tạo | Ai được tự join | Khi kết thúc |
|---|---|---|---|
| Knowledge Battle 1v1 | 1 × `match` | 2 người chơi | `match` đóng (read-only); lịch sử xem được từ màn hình kết quả/lịch sử trận đến khi purge |
| Team Battle / Co-op Challenge | 1 × `match` (cả trận) + 1 × `team` mỗi đội | Người chơi của trận; `team` chỉ thành viên đội | `match` và `team` của trận đóng (read-only), purge theo ADR-0033 |
| Trận tournament | Như 1v1/đội theo format | Người chơi của trận | Như trên |
| Party (nhóm chơi) | 1 × `party` | Mọi member, kể cả người vào sau | `party.disbanded` → đóng (read-only), purge theo ADR-0033; rời party → mất quyền gửi, không đọc tin mới |

Quy tắc:
- module competition ghi outbox (`match.started`, `party.created`, `party.member_joined`) **cùng transaction** với tạo trận/party; module chat tạo channel và thêm member khi tiêu thụ event (idempotent). Không thể tạo channel trong cùng transaction vì hai module khác database (ADR-0037, ADR-0074). `channel_id` là UUIDv5 **tất định** từ loại channel + `match_id`/`party_id`/`team_id` (hàm thuần trong `src/backend/pkg/`), nên module competition trả `chat_channel_id` cho client ngay mà không chờ module chat; client gửi tin trước khi channel kịp tạo nhận lỗi retryable và tự gửi lại. Không có bước "bật chat". Hàm: `src/backend/pkg/chatid` (`Party(party_id)`, `Match(match_id)`, `Team(match_id, team_no)` = UUIDv5 của `"<kind>:<scope_id>[:<team_no>]"`, namespace cố định) — F117. Consumer `chat.gameplay_channels` (worker events) nhận `party.*` (cả `party.created` v1 và v2), `match.started`, `match.completed`, `match.aborted`; chịu được giao lại và lệch thứ tự: phiên bản thay đổi theo từng thành viên (`ref.parties.member_versions`, `state_version`) — thay đổi cũ hơn bị bỏ qua; event kết thúc đến trước `match.started` tạo channel ở trạng thái đóng; channel đã đóng không bao giờ thêm thành viên. Trận đội = ≥ 2 đội và có đội ≥ 2 người (mỗi đội một channel `team`); 1v1 và co-op một đội chỉ có `match`. Người tham gia khác tenant của event hoặc không phải học sinh đang hoạt động trong `ref.students` (nhân sự, bot) bị bỏ qua, ghi metric `eduworld_chat_gameplay_member_skipped_total{reason}` (không PII). Khi đóng: `closed_at` = thời điểm của event, `purge_after = closed_at + gameplay_chat_retention_days` hiệu lực (ghi đè tenant nếu platform cho phép, nếu không thì giá trị platform, chưa có thì mặc định cấu hình; dưới 3 ngày → 3 ngày + cảnh báo) chốt lúc đóng — đổi cài đặt sau đó không áp hồi tố.
- **Reconnect / đổi thiết bị** (ADR-0031): người chơi tiếp tục trận từ thiết bị mới vẫn là member channel; client gọi `system.session.resume` với `last_seq` để nhận tin bỏ lỡ.
- Không giới hạn theo quan hệ bạn bè/trường trong channel gameplay: ai cùng trận/party đều chat được với nhau (người ngoài trường chỉ thấy nickname). Block vẫn ẩn tin hai chiều; người chơi bị block không bị đưa ra khỏi trận. Server tính danh sách người nhận khi fan-out (thành viên đang hoạt động trừ người có quan hệ block hai chiều với người gửi và người đã tắt tiếng người gửi) và lọc cùng quy tắc trong lịch sử (`listChannelMessages`, `chat.history.fetch`, `chat.channel.sync`); tắt tiếng chỉ ẩn tin một chiều (F117-AC-12).
- Học sinh đang bị sanction `mute` vẫn vào trận/party bình thường, chỉ không gửi được tin (UI hiển thị "Bạn đang tạm thời không thể chat") **[Confirmed]**.

### 4.2. UI chat trong trận và party (Godot)

- Panel chat luôn có trong màn hình party lobby, trận 1v1, team battle (tab "Đội" / "Cả trận"). Người chơi có thể **thu gọn/ẩn panel** nhưng không tắt được tính năng; tin mới hiển thị badge.
- Hàng quick message (emoji + câu soạn sẵn như "Chúc may mắn!", "Hay quá!") luôn hiển thị để chat nhanh không làm gián đoạn câu hỏi; ô nhập text mở bằng một chạm.
- Mỗi tin có menu nhanh: **Tắt tiếng người này**, **Chặn**, **Báo cáo** (report gắn `match_id`/`party_id` làm ngữ cảnh).
- Trong lúc câu hỏi đang hiển thị ở Knowledge Battle hoặc boss, panel thu nhỏ tự động để không che câu hỏi; tin vẫn nhận và lưu [Confirmed].
- Team battle có hai tab **Đội** (chỉ đồng đội) và **Cả trận**.
- Màn hình lịch sử trận/party ghi "Tin nhắn trong trận được giữ {N} ngày" (mặc định 7, tối thiểu 3 — §6.1); học sinh cần báo cáo thì báo cáo trước thời hạn.

### 4.3. Trải nghiệm chat của học sinh

Áp dụng như nhau cho mọi lớp 1–9 (ADR-0020); khác biệt chỉ ở cỡ chữ/nút theo [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md).

**Phản hồi khi gửi** (quyết định từ [MODERATION](MODERATION.md)):

| Quyết định | Người gửi thấy | Người nhận thấy |
|---|---|---|
| `allow` / `unchecked` | Tin nhắn bình thường | Tin nhắn |
| `mask` | Tin với `***` + gợi ý "Hãy dùng lời lẽ thân thiện nhé" | Tin đã che |
| `block` (PII: SĐT, địa chỉ, link, mạng xã hội) | "Tin nhắn chưa gửi được: không chia sẻ thông tin cá nhân để giữ an toàn" | Không thấy |
| `block` (từ trong danh mục, mức trung bình) | "Tin nhắn có từ chưa phù hợp, em thử viết lại nhé" | Không thấy |
| `hold_for_review` | "Tin nhắn đang được kiểm tra" | Thấy sau khi duyệt (hoặc không) |
| `block_and_alert` (tự hại, dụ dỗ) | Thông điệp quan tâm + gợi ý nói với thầy cô/bố mẹ, đường dây nóng 111 khi phù hợp | Không thấy |
| Gửi quá nhanh | "Bạn gửi hơi nhanh, chờ chút nhé" | — |

Không bao giờ cho người gửi biết từ nào bị khớp, vị trí khớp hay danh mục cụ thể (ADR-0030). Khi System Admin tắt kiểm duyệt cho tenant, học sinh không thấy khác biệt nào [Confirmed].

**Menu thao tác trên tin** (nhấn giữ trên mobile, chuột phải hoặc nút "⋯" trên Web/PC); mục hiển thị theo quyền và loại channel (§7.1):

| Mục | Ai thấy | Trải nghiệm |
|---|---|---|
| Trả lời | Mọi người | Thanh "Đang trả lời An: …" trên ô nhập; tin gửi có khung trích ngắn, chạm để nhảy tới tin gốc |
| Trích dẫn | Mọi người | Chọn đoạn chữ cần trích (server cắt từ bản đã che) |
| Chuyển tiếp | Mọi người (trừ World/Area) | Chọn tối đa 5 channel em được gửi; tin đến có nhãn "Đã chuyển tiếp từ {nickname}" |
| Sao chép | Mọi người | Thông báo nhỏ "Đã sao chép" |
| Ghim | Theo §7.8 | Tối đa 3 tin, hiện ở thanh trên cùng |
| Sửa | Người gửi, trong 15 phút | Ô nhập điền sẵn nội dung cũ; tin có nhãn "đã chỉnh sửa" |
| Thu hồi | Người gửi, trong 24 giờ | Hỏi lại trước khi thu hồi; mọi người thấy "Tin nhắn đã được thu hồi" |
| Thả cảm xúc | Mọi người | Hàng 6 emoji nhanh; tối đa 3 reaction/tin |
| Tắt tiếng · Chặn · Báo cáo | Mọi người (trừ tin của chính mình) | §9 |

**Thành phần khác:**
- Nút mặt cười mở bảng 3 tab Emoji / Sticker / GIF; pack sticker khóa hiện ổ khóa + "Mở khóa khi hoàn thành nhiệm vụ…"; GIF tìm theo chủ đề trong thư viện nội bộ, không có ô tìm kiếm Internet.
- Hàng quick message theo ngữ cảnh luôn ở trên ô nhập để học sinh nhỏ chat bằng một chạm (§8.5).
- Gõ `@` hiện danh sách nickname thành viên; tin có mention được tô sáng, biểu tượng chat có chấm "@".
- "An đang soạn tin…" dưới khung chat (không có ở World/Area).
- Trạng thái tin của mình: ✓ đã gửi, ✓✓ đã nhận, ✓✓ màu xanh đã xem; nhóm nhỏ chạm để xem ai đã xem, lớp chỉ hiện "12 bạn đã xem".
- Badge số chưa đọc trên nút chat ở HUD và từng channel; badge "@" riêng.
- Kéo lên để xem tin cũ; kính lúp tìm trong các channel đang tham gia (có dấu/không dấu đều được); tin trận chỉ tìm được trong thời hạn lưu giữ.
- Tin chưa gửi được có biểu tượng đồng hồ và tự gửi lại khi có mạng; chat tạm gián đoạn hiện "Đang kết nối lại chat…".

## 5. Kiến trúc truyền tải & message flow (ADR-0035)

### 5.1. Phân vai

| Thành phần | Vai trò trong chat | Không làm |
|---|---|---|
| **NATS JetStream** (stream `CHAT_MESSAGES`) | Đường ghi **bền vững**: mọi tin mới và mọi sự kiện thay đổi tin (sửa, thu hồi, reaction, pin, gỡ bởi moderator, forward) được publish vào stream **trước khi** người gửi nhận "Sent". Bộ đệm cho `chat-persister` và nguồn cho consumer bất đồng bộ | Không lưu lâu dài (`max_age` 72 h) |
| **Valkey Pub/Sub** (`ew:chat:fanout:<channel_id>`) | **Fan-out realtime** tới các node `realtime` giữ kết nối của thành viên (module chat publish, gateway subscribe). Mang cả tín hiệu tạm thời: typing, receipts, `chat.unread.updated` | Không đảm bảo giao, không lưu |
| **Valkey** (key thường) | Cấp `seq` (`INCR ew:chat:seq:<channel_id>`), bộ đệm gần nhất `ew:chat:recent:<channel_id>`, cache unread, rate limit | Không là nguồn sự thật |
| **PostgreSQL** (schema `chat`) | **Nguồn sự thật**: tin nhắn, revision, reaction, pin, con trỏ, lịch sử, tìm kiếm | Không nằm trên đường trễ gửi/nhận |

Game client và CMS **không bao giờ** kết nối NATS hay Valkey; chỉ đi qua WebSocket `/realtime/v1` và REST `/api/v1/chat/*`.

### 5.2. Luồng gửi

```mermaid
sequenceDiagram
  autonumber
  participant C as Godot (sender)
  participant RT as realtime (node A) → module chat (NATS request ew.cmd.chat.message.send)
  participant F as Moderation pipeline (sync)
  participant VK as Valkey
  participant JS as NATS JetStream (CHAT_MESSAGES)
  participant PS as Valkey Pub/Sub
  participant RT2 as realtime (node B)
  participant R as Recipients

  C->>RT: chat.message.send {client_message_id, channel_id, kind, body | quick_message_id | sticker_id | gif_id, reply_to?, quote?, mentions?}
  RT->>RT: Membership + EffectivePolicy + features (§7) + rate limit (Valkey)
  alt Bị từ chối
    RT-->>C: system.command.failed {ref: id lệnh, code}
  end
  RT->>F: Validate(text, quote excerpt, tham số quick)  [theo moderation_switches]
  F-->>RT: allow | unchecked | mask | block | hold_for_review | block_and_alert
  alt block
    RT-->>C: system.command.failed {code: CHAT_CONTENT_BLOCKED}
  else allow / unchecked / mask / hold / block_and_alert
    RT->>VK: EVAL seq_next.lua ew:chat:seq:<channel_id> (khởi tạo từ MAX(seq) PG nếu thiếu)
    VK-->>RT: seq
    RT->>JS: PUBLISH ew.chat.<tenant_id>.msg.<channel_id> (Nats-Msg-Id = message_id)
    alt NATS lỗi / timeout chat_publish_ack_timeout (2 s)
      RT-->>C: system.command.failed {code: CHAT_TEMPORARILY_UNAVAILABLE, retryable: true}
    else ack
      JS-->>RT: PubAck {stream_seq}
      RT-->>C: chat.message.acked {client_message_id, message_id, seq}  (Sent)
      opt decision ∈ {allow, unchecked, mask}
        RT->>VK: LPUSH+LTRIM ew:chat:recent:<channel_id> (200 tin, TTL 10')
        RT->>PS: PUBLISH ew:chat:fanout:<channel_id>
        PS-->>RT2: fan-out
        RT2-->>R: chat.message.created
        R-->>RT2: chat.receipt.delivered {channel_id, seq}
      end
      opt decision ∈ {hold_for_review, block_and_alert}
        RT-->>C: chat.message.held (không phát cho người khác)
      end
    end
  end
```

- Tin chỉ được phát **sau khi** JetStream ack. Không có tin nào tới người nhận mà chưa nằm trong JetStream.
- `client_message_id` (tên duy nhất ở REST, realtime và DB) là idempotency key ở realtime (Valkey `SET NX ew:chat:cmid:<sender>:<client_message_id>` TTL 10'), `message_id` là khóa khử trùng lặp của JetStream (`duplicate_window` 2') và PostgreSQL (`ON CONFLICT DO NOTHING`).
- REST fallback `POST /api/v1/chat/channels/{channel_id}/messages` dùng cùng use case.
- `seq` cấp trước khi publish; nếu publish thất bại, `seq` đó bị bỏ trống. Client coi khoảng trống do lỗi gửi là bình thường: lịch sử không có tin đó, không retry vô hạn (§5.4).

### 5.3. Luồng lưu và consumer bất đồng bộ

```mermaid
sequenceDiagram
  participant JS as JetStream CHAT_MESSAGES
  participant P as chat-persister (worker-chat, N replica)
  participant PG as PostgreSQL
  participant N as chat-notifier
  participant M as chat-moderation-async
  participant S as chat-stats
  JS->>P: Fetch batch (durable "persister", ack_wait 30 s)
  P->>PG: INSERT ... ON CONFLICT (message_id) DO NOTHING, apply edit/recall/reaction/pin/cursor, UPDATE channels.last_seq = GREATEST(...)
  PG-->>P: commit
  P->>JS: Ack (chỉ sau commit)
  Note over P,JS: Lỗi > max_deliver (5, chat_persister_batch) → publish DLQ_CHAT + alert
  JS->>N: durable "notifier": mention / tin mới cho member offline
  JS->>M: durable "moderation": bước async_analysis (ADR-0030) nếu bật
  JS->>S: durable "stats": rollup cho analytics (ADR-0033)
```

- Mục tiêu độ trễ lưu (publish → commit) p95 ≤ 500 ms [Confirmed]; metric `eduworld_chat_persist_lag_seconds`.
- Persister ghi theo lô `chat_persister_batch` (≤ 500 sự kiện hoặc 50 ms; `max_deliver` 5 → `DLQ_CHAT` + alert). Thứ tự ghi không quan trọng vì `seq` đã cấp trước; sự kiện thay đổi (sửa, thu hồi) áp dụng theo `revision` tăng dần, bản cũ đến muộn bị bỏ qua.
- Con trỏ đọc (`chat.receipt.read`) được realtime gộp (tối đa 1 lần/2 s/thành viên) rồi publish `ew.chat.<tenant_id>.cursor.<channel_id>` để persister cập nhật `channel_members.last_read_seq`.
- Tin `world`/`area` (ephemeral) cũng đi qua JetStream; persister ghi vào `chat.ephemeral_messages` (purge 24 h, ADR-0033).

### 5.4. Độ tin cậy

| Sự cố | Hành vi |
|---|---|
| Mất gói Valkey Pub/Sub (node restart, mạng chập chờn) | Client theo dõi `seq` từng channel; phát hiện **seq gap** → gọi `chat.channel.sync` hoặc `GET /chat/channels/{id}/messages?after_seq=` để bù. Server trả từ `ew:chat:recent` (tin chưa kịp vào PG) hợp nhất với PostgreSQL |
| NATS không sẵn sàng | Realtime từ chối gửi `CHAT_TEMPORARILY_UNAVAILABLE` (retryable); UI hiển thị "Đang kết nối lại chat…". Đọc lịch sử vẫn hoạt động |
| PostgreSQL không sẵn sàng | Gửi/nhận realtime vẫn chạy: facts của đường gửi (channel, thành viên, effective policy, giới nghiêm) đọc từ cache Valkey, tươi `chat_facts_cache`, dùng bản cũ tới `chat_facts_stale_ttl` (hết → `CHAT_TEMPORARILY_UNAVAILABLE`, fail closed); JetStream giữ tối đa 72 h; persister giữ lô (không tốn lần giao) và ghi bù khi DB trở lại. Lịch sử phục vụ tạm từ `ew:chat:recent` |
| Valkey mất key `seq` | Script Lua khởi tạo lại từ `MAX(seq)` trong PostgreSQL + số tin còn pending trong JetStream (đọc `channels.last_seq` sau khi persister bắt kịp); unique `(channel_id, seq)` chặn trùng — xung đột hiếm gặp được persister gán `seq` mới và phát `chat.message.resequenced` [Confirmed] |
| Persister chậm | Alert khi consumer pending > 10.000 hoặc lag p95 > 5 s |

Thứ tự hiển thị luôn theo `seq`, không theo thứ tự đến.

## 6. Lưu trữ (PostgreSQL — ghi bởi `chat-persister`)

| Bảng | Ghi chú |
|---|---|
| `chat.channels` | `channel_id`, `tenant_id`, `channel_type`, `scope_ref`, `school_id?`, `is_gameplay`, `policy_overrides`, `retention_days`, `last_seq` (persister cập nhật bằng `GREATEST`), `status`, `closed_at`, `purge_after` |
| `chat.channel_members` | §4 + `last_delivered_seq`, `last_read_seq`, `unread_mentions` (con trỏ, §8.1) |
| `chat.messages` | Tin của channel **bền vững** (team cố định, direct; club theo SOCIAL_SYSTEM — không còn class/study_group, ADR-0095). **Partition theo tháng** (`created_at`); unique `(channel_id, seq)` và `message_id`. Cột tính năng: `kind` (`text`/`emoji`/`quick`/`sticker`/`gif`/`system`), `body` (gốc), `body_display` (đã che), `sticker_id`, `gif_id`, `quick_message_id`, `reply_to_message_id`, `quote jsonb`, `forwarded_from jsonb`, `mentions uuid[]`, `edited_at`, `edit_count`, `revision`, `recalled_at`, `recalled_by`, `moderation_state`, `search_tsv` (generated, unaccent) |
| `chat.gameplay_messages` | Cùng cột với `messages`, cho channel gameplay (`match`, `team` theo trận, `party`). **Partition theo ngày tạo** (`created_on`), không phải di chuyển dòng khi channel đóng; purge theo channel đã hết hạn (§6.1) |
| `chat.message_revisions` | `(message_id, revision)`, `body`, `decision`, `edited_at`: bản cũ khi sửa (bằng chứng) |
| `chat.message_reactions` | `(message_id, user_id, emoji_code)` unique |
| `chat.pinned_messages` | `(channel_id, message_id)`, `pinned_by`, `pinned_at`; tối đa 3/channel |
| `chat.sticker_packs`, `chat.stickers` | Platform (Game Studio), `status`, `unlock_rule`, gán tenant (ADR-0029) |
| `chat.gif_assets`, `chat.gif_tags` | Thư viện GIF nội bộ đã duyệt, platform-only |
| `chat.quick_messages` | Danh mục đã duyệt, đa ngôn ngữ, quản lý trong CMS |
| `chat.ephemeral_messages` | World/area, partition theo ngày, **purge sau 24 h** (ADR-0033) |
| `chat.chat_policies` | §3 |

- `seq` theo channel được cấp bằng Valkey `INCR`, script Lua khởi tạo từ PostgreSQL (ADR-0035, [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md)). PostgreSQL giữ unique `(channel_id, seq)` làm chốt chặn cuối.
- **Channel gameplay và World/Area: purge vật lý** theo §6.1 (ngoại lệ có kiểm soát của ADR-0028, quy định ở ADR-0033).
- **Channel bền vững:** retention job **ẩn danh hóa** nội dung quá `retention_days` (ghi đè `body`, `body_display`, revision; giữ metadata) và ẩn khỏi API. Ứng dụng không hard-delete (ADR-0028). Release 1.0 không gỡ partition, không xóa vật lý; partition đã ẩn danh hóa được giữ lại (ADR-0070). Tin liên quan `moderation_case` đang mở được sao vào case trước **[Confirmed]**. Job `persistent-chat-retention` (F120, worker-chat, role `chat_app`, mỗi giờ): theo tenant, theo lô (`EW_CHAT_PERSISTENT_RETENTION_BATCH`), tin `team` cố định/`direct` quá `coalesce(channels.retention_days, persistent_chat_retention_days)` → sao bằng chứng (report, case, case action ±`evidence_context`) vào `chat.moderation_evidence` → ghi đè `body`, `body_display`, `quote`, revision (`chat.f_revisions_guard` chỉ cho phép đặt NULL) và đặt `anonymized_at`; ghi `chat.persistent_retention_runs` + audit tổng hợp `chat.retention_applied` (số đếm, không nội dung); metric `eduworld_chat_retention_anonymized_total`.
- **Thu hồi** (người gửi) = `recalled_at`. **Gỡ** (moderator) = `moderation_state = removed` + soft-delete. Cả hai giữ nội dung gốc làm bằng chứng; API người dùng chỉ trả placeholder.

### 6.1. Purge chat gameplay (ADR-0033)

| Loại channel | Bị purge? | Mốc tính | Thời hạn |
|---|---|---|---|
| `match` | Có | `closed_at` (trận kết thúc/hủy) | `gameplay_chat_retention_days`: mặc định 7, **tối thiểu 3** [Confirmed]; job theo `gameplay_chat_purge_job` (hằng giờ, lô 1.000 dòng) |
| `team` gắn với một trận | Có | `closed_at` | như `match` |
| `party` | Có | `closed_at` (party giải tán) | như `match` |
| `world`, `area` (ephemeral) | Có | `created_at` | 24 giờ |
| `team` cố định, `direct` | **Không** | — | Chính sách lưu trữ chung, ẩn danh hóa (DATA_PRIVACY, ADR-0070) |

Quy trình job `gameplay-chat-purge` (profile `worker-chat` của module chat, khóa leader, role DB `chat_purger`, chi tiết DB/metric ở DATABASE_DESIGN và MONITORING):
1. Rollup chỉ số tổng hợp (số tin, số tin bị chặn theo category…) vào analytics.
2. Tin thuộc report, moderation case hoặc sanction → **sao chép** sang `chat.moderation_evidence` ([MODERATION](MODERATION.md) §6.1).
3. Channel còn report chưa xử lý hoặc case đang mở → **hoãn purge** đến khi case đóng.
   **Fail-closed:** khi module moderation chưa sẵn sàng (chưa có nguồn report/case/sanction để kiểm, ví dụ trước khi triển khai Platform Safety Team) hoặc sao bằng chứng của một channel thất bại, job **không xóa** channel đó, đặt gauge `eduworld_chat_purge_blocked{reason="moderation_unavailable"} = 1` và cảnh báo vận hành (lần chạy ghi `status = blocked`, một dòng cấp lượt chạy mỗi loại). Mỗi channel được purge trong **một transaction** (rollup → bằng chứng → xóa): bất kỳ bước nào lỗi thì toàn bộ channel được giữ nguyên đến lần chạy sau (`eduworld_chat_purge_failures_total{stage}`). Thiếu DSN của `chat_purger` thì job không chạy (`reason="purger_not_configured"`), không role nào khác xóa thay. Bằng chứng do purge sao có `snapshot.kind = gameplay_purge` (hoặc `ephemeral_purge`), `target_id`, ±`evidence_context` tin (nội dung gốc, revision, trạng thái kiểm duyệt), ngữ cảnh `channel_type`, `match_id`/`party_id`, `catalog_version`, `report_id`; id tất định theo (case, tin) nên chạy lại không sao trùng. Giữ dữ liệu lâu hơn hạn tạm thời là chấp nhận được; mất bằng chứng thì không.
4. Purge **theo channel** đã hết hạn: xóa lô 1.000 dòng `chat.gameplay_messages`, revision, reaction, pin, member, channel (bảng revision/reaction/pin do F118 tạo; khi có, purge xóa kèm theo cùng điều kiện) của các channel có `purge_after < now()` và không bị hoãn. Partition ngày `created_on = D` chỉ `DETACH` + `DROP` khi **mọi** channel có tin trong partition đó đã purge (kiểm tra bằng bảng theo dõi `chat.gameplay_partition_channels`) — nhanh với trận ngắn, an toàn với party sống lâu. Không có API ứng dụng nào làm việc này.
   *Lý do không partition theo `closed_on`:* tin được ghi khi channel còn mở (chưa biết `closed_on`), nên partition theo `closed_on` buộc phải di chuyển dòng khi đóng channel — tốn I/O và cần quyền ghi xóa cho role app. Partition theo ngày tạo tránh việc này [Confirmed].
5. Ghi audit tổng hợp (không nội dung) + metric `eduworld_chat_purge_total{tenant,kind}`. Mỗi lượt ghi `chat.gameplay_purge_runs` (theo tenant + dòng cấp lượt chạy), audit `chat.gameplay_purge.run`, event `chat.gameplay_purged` và `chat.daily_stats` (phần cộng thêm của rollup); tồn đọng quá `gameplay_chat_purge_job` (2 ngày) → cảnh báo (`eduworld_chat_purge_backlog_oldest_seconds`).
6. Tin `world`/`area` (`chat.ephemeral_messages`) quá `ephemeral_chat_retention`: sao bằng chứng tin thuộc report/case của từng tenant trước; chỉ khi mọi tenant sao thành công mới `DETACH` + `DROP` partition ngày (hàm `chat.drop_ephemeral_partition`), rồi xóa lô phần còn lại (partition default, ngày dở dang) của tenant đã sao xong.
7. Job `chat-partition-maintainer` (cùng profile, cùng role) tạo trước partition ngày cho `gameplay_messages` và `ephemeral_messages` (`chat_message_partition_days_ahead`); không bao giờ drop.

Phía người chơi: màn hình lịch sử trận/party ghi rõ "Tin nhắn trong trận được giữ N ngày", sau thời hạn chỉ còn kết quả trận, không còn tin nhắn.

## 7. Tính năng tin nhắn (ADR-0034)

### 7.1. Ma trận tính năng × loại channel (mặc định platform)

| Tính năng | world | area | match | party | team | direct |
|---|---|---|---|---|---|---|
| Gửi / nhận (text, emoji, quick) | ✓ | ✓ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Sticker / GIF | ✓ | ✓ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Sửa (15') | ✓ | ✓ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Thu hồi (24 h) | ✓ | ✓ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Reply / Quote | ✓ | ✓ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Forward **ra khỏi** channel | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ |
| Reactions | ✓ | ✓ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Mention `@nickname` | ✓ | ✓ | ✓ luôn | ✓ luôn | ✓ luôn | — (2 người) |
| Pin (≤ 3) | ✗ | ✗ | ✓ | ✓ (leader) | ✓ | ✓ (cả hai) |
| Copy | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Typing indicator | ✗ | ✗ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Receipts | ✗ | ✗ | Chi tiết từng người (≤ 8) | Chi tiết (≤ 8) | Chi tiết (≤ 8) | Chi tiết |
| Unread + mention count | ✗ (chỉ badge mention) | ✗ | ✓ luôn | ✓ luôn | ✓ luôn | ✓ |
| Lịch sử & tìm kiếm | Lịch sử 24 h, không tìm | Lịch sử 24 h, không tìm | ✓ đến khi purge | ✓ đến khi purge | ✓ | ✓ |

- Cấu hình bằng `features` trong chat policy, **chỉ cấp platform** được bật/tắt từng tính năng; tenant/school không nới được. Channel gameplay (`match`/`party`/`team`): các mục "luôn" không tắt được (schema chặn).
- "✓ luôn" = bắt buộc theo ADR-0020 mục 6 + ADR-0034.

### 7.2. Sửa tin

- Lệnh `chat.message.edit {message_id, body, revision_expected}`. Chỉ người gửi, trong **15 phút** kể từ `created_at` [Confirmed]; quá hạn → `CHAT_EDIT_WINDOW_EXPIRED`. Chỉ sửa `text` (sticker/GIF/quick không sửa).
- Nội dung mới **chạy lại toàn bộ pipeline** (ADR-0030, theo công tắc). `block` → giữ bản cũ, trả `CHAT_CONTENT_BLOCKED`; `hold_for_review` → bản mới chờ duyệt, người khác vẫn thấy bản cũ.
- Bản cũ lưu `chat.message_revisions (message_id, revision, state, body, decision, edited_at)` (`state = previous`; bản sửa bị giữ chờ duyệt lưu `state = held`, không đổi revision của tin, mở case qua `chat.message_held` — F118); `messages.edit_count`, `edited_at`. Người dùng chỉ thấy nhãn "đã chỉnh sửa", **không** xem được bản cũ; moderator xem trong case. Revision serialize bằng compare-and-set (`If-Match`/`revision_expected` lệch → `412 VERSION_MISMATCH`); sự kiện đi qua stream `ew.chat.<tenant>.evt.<channel>` trước khi phát (F118).
- Sự kiện: JetStream `chat.message_edited` → fan-out `chat.message.edited {message_id, seq, body, edited_at, revision}`.

### 7.3. Thu hồi

- Lệnh `chat.message.recall {message_id}`. Người gửi, trong **24 giờ** [Confirmed]; quá hạn → `CHAT_RECALL_WINDOW_EXPIRED`.
- Mọi người thấy placeholder "Tin nhắn đã được thu hồi"; reaction, pin, quote/reply tới tin đó hiển thị "Tin nhắn gốc không còn". Tin bị pin tự bỏ ghim.
- Là **soft** (ADR-0028): đặt `recalled_at`, `recalled_by`; nội dung gốc **giữ làm bằng chứng** đến hết thời hạn lưu trữ của channel (gameplay: đến purge, ADR-0033), chỉ moderator xem trong case. API người dùng, tìm kiếm, forward, quote không bao giờ trả nội dung tin đã thu hồi.
- Vẫn **báo cáo được** tin đã thu hồi trong cửa sổ báo cáo (chống bắt nạt rồi xóa dấu vết) — [MODERATION](MODERATION.md) §6.1.
- `safety_moderator` gỡ tin của người khác là thao tác **moderation** (`moderation_state = removed`, audit), không phải thu hồi.

### 7.4. Reply, Quote, Forward

| Tính năng | Dữ liệu | Quy tắc |
|---|---|---|
| Reply | `reply_to_message_id` (cùng channel) | Server trả `reply_excerpt` (≤ 80 ký tự) tính lúc đọc từ bản **đã kiểm duyệt** của tin gốc; tin gốc thu hồi/gỡ/held/purge → vắng `reply_excerpt`, client hiển thị "Tin nhắn gốc không còn" (F118) |
| Quote | `quote: {message_id, excerpt_start, excerpt_end}` | Server tự cắt `excerpt` từ bản đã kiểm duyệt (đã che); client **không** gửi nội dung trích. Excerpt ≤ 200 ký tự |
| Forward | Lệnh `chat.message.forward {message_id, target_channel_ids[≤5]}` → tạo tin mới ở mỗi channel đích với `forwarded_from: {message_id, channel_type, sender_nickname}` | Chỉ trong **cùng tenant**; người dùng phải được gửi tin ở channel đích; **chạy lại pipeline và policy của channel đích**; không forward tin đã thu hồi/bị che/bị gỡ/đang held; không forward ra khỏi `world`/`area`. Vi phạm → `CHAT_FORWARD_NOT_ALLOWED` |

### 7.5. Emoji, Sticker, GIF

- **Emoji**: bộ Unicode giới hạn đã duyệt (danh sách cấu hình platform: `src/backend/modules/chat/seed/emoji/emoji_sets.json`, mã emoji và tập con dùng làm reaction), loại emoji có thể dùng để bắt nạt/gợi dục; ngoài bộ → `422 VALIDATION_FAILED` (F118).
- **Sticker**: `chat.sticker_packs` → `chat.stickers`, do **Game Studio (platform-only, ADR-0027)** quản lý; pack có thể **mở khóa làm phần thưởng** (`unlock_rule` tham chiếu reward rule) hoặc mặc định cho mọi người. Tin `kind = sticker` gửi `sticker_id`; server kiểm tra pack đã publish, được gán cho tenant (ADR-0029) và học sinh đã sở hữu (nếu pack bị khóa).
- **GIF**: **thư viện nội bộ đã duyệt** (`chat.gif_assets` + `chat.gif_tags`), platform upload qua presigned URL, kiểm tra kích thước (≤ 2 MB, ≤ 480 px), gắn tag tiếng Việt/Anh, publish; phục vụ từ CDN EduWorld. Học sinh tìm theo tag (`GET /chat/gifs?tag=`). **Không** gọi Giphy/Tenor hay dịch vụ GIF bên thứ ba.
- **Quản trị thư viện (Game Studio, F082)**: danh sách một dòng mỗi pack/GIF (version mới nhất) phân trang keyset trong truy vấn, lọc trạng thái/người upload/tag; ảnh bản chưa phát hành chỉ xem qua URL GET ký ngắn hạn (`media_presign_ttl`) từ bucket riêng tư — không qua CDN trước khi publish (R31). Tên pack và alt text từng sticker: `vi` bắt buộc khi lưu nháp, `en` bắt buộc khi `submit_review` (`game_text_required_languages`, `422 TRANSLATION_INCOMPLETE`); khi gửi duyệt server ghi lại kích thước/định dạng/kích cỡ đọc từ bucket vào version (không tin số client khai). `unlock_rule` là `{item_id}` (mã vật phẩm khớp `item.granted`) hoặc `{reward_rule_id}` (reward rule đã publish). Rollback chỉ về version từng publish (dựng lại thư viện, đổi `current_version_id`, version hiện hành → `archived`); unpublish đặt pack/GIF `archived`, học sinh không chọn được nữa nhưng tin đã gửi vẫn hiển thị (asset bất biến); lý do bắt buộc, `content.rolled_back`/`content.unpublished` + audit. Chỉ sticker pack gán được cho tenant (facade `applyTenantAssignments`/`revokeTenantAssignments`/`listResourceAssignments` của module chat, gọi từ module identity — hộp gán F057); GIF là thư viện chung.
- **Không có upload ảnh/file của người dùng**. Sticker/GIF không qua bước kiểm tra văn bản (đã duyệt trước) nhưng vẫn chịu rate limit `chat_sticker_gif_rate` (sticker/GIF tính chung, tối đa 4/phút mỗi người gửi mỗi channel — cùng đơn vị với rate channel, F118) **[Confirmed — Technical Architect 2026-11-16]**; file theo `chat_asset_file_max` (sticker WebP/PNG ≤ 512 KB; GIF ≤ 2 MB, ≤ 480 px, có preview tĩnh).

### 7.6. Reactions

`chat.reaction.toggle {message_id, emoji_code}` → bật/tắt; chỉ emoji trong bộ reaction; tối đa **3 reaction/người/tin** [Confirmed]. Lưu `chat.message_reactions`; fan-out `chat.reaction.updated {message_id, counts, my_reactions}`. Không reaction tin đã thu hồi.

### 7.7. Mention

- Cú pháp `@nickname` trong text; client gửi kèm `mentions: [user_id]` lấy từ autocomplete. Server **kiểm tra lại**: mỗi user_id phải là thành viên channel, không block người gửi; user_id không hợp lệ bị **loại khỏi** danh sách mention (tin vẫn gửi, không lỗi, không thông báo cho người đó); nickname hiển thị lấy từ GameProfile — **không** dùng username đăng nhập (chứa mã học sinh).
- Tối đa **5 mention/tin** → vượt `CHAT_MENTION_LIMIT`. Không có `@all` (giảm spam, tránh dồn dập thông báo) [Confirmed].
- Người được mention: tăng `channel_members.unread_mentions`, nhận `chat.mention.created` (realtime) hoặc thông báo qua `chat-notifier` khi offline; `notification_level = mentions` vẫn nhận.

### 7.8. Pin

- `chat.pin.add/remove {message_id}`; tối đa **3 tin/channel** → `CHAT_PIN_LIMIT_REACHED`. Quyền: leader (party), thành viên (match/team — bất kỳ người chơi), cả hai (direct). Không có ở world/area (không có người điều khiển channel trong các channel này).
- Lưu `chat.pinned_messages (channel_id, message_id, pinned_by, pinned_at)`; audit; fan-out `chat.pin.updated`. Tin bị thu hồi/gỡ → tự bỏ ghim.

### 7.9. Copy

Tính năng client: copy **nội dung đang hiển thị** (bản đã che, placeholder nếu đã thu hồi) vào clipboard (`DisplayServer.clipboard_set` trong Godot). Không có API riêng; không copy được nội dung gốc.

### 7.10. Typing indicator

`chat.typing.start/stop {channel_id}` → realtime throttle **1 lần/3 s**, tự hết sau **5 s** không cập nhật (`chat_typing_throttle`); publish chỉ qua Valkey Pub/Sub (`chat.typing.updated {channel_id, user_ids[]}`), **không** qua JetStream, **không** lưu DB. Không gửi tới người đã block; không có ở world/area. Hiển thị tối đa 3 nickname ("An, Bình đang soạn tin…").

## 8. Lịch sử, receipts, unread, tìm kiếm

### 8.1. Sent / Delivered / Read (theo con trỏ)

| Trạng thái | Định nghĩa | Lưu ở đâu |
|---|---|---|
| **Sent** | Server đã chấp nhận và JetStream đã ack (`chat.message.acked`) | Envelope JetStream → `messages` |
| **Delivered** | Thiết bị người nhận đã nhận và ack `chat.receipt.delivered {channel_id, seq}` | `channel_members.last_delivered_seq` (gộp, max) |
| **Read** | Người nhận mở channel, con trỏ đọc vượt `seq` (`chat.receipt.read {channel_id, seq}`) | `channel_members.last_read_seq` (gộp, max) |

- **Không** lưu một dòng cho mỗi tin × người. Trạng thái của tin `seq = s` với người X: Read nếu `X.last_read_seq ≥ s`, Delivered nếu `X.last_delivered_seq ≥ s`, ngược lại Sent.
- Realtime gộp receipt (`chat_receipt_coalesce`: ≤ 1 lần/2 s/thành viên/channel), fan-out `chat.receipt.updated {channel_id, user_id, delivered_seq, read_seq}` qua Valkey Pub/Sub (F118). Con trỏ đọc (`chat.receipt.read`, F117): receipt đầu của cửa sổ 2 s ghi ngay `channel_members.last_read_seq = GREATEST(...)` (không lùi, không vượt seq đã biết), các receipt sau trong cửa sổ gộp ở `ew:chat:cursor_buf` và một lần ghi cuối mang seq mới nhất; sau mỗi lần ghi, mọi thiết bị của người đọc nhận `chat.unread.updated`.
- Hiển thị: nhóm ≤ 8 người (party, match, team, direct) — chi tiết từng người; world/area — không có. Không còn channel đếm "N người đã xem" (class/study_group bỏ theo ADR-0095).
- Người dùng bị block không thấy trạng thái đọc của nhau.

### 8.2. Unread count

- Theo channel: `channels.last_seq − channel_members.last_read_seq` (hiển thị "99+"); mention chưa đọc: `channel_members.unread_mentions` (reset khi đọc qua tin chứa mention).
- Tổng badge HUD = tổng unread của các channel `notification_level ≠ none` + tổng mention. Cache `ew:chat:unread:<user_id>` (hash channel → count), nguồn sự thật PostgreSQL; cập nhật đa thiết bị/màn hình qua `chat.unread.updated`.

### 8.3. Lịch sử & đồng bộ

| Chức năng | API / cơ chế |
|---|---|
| Danh sách channel | `GET /api/v1/chat/channels` — kèm `unread_count`, `unread_mentions`, `last_message` preview (đã lọc), `pinned_count` |
| Lịch sử | `GET /api/v1/chat/channels/{id}/messages?before_seq=&after_seq=&limit=50` — hợp nhất PostgreSQL với `ew:chat:recent` để che độ trễ persister; tin thu hồi/gỡ trả placeholder |
| Pins | `GET /api/v1/chat/channels/{id}/pins` |
| Đồng bộ khi reconnect / seq gap | WS `chat.channel.sync {channel_id: last_seen_seq}` → server trả tin thiếu (≤ 200), nhiều hơn → client tải lịch sử REST |
| Quyền xem lịch sử | Member chỉ xem tin từ `joined_at` [Confirmed]; không có vai trò nhân sự nào xem lịch sử channel ngoài moderation case |

### 8.4. Tìm kiếm

- `GET /api/v1/chat/search?q=&channel_id=&sender_id=&from=&to=&cursor=` — chỉ trong channel người dùng **đang là thành viên**, luôn lọc `tenant_id`, `deleted_at IS NULL`, `recalled_at IS NULL`, `moderation_state IN ('allowed','masked')`.
- PostgreSQL full-text: cột `search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', unaccent(lower(body_display))))` + GIN index; tìm có dấu hoặc không dấu đều khớp. Trigram (`pg_trgm`) cho truy vấn ngắn ≤ 3 ký tự [Confirmed].
- Tìm trên **nội dung đã che** (`body_display`), không bao giờ khớp nội dung gốc bị che.
- Không có ở world/area; dữ liệu gameplay đã purge không còn tìm được (ADR-0033).
- Phạm vi F120: channel bền vững `team` cố định (club) và `direct`, tin từ `joined_at` của người gọi; bỏ tin `anonymized_at IS NOT NULL`, tin `held`/`removed` và tin của người block/bị block; `q` 2–100 ký tự, tối đa 8 từ ghép tiền tố (`từ:*`), phân trang cursor `(created_at, message_id)`. Trigram cho truy vấn ngắn chưa bật (tiền tố tsquery thay thế) [Proposed].
- `safety_moderator` dùng công cụ tìm kiếm riêng, chỉ trong phạm vi case ([MODERATION](MODERATION.md) §6.1).

### 8.5. Quick messages

30–60 câu chia nhóm (chào hỏi, phối hợp quest "Đi theo mình!" / "Follow me!", khen ngợi, học tập "Mình cần gợi ý" / "I need a hint", cảm ơn); quản lý trong CMS, phiên bản hóa; mỗi câu là `localized_text` **bắt buộc đủ `vi` và `en`** [Confirmed — ADR-0056]. Client gửi `quick_message_id` (+ tham số); **mỗi người nhận thấy câu theo ngôn ngữ của chính mình** (người gửi chọn tiếng Việt, người nhận chọn tiếng Anh thì người nhận thấy bản tiếng Anh); tham số và ngữ cảnh vẫn qua pipeline.

### 8.6. Ngôn ngữ trong chat (ADR-0056)

- **Không dịch máy** tin nhắn tự do giữa người chơi (riêng tư, tránh sai nghĩa); tin hiển thị đúng nguyên văn người gửi viết.
- Quick message, tin hệ thống (`kind = system`: "Bạn A đã vào party"), nhãn kiểm duyệt ("Tin nhắn đã được thu hồi") gửi dưới dạng **mã + tham số** và client hiển thị theo ngôn ngữ của người xem.
- Pipeline kiểm duyệt chạy **cả danh mục `vi` và `en`** trên mọi tin, không phụ thuộc ngôn ngữ giao diện của người gửi (học sinh có thể gõ tiếng Anh khi đang chọn giao diện tiếng Việt và ngược lại); danh mục theo `locale` do [MODERATION §11](MODERATION.md) sở hữu.
- Sticker/GIF có nhãn và tag song ngữ để tìm kiếm ở cả hai ngôn ngữ.

## 9. Mute, block, report (phía chat)

| Hành động | Hiệu lực | API |
|---|---|---|
| Mute user (cá nhân) | Ẩn tin của người đó với mình; người kia không biết | `POST /api/v1/social/mutes` |
| Mute channel | Không thông báo | `PATCH /api/v1/chat/channels/{id}/membership {notification_level}` |
| Block | Ẩn tin hai chiều trong channel chung, chặn DM, lời mời, friend request | `POST /api/v1/social/blocks` |

Ẩn tin do **server** làm ở mọi loại channel, kể cả World/Area (không có danh sách thành viên — lọc theo quan hệ của người gửi khi fan-out và của người xem khi trả lịch sử, §2 bảng membership); client không phải nguồn quyết định.
| Report message / user | Tạo `report` → [MODERATION](MODERATION.md); báo cáo được cả tin **đã thu hồi** hoặc **đã sửa** (moderator thấy bản gốc và các revision) | `POST /api/v1/chat/reports` |

Quy tắc quan hệ block/mute (hiệu lực ngoài chat: lời mời, ghép trận, instance) thuộc [SOCIAL_SYSTEM §5](SOCIAL_SYSTEM.md#5-block-mute-report-điểm-vào).

## 10. Notifications

- Module notification của module chat: **chỉ in-app** (WS `notification.created`, hộp thông báo, badge). **Không push ngoài game cho học sinh trong Release 1.0** [Confirmed] — thống nhất với [SOCIAL_SYSTEM §8](SOCIAL_SYSTEM.md#8-thông-báo-xã-hội) và nguyên tắc chống kéo trẻ quay lại game. Push (nếu sau này có) cần ADR mới: payload không chứa nội dung tin, không gửi ngoài giờ trường cho phép.
- `notification_level`: `all` / `mentions` / `none`. Mặc định: party/team/match/direct `all`. Thông báo trường (chỉ bài giao và kết quả bài giao) đi qua bảng tin của trường ([SOCIAL_SYSTEM §8](SOCIAL_SYSTEM.md#8-thông-báo-xã-hội)), không qua chat.

## 11. Mở rộng voice chat (chỉ thiết kế; không làm trong Release 1.0, ADR-0070)

Chuẩn bị sẵn để không phải đổi model:
- `channel.capabilities` có cờ `voice` (luôn `false` trong Release 1.0, ADR-0070); `ChatPolicy` có field `voice` (deny mặc định).
- Signaling dự kiến qua WS (`voice.session.*`), media qua SFU WebRTC bên ngoài (LiveKit/mediasoup) — cần ADR mới, đánh giá an toàn (ghi âm? kiểm duyệt realtime?) và pháp lý trước khi bật.
- UI Godot dành chỗ cho nút mic trong chat panel nhưng ẩn bằng feature flag.

## 12. Metrics

`eduworld_chat_messages_total{channel_type,kind,decision}`, `eduworld_chat_filter_latency_seconds`, `eduworld_chat_delivery_latency_seconds`, `eduworld_chat_rejected_total{code}`, `eduworld_chat_reports_total{reason}`, `eduworld_chat_active_channels`, `eduworld_chat_sensitive_matches_total{category,action}`, `eduworld_chat_unchecked_messages_total{tenant}`, `eduworld_chat_policy_disabled{tenant,step}` (gauge, alert khi = 1), `eduworld_chat_sensitive_catalog_version{scope}`, `eduworld_chat_send_latency_seconds` (nhận lệnh → `acked`), `eduworld_chat_persist_lag_seconds` (publish → commit PG), `eduworld_chat_jetstream_publish_errors_total`, `eduworld_chat_seq_gaps_total` (client báo), `eduworld_chat_events_total{event}` (edit/recall/forward/reaction/pin), `eduworld_chat_search_latency_seconds`. JetStream consumer pending/redelivered lấy từ `prometheus-nats-exporter`. Không gắn label nội dung hay user id.

## 13. Acceptance criteria (Master Prompt §22 — Chat)

Mã `AC-CHAT-NNN` chỉ do [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) cấp (ADR-0082). Tiêu chí chi tiết của tài liệu này dùng tiền tố cục bộ `AC-CHS-NNN` và ghi AC của PRD mà nó chi tiết hóa (một AC PRD có thể có nhiều dòng).

| ID | AC (PRD) | Tiêu chí chi tiết |
|---|---|---|
| AC-CHS-001 | AC-CHAT-001 | Tạo channel: tạo party → channel `party` tồn tại, mọi member có membership |
| AC-CHS-002 | AC-CHAT-001 | ~~Tham gia channel lớp~~ — Removed (ADR-0095): không có channel `class`/`study_group`; mở channel `class` bị từ chối |
| AC-CHS-003 | AC-CHAT-002 | Gửi/nhận: tin text hợp lệ ở party tới mọi member online ≤ 250 ms p95 (cùng region) |
| AC-CHS-004 | AC-CHAT-003 | Cùng một tin text hợp lệ, học sinh lớp 2 và lớp 8 gửi ở cùng loại channel nhận cùng kết quả `allow`; cùng một tin vi phạm (tục tĩu/PII) bị chặn như nhau cho cả hai — không có nhánh xử lý theo lớp |
| AC-CHS-005 | AC-CHAT-004 | Tin chứa số điện thoại bị chặn, người nhận không nhận được |
| AC-CHS-006 | AC-CHAT-005 | Block: sau khi A block B, B không DM/mời A được, A không thấy tin B trong party chung |
| AC-CHS-007 | AC-CHAT-006 | Report tin nhắn tạo `report` và `moderation_case` trong hàng đợi Platform Safety Team, gắn đúng tenant và trường của học sinh bị báo cáo (ADR-0072) |
| AC-CHS-008 | AC-CHAT-007 | Không truy cập channel trái phép: user không phải member gọi history → 404 |
| AC-CHS-009 | AC-CHAT-007 | Không đọc tin trường khác: học sinh hoặc nhân sự trường A truy vấn channel trường B → 404, RLS chặn ở DB khi bỏ qua tầng app (test trực tiếp repository); `safety_moderator` chỉ đọc được qua API moderation case |
| AC-CHS-010 | AC-CHAT-011 | Gửi lại cùng `client_message_id` sau reconnect không tạo tin trùng |
| AC-CHS-011 | AC-CHAT-001 | Policy trường không thể nới lỏng quá platform (API trả `CHAT_POLICY_RELAX_NOT_ALLOWED`) |
| AC-CHS-012 | AC-CHAT-003 | Lưu policy có trường `grade_band`/`grade_level` bị từ chối (validation schema) |
| AC-CHS-013 | AC-CHAT-005 | DM chỉ gửi được khi hai người là bạn bè và không block nhau; ngược lại `CHAT_NOT_ALLOWED` |
| AC-CHS-014 | AC-CHAT-004 | Quick message có tham số chứa PII bị chặn như tin text |
| AC-CHS-015 | AC-CHAT-003 | Tin chứa term thuộc category đang bật bị xử lý theo action của danh mục; phản hồi chỉ có `category_code`, không có từ đã khớp |
| AC-CHS-016 | AC-CHAT-003 | Khi `enabled_all` của tenant tắt, tin không vi phạm PII/grooming vẫn được gửi với decision `unchecked`; tin có SĐT/mạng xã hội vẫn bị chặn (ADR-0078); vẫn tuân giới hạn độ dài, rate limit, quyền channel và sanction |
| AC-CHS-017 | AC-CHAT-008 | Trận 1v1 bắt đầu → channel `match` tồn tại, cả hai người chơi là member và gửi/nhận text được ngay, không cần bật |
| AC-CHS-018 | AC-CHAT-008 | Team battle → mỗi đội có channel `team` riêng (đội kia không đọc được) và một channel `match` chung |
| AC-CHS-019 | AC-CHAT-008 | Tạo party → channel `party` có ngay (tạo từ event, ≤ 1 s); người vào party sau tự thành member; giải tán → channel read-only |
| AC-CHS-020 | AC-CHAT-009 | Policy trường/lớp đặt `enabled:false`, bỏ `text`, đặt `allowed_hours` hoặc slow mode > 10 s cho `match`/`team`/`party` → bị từ chối `CHAT_POLICY_ALWAYS_ON` |
| AC-CHS-021 | AC-CHAT-008 | Người chơi đổi thiết bị giữa trận (ADR-0031) vẫn ở trong channel `match` và nhận tin bỏ lỡ qua resume |
| AC-CHS-022 | AC-CHAT-008 | Trận kết thúc → channel `match` read-only: gửi tin trả `CHAT_CHANNEL_CLOSED` |
| AC-CHS-023 | AC-CHAT-006 | Học sinh bị mute (sanction) vẫn vào trận/party, không gửi được tin; người khác vẫn chat bình thường |
| AC-CHS-024 | AC-CHAT-010 | Channel `match` đóng ngày D → purge đúng sau `gameplay_chat_retention_days` (mặc định 7); trước hạn vẫn xem được lịch sử, sau hạn API trả 404 |
| AC-CHS-025 | AC-CHAT-010 | Tin trong channel gameplay có report trước hạn purge được sao sang `moderation_evidence` và còn xem được trong case sau khi channel bị purge |
| AC-CHS-026 | AC-CHAT-010 | Channel có moderation case đang mở không bị purge cho đến khi case đóng |
| AC-CHS-027 | AC-CHAT-010 | Job purge không xóa bất kỳ dòng nào của `team` cố định, `direct` |
| AC-CHS-028 | AC-CHAT-010 | Cấu hình `gameplay_chat_retention_days` < 3 bị từ chối |
| AC-CHS-029 | AC-CHAT-011 | Người gửi chỉ nhận `chat.message.acked` sau khi JetStream ack; giả lập NATS lỗi → `CHAT_TEMPORARILY_UNAVAILABLE`, không người nhận nào thấy tin |
| AC-CHS-030 | AC-CHAT-011 | PostgreSQL ngừng 10 phút: gửi/nhận realtime vẫn chạy; sau khi DB trở lại, mọi tin trong khoảng đó có trong lịch sử, không trùng, đúng `seq` |
| AC-CHS-031 | AC-CHAT-002 | Bỏ một gói Valkey Pub/Sub: client phát hiện seq gap và bù đủ tin qua `chat.channel.sync`/lịch sử |
| AC-CHS-032 | AC-CHAT-012 | Sửa tin trong 15': người khác thấy bản mới + nhãn "đã chỉnh sửa"; sau 15' → `CHAT_EDIT_WINDOW_EXPIRED`; bản sửa vi phạm bị chặn và bản cũ giữ nguyên |
| AC-CHS-033 | AC-CHAT-012 | Thu hồi trong 24 h: mọi người thấy placeholder; `safety_moderator` vẫn xem được nội dung gốc trong case; tin đã thu hồi vẫn báo cáo được |
| AC-CHS-034 | AC-CHAT-012 | Quote/reply hiển thị bản đã che; tin gốc thu hồi → "Tin nhắn gốc không còn" |
| AC-CHS-035 | AC-CHAT-013 | Forward sang channel khác tenant hoặc channel không có quyền gửi → `CHAT_FORWARD_NOT_ALLOWED`; forward hợp lệ chạy lại kiểm duyệt của channel đích |
| AC-CHS-036 | AC-CHAT-013 | Sticker của pack chưa mở khóa / chưa gán cho tenant bị từ chối; GIF chỉ lấy từ thư viện nội bộ |
| AC-CHS-037 | AC-CHAT-002 | Mention > 5 người → `CHAT_MENTION_LIMIT`; mention tới người không hợp lệ bị loại khỏi tin (không lỗi); người được mention nhận `chat.mention.created` và `unread_mentions` tăng; mention dùng nickname, không lộ username |
| AC-CHS-038 | AC-CHAT-012 | Pin tin thứ 4 → `CHAT_PIN_LIMIT_REACHED`; thu hồi tin đã pin → tự bỏ ghim |
| AC-CHS-039 | AC-CHAT-014 | Typing không ghi DB, không gửi tới người đã block, tự hết sau 5 s |
| AC-CHS-040 | AC-CHAT-014 | Receipts: trong party 4 người, người gửi thấy chính xác ai đã nhận/đã xem (không còn channel class đếm số — ADR-0095) |
| AC-CHS-041 | AC-CHAT-002 | Unread count và mention count đúng sau khi đọc trên thiết bị khác (cập nhật qua `chat.unread.updated`) |
| AC-CHS-042 | AC-CHAT-014 | Tìm "nhan vat" khớp tin "nhân vật"; không trả tin đã thu hồi/bị chặn/khác tenant/channel không phải thành viên; không khớp nội dung gốc đã bị che |
| AC-CHS-043 | AC-CHAT-012 | Copy tin đã che chỉ copy bản đã che |
| AC-CHS-044 | AC-CHAT-001 | Trong giờ giới nghiêm `platform_curfew`, gửi tin ở mọi channel bị từ chối; `school_admin` đặt giờ chat rộng hơn giờ ngoài giới nghiêm → `CHAT_POLICY_RELAX_NOT_ALLOWED` (ADR-0078) |
| AC-CHS-045 | AC-CHAT-001 | Trường chọn preset `quick_message_emote_only` → text tự do, sticker, GIF bị từ chối ở mọi channel của trường kể cả `party`/`match`; quick message và emote gửi được; channel gameplay vẫn tự tạo |
| AC-CHS-046 | AC-CHAT-005 | Tenant `common`: gửi friend request, mở DM, gửi tin `world` → `CHAT_NOT_ALLOWED`/bị từ chối; System Admin bật lại có audit |
