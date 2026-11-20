# Feature Specification: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Feature Branch**: `100-party-management` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` · **Mốc**: M4a · **Phụ thuộc**: F095
**Input**: Học sinh lập nhóm chơi tạm thời (party): tạo, mời bạn, chấp nhận/từ chối/hủy lời mời, tham gia party công khai trong lớp/bạn bè, rời, kick, chuyển leader, giải tán, sửa cài đặt; một học sinh tối đa một party active; giới hạn theo trường/lớp/khối/block/sanction; chống spam lời mời; audit. FR nguồn: FR-PTY-001…005, FR-PTY-009…011, FR-PTY-014.

## Clarifications

### Session 2026-11-16

- Q: Sự kiện party tới client đi qua topic `ew:rt:party:<party_id>` hay `ew:rt:user:<id>`? → A: Fan-out tới `ew:rt:user:<member_id>` của từng thành viên (≤ 8); topic party cần lệnh WS để subscribe và không gỡ được khi bị kick — xem [research.md](research.md) R1 [Confirmed].
- Q: Mã lỗi khi mời người đã block/bị block? → A: `403 PARTY_INVITE_NOT_ALLOWED` (không tiết lộ việc block, theo PARTY_SYSTEM §14); vi phạm trường/lớp/khối/sanction → `403 PARTY_POLICY_VIOLATION` + audit. API_DESIGN §5 đã sửa theo (RB-405).
- Q: Tên party tự đặt kiểm duyệt ở đâu? → A: Gọi facade `chat` (`moderateDisplayText`, trong tiến trình) theo MODERATION §1; module chat không trả lời được → dùng tên mặc định sinh tự động, không chặn tạo party (research R2).
- Q: Channel chat `party` tạo "cùng transaction" (PARTY_SYSTEM §2)? → A: Không khả thi giữa hai database (ADR-0037); module chat tạo channel khi tiêu thụ `party.created` (F117). F100 chỉ phát event.
- Q: `party_policies` cấp trường/lớp ai cấu hình? → A: F100 chỉ đọc (mặc định platform từ cấu hình + bản ghi `party_policies` nếu có); API/UI quản trị chưa có feature trong danh mục (ghi ở Ngoài phạm vi).

### Session 2026-11-17

- Q: Xác nhận sau đối soát (RB-405, RB-302)? → A: Block → `403 PARTY_INVITE_NOT_ALLOWED`; `PARTY_ACTIVITY_NOT_ELIGIBLE` là `409`; `party_invite_rate` = 10/phút, **50/ngày** mỗi học sinh ([CONFIG_DEFAULTS](../../docs/00-overview/CONFIG_DEFAULTS.md) C-3); nickname `maxLength` 32.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tạo party và xem party của mình (Priority: P1)

Học sinh A (chưa có party) tạo nhóm từ HUD hoặc danh sách bạn bè, trở thành leader; xem lại party của mình bất cứ lúc nào (US-PTY-001).

**Why this priority**: Mọi thao tác party khác cần party tồn tại và bất biến "một party active".

**Independent Test**: `createParty` bằng token `hs.an.3a` → `201`, A là leader; gọi lại cùng `Idempotency-Key` → cùng party; tạo party thứ hai → `409 PARTY_ALREADY_IN_PARTY`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F100-AC-01 | KHI học sinh chưa có party active gọi `createParty` hợp lệ, module competition PHẢI tạo party `status = open` với người gọi là leader, `tenant_id`/`school_id`/`grade_band` lấy từ hồ sơ học sinh của phiên (không từ body), giá trị mặc định `visibility = private`, `invitation_policy = leader_only`, `max_members` theo `party_max_members`, trả `201` và phát `party.created` | FR-PTY-001 · FR-PTY-002 · FR-PTY-004 · AC-PTY-001 | TC-PTY-001 |
| F100-AC-02 | NẾU học sinh đã là thành viên một party active (kể cả khi hai request tạo/tham gia đến đồng thời), THÌ module competition PHẢI trả `409 PARTY_ALREADY_IN_PARTY` và không tạo membership thứ hai | FR-PTY-005 · AC-PTY-010 | TC-PTY-012 |
| F100-AC-03 | NẾU `createParty`/`inviteToParty`/`acceptPartyInvitation`/`joinParty`/`transferPartyLeader` thiếu `Idempotency-Key`, THÌ module competition PHẢI trả `428 PRECONDITION_REQUIRED`; gửi lại cùng key PHẢI trả cùng kết quả, không ghi lần hai | FR-PTY-005 · AGENTS R10 | — |
| F100-AC-04 | NẾU `name` rỗng hoặc dài hơn 40 ký tự, `max_members` ngoài 2–8 hoặc vượt `max_members_cap` của policy, hoặc `party_type` không được phép cho grade band của học sinh, THÌ module competition PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường | FR-PTY-003 · FR-PTY-004 · ADR-0041 | TC-TEN-024 |
| F100-AC-05 | KHI tên party do học sinh tự đặt, module competition PHẢI kiểm tra qua pipeline kiểm duyệt của module chat trước khi lưu; NẾU bị chặn, THÌ PHẢI trả `422 VALIDATION_FAILED` (`name`) không kèm từ đã khớp; NẾU module chat không phản hồi, THÌ PHẢI dùng tên mặc định sinh tự động | FR-PTY-002 · AGENTS R15 · [MODERATION §1](../../docs/03-social-multiplayer/MODERATION.md) | — |
| F100-AC-06 | KHI thành viên gọi `getMyParty`/`getParty`, module competition PHẢI trả party kèm danh sách thành viên chỉ gồm `student_id`, nickname, avatar, vai trò, `joined_at` (không tên thật, trường, lớp); người không phải thành viên hoặc party của tenant/trường khác PHẢI nhận `404 PARTY_NOT_FOUND` | FR-PTY-002 · AGENTS R16 · NFR-SEC-006 | TC-TEN-001 |

---

### User Story 2 — Mời bạn, nhận/từ chối/hủy lời mời (Priority: P1)

A mời B, C, D (bạn bè cùng trường); B, C, D nhận lời mời realtime (hoặc thấy khi vào game), chấp nhận để vào nhóm (US-PTY-001).

**Why this priority**: Party riêng tư chỉ có thể có thành viên qua lời mời; đây là đường vào chính.

**Independent Test**: A mời B, C, D → 3 lời mời `pending`, mỗi người nhận `party.invitation.received`; B, C accept → 3 thành viên; với `max_members = 3`, D accept → `409 PARTY_FULL`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F100-AC-07 | KHI leader (hoặc thành viên khi `invitation_policy = members`) gọi `inviteToParty` cho học sinh thỏa policy, module competition PHẢI tạo lời mời `pending` có `expires_at` = thời điểm server + `party_invite_ttl`, phát `party.invitation_sent` và đẩy `party.invitation.received` tới invitee | FR-PTY-001 · FR-PTY-009 · AC-PTY-002 | TC-PTY-002 |
| F100-AC-08 | NẾU người mời không được phép theo `invitation_policy`, hoặc hai học sinh block nhau (một chiều bất kỳ), THÌ module competition PHẢI trả `403 PARTY_INVITE_NOT_ALLOWED` không tiết lộ lý do block | FR-PTY-009 · [PARTY_SYSTEM §14](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-011 |
| F100-AC-09 | NẾU invitee khác trường (trừ tenant `unit` bật liên trường), khác tenant, đang bị sanction `restrict_multiplayer`, hoặc vi phạm `party_policies` (lớp, grade band, loại party; lớp 1–3 chỉ mời bạn cùng lớp), THÌ module competition PHẢI trả `403 PARTY_POLICY_VIOLATION` và ghi audit | FR-PTY-010 · ADR-0054 · [PARTY_SYSTEM §8](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-011 |
| F100-AC-10 | NẾU người mời vượt `party_invite_rate`, hoặc mời lại người vừa từ chối trong thời gian chờ, hoặc đã có lời mời `pending` tới cùng người cho party đó, THÌ module competition PHẢI trả `429 PARTY_INVITE_RATE_LIMITED` (hoặc `409 CONFLICT` với lời mời trùng) và ghi audit khi vượt rate | FR-PTY-011 · AC-PTY-011 | TC-PTY-013 |
| F100-AC-11 | KHI invitee gọi `acceptPartyInvitation` cho lời mời `pending` còn hạn, module competition PHẢI kiểm tra lại toàn bộ policy tại thời điểm accept, thêm thành viên, đặt lời mời `accepted`, phát `party.invitation_accepted` + `party.member_joined` và đẩy `party.member.joined` tới mọi thành viên; gọi lại PHẢI không tạo thành viên trùng | FR-PTY-001 · FR-PTY-005 · AC-PTY-002 | TC-PTY-003 |
| F100-AC-12 | NẾU party đã đủ `max_members` (kể cả khi nhiều lời mời được accept đồng thời), THÌ module competition PHẢI trả `409 PARTY_FULL` cho phần vượt; NẾU lời mời quá `expires_at`, THÌ PHẢI trả `410 PARTY_INVITATION_EXPIRED`; NẾU người gọi không phải invitee, THÌ PHẢI trả `403 PERMISSION_DENIED` | FR-PTY-004 · FR-PTY-009 · AC-PTY-010 | TC-PTY-003 |
| F100-AC-13 | KHI invitee gọi `rejectPartyInvitation` hoặc inviter/leader gọi `cancelPartyInvitation`, module competition PHẢI đặt `rejected`/`cancelled`, phát event tương ứng và đẩy `party.invitation.updated` tới bên còn lại; người khác gọi `cancelPartyInvitation` PHẢI nhận `403 PERMISSION_DENIED` | FR-PTY-001 | TC-PTY-014 |
| F100-AC-14 | TRONG KHI lời mời `pending`, job `party-invitation-expiry` PHẢI chuyển sang `expired` khi quá `expires_at`, khi party giải tán, khi invitee vào party khác, hoặc khi invitee vượt số lời mời đang chờ tối đa (lời mời cũ nhất), và phát `party.invitation_expired` | FR-PTY-009 · [PARTY_SYSTEM §10](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | — |
| F100-AC-15 | KHI invitee gọi `listMyInvitations?status=pending`, module competition PHẢI chỉ trả lời mời gửi tới chính người gọi, thông tin inviter chỉ gồm nickname/avatar | FR-PTY-014 · AGENTS R16 | TC-TEN-001 |

---

### User Story 3 — Tham gia, rời, kick, chuyển leader, giải tán, sửa cài đặt (Priority: P1)

Bạn cùng lớp tham gia party `visibility = class`; thành viên rời nhóm; leader kick, chuyển leader, sửa cài đặt hoặc giải tán.

**Why this priority**: Vòng đời party đầy đủ là điều kiện để lobby và phiên chơi (F101) hoạt động đúng.

**Independent Test**: B `joinParty` party class của A → thành viên; A kick B → B không được mời lại trong `party_kick_reinvite_block`; A chuyển leader cho C (online) → C leader; C disband → mọi thành viên rời, lời mời pending hết hạn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F100-AC-16 | KHI học sinh gọi `joinParty` cho party `visibility = class` (cùng lớp) hoặc `friends` (là bạn của leader) và thỏa policy, module competition PHẢI thêm thành viên như F100-AC-11; NẾU party `private` và không có lời mời `accepted` còn hạn, THÌ PHẢI trả `403 PARTY_POLICY_VIOLATION` | FR-PTY-001 · FR-PTY-009 · AC-PTY-010 | TC-PTY-011 |
| F100-AC-17 | KHI thành viên gọi `leaveParty`, module competition PHẢI đặt `left_at`, `left_reason = left`, phát `party.member_left`; NẾU người rời là leader, THÌ PHẢI chuyển leader cho thành viên online gia nhập sớm nhất (không ai online → gia nhập sớm nhất) với reason `leader_left`; NẾU không còn thành viên, THÌ PHẢI giải tán party với reason `all_left` | FR-PTY-001 · [PARTY_SYSTEM §9](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-014 |
| F100-AC-18 | KHI leader gọi `kickPartyMember`, module competition PHẢI loại thành viên (`left_reason = kicked`), hủy các lời mời do người đó gửi, chặn mời lại người đó vào party này trong `party_kick_reinvite_block`, phát `party.member_kicked` và ghi audit; người không phải leader PHẢI nhận `403 PARTY_NOT_LEADER` | FR-PTY-001 · FR-PTY-011 · AC-PTY-011 | TC-PTY-014 |
| F100-AC-19 | KHI leader gọi `transferPartyLeader` tới thành viên đang online, module competition PHẢI đổi leader trong một transaction (đúng một leader active), phát `party.leader_transferred` (reason `manual`) và ghi audit; NẾU đích không online, THÌ PHẢI trả `409 PARTY_MEMBER_NOT_ONLINE` | FR-PTY-001 · FR-PTY-011 | TC-PTY-014 |
| F100-AC-20 | KHI leader gọi `disbandParty`, module competition PHẢI đặt `status = disbanded`, `disbanded_at`, cho mọi thành viên rời (`disbanded`), chuyển lời mời `pending` sang `expired`, phát `party.disbanded` (reason `leader_disbanded`) và ghi audit; gọi lại PHẢI trả `204` không phát event lần hai | FR-PTY-001 · FR-PTY-011 · AC-PTY-011 | TC-PTY-014 |
| F100-AC-21 | KHI leader gọi `updatePartySettings`, module competition PHẢI cập nhật `name`/`visibility`/`invitation_policy`/`max_members` (≥ số thành viên hiện tại, ≤ cap), phát `party.settings_updated` và đẩy `party.state.snapshot`; người không phải leader PHẢI nhận `403 PARTY_NOT_LEADER` | FR-PTY-001 · FR-PTY-004 | TC-PTY-014 |
| F100-AC-22 | HỆ THỐNG PHẢI ghi mỗi thay đổi trạng thái party (tạo, thành viên vào/ra/kick, chuyển leader, cài đặt, giải tán) vào lịch sử party append-only trong cùng transaction với thay đổi | FR-PTY-011 · [PARTY_SYSTEM §12.3](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | — |

---

### User Story 4 — Chỉ học sinh, đúng tenant, an toàn (Priority: P1)

Nhân sự không vào được API party; học sinh tenant/trường khác không thấy party; các ràng buộc bảo vệ học sinh không bị vượt qua bằng party.

**Why this priority**: Điều III, V, VI của constitution không được phép vi phạm.

**Independent Test**: Sinh test từ OpenAPI: mọi route `/multiplayer/parties*`, `/multiplayer/invitations*` × token nhân sự → `403`; `hs.dung.7a` (tenant khác) gọi `getParty` party của `hs.an.3a` → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F100-AC-23 | HỆ THỐNG PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` cho mọi operation của feature khi token không phải token game của học sinh (`school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin`) | AGENTS R23 · ADR-0031 | TC-AUTH-014 |
| F100-AC-24 | NẾU `X-Tenant-Id` khác tenant của phiên, THÌ module competition PHẢI trả `403 TENANT_MISMATCH`; mọi truy vấn party/lời mời PHẢI lọc theo tenant của phiên và `deleted_at IS NULL`, ID của tenant khác PHẢI trả `404` | AGENTS R8 · ADR-0065 | TC-TEN-011 |
| F100-AC-25 | KHI nhận `user.blocked` hoặc `sanction.applied` (`restrict_multiplayer`) liên quan thành viên party, module competition PHẢI hủy lời mời `pending` giữa hai bên/của người bị sanction và áp chặn cho lời mời/tham gia tiếp theo; thành viên bị `restrict_multiplayer` PHẢI bị loại khỏi party (`left_reason = policy`) | FR-PTY-009 · FR-PTY-010 · [PARTY_SYSTEM §8](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-CHAT-005 |

### Edge Cases

- NẾU hai học sinh accept lời mời của hai party khác nhau đồng thời, THÌ chỉ một thành công (F100-AC-02, unique partial index).
- NẾU invitee đang ở party khác khi accept, THÌ trả `409 PARTY_ALREADY_IN_PARTY`; client hỏi "Rời nhóm hiện tại và tham gia", gọi `leaveParty` rồi accept (không làm ngầm).
- NẾU leader bị kick hoặc rời khi party đang `in_activity`, THÌ chuyển leader như F100-AC-17; xử lý activity đang chạy thuộc F101.
- NẾU projection hồ sơ học sinh/bạn bè chưa có (event đến chậm), THÌ trả `503 UNAVAILABLE` có `Retry-After`, không ghi.
- NẾU Valkey lỗi sau commit, THÌ dữ liệu PG vẫn đúng; state realtime rebuild ở lần đọc kế tiếp, client bù bằng `getMyParty`.
- NẾU học sinh chuyển trường/khóa tài khoản (`student.synced`, `user.status_changed`), THÌ loại khỏi party với `left_reason = policy`.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-PTY-001 | Create, Join, Leave, Invite, Accept, Reject, Kick, Transfer Leader, Disband, Settings; phát event để module chat mở/đóng channel `party` (channel do F117) | [PARTY_SYSTEM §2](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-002 | Thuộc tính party trong response và DB | [PARTY_SYSTEM §3](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-003 | Loại party `friend`, `study_group`, `quest`, `competition_team`, `custom`; nhóm lớp = `visibility = class` | [PARTY_SYSTEM §4](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-004 | `max_members` cấu hình 2–8, mặc định theo `party_max_members` | [PARTY_SYSTEM §3](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-005 | Một party active/học sinh; chống duplicate membership | [DATABASE_DESIGN §3.7](../../docs/07-backend/DATABASE_DESIGN.md) |
| FR-PTY-009 | Party private cần lời mời; hết hạn lời mời; block | [PARTY_SYSTEM §8, §13.2](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-010 | Giới hạn trường/lớp/khối/loại party; mặc định cùng trường | [ADR-0054](../../docs/adr/ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) · [PARTY_SYSTEM §8](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-011 | Rate limit lời mời, audit create/kick/transfer/disband | [PARTY_SYSTEM §10, §11](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-014 | API `/api/v1/multiplayer/parties`, `/api/v1/multiplayer/invitations`; message realtime | [API_DESIGN §9.6](../../docs/07-backend/API_DESIGN.md) |

### Key Entities

- **Party**: nhóm chơi tạm thời trong một tenant/trường; leader, loại, visibility, chính sách mời, số thành viên tối đa, trạng thái `open`/`in_activity`/`disbanded`, phiên bản trạng thái.
- **PartyMember**: tư cách thành viên có thời điểm vào/ra và lý do ra; tối đa một membership active mỗi học sinh.
- **PartyInvitation**: lời mời có hạn; `pending`/`accepted`/`rejected`/`expired`/`cancelled`.
- **PartyPolicy**: giới hạn cấp trường/lớp (loại party, liên lớp, liên grade band, trần thành viên); cấp dưới chỉ siết chặt.
- **PartyHistory**: nhật ký append-only của party.
- **Projection**: hồ sơ học sinh (trường, lớp, grade band, trạng thái), hồ sơ người chơi (nickname, avatar), quan hệ bạn bè, block, sanction.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `party_max_members` | Mặc định và trần `max_members` |
| `party_invite_ttl` | `expires_at` của lời mời |
| `party_invite_rate` | Rate limit gửi lời mời |
| `party_kick_reinvite_block` | Chặn mời lại sau kick |
| `idempotency_record_ttl` | Lưu kết quả theo `Idempotency-Key` |
| `api_rate_default` | Rate limit chung của route |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Middleware `game_player` mọi route; không cửa sau cho nhân sự | AGENTS R23 · ADR-0031 |
| Party/lời mời ghi PostgreSQL trước, Valkey chỉ là state tạm rebuild được | AGENTS R9 · ADR-0004 |
| Chỉ soft-delete; giải tán dùng `status`, không xóa | AGENTS R21 · ADR-0028 |
| Không hiển thị tên thật/trường/lớp cho người chơi khác | AGENTS R16 · ADR-0054 |
| Không gọi đồng bộ module khác trừ kiểm duyệt tên qua facade (có fallback) | AGENTS R29 · ADR-0074 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường hợp học sinh có hai party active trong 100 lần chạy test đồng thời tạo/accept/join.
- **SC-002**: Invitee online nhận lời mời trong ≤ 1 giây (p95) sau khi người mời gửi.
- **SC-003**: 100% thao tác tạo/kick/chuyển leader/giải tán và vượt rate limit có bản ghi audit.
- **SC-004**: 0 response party/lời mời chứa tên thật, trường hoặc lớp của học sinh khác.

## Ngoài phạm vi

- Party Lobby (ready, chọn hoạt động), Start Session, presence/disconnect, chuyển leader tự động khi mất kết nối, tự giải tán khi dormant → F101.
- Giao diện party trong Godot → F102. Channel chat `party` (tạo/đóng/purge) → F117; giao diện chat → F119.
- Bạn bè, block (nguồn event) → F095; sanction `restrict_multiplayer` (nguồn event) → F121; pipeline kiểm duyệt tên → F116.
- Thông báo offline cho invitee → F123. `safety_moderator` giải tán party khi xử lý case → F121 (cần API moderation riêng; route người chơi không cho nhân sự).
- API/UI quản trị `party_policies` cấp trường/lớp: chưa có feature trong danh mục (đề xuất thêm vào F052 + endpoint `/tenant/party-policies`).
- Party tạo từ club (`club_id`) → F120; party `competition_team` vào matchmaking → F104.

## Assumptions

- Các giới hạn phụ của PARTY_SYSTEM §10 (chờ sau khi bị từ chối, 3 lần từ chối/ngày, tối đa lời mời đang chờ nhận, tạo/giải tán mỗi giờ, kick mỗi giờ) chưa có tên trong CONFIG_DEFAULTS; đề xuất `party_invite_repeat_rules`, `party_pending_invites_max`, `party_create_rate`, `party_kick_rate` (task Polish) [Confirmed].
- Tenant `unit` bật liên trường đọc từ `ref.tenants.settings_subset` (khóa tenant setting theo ADR-0054) [Confirmed].
- F095 phát `friend.added`/`friend.removed`, `user.blocked`/`user.unblocked`; F121 phát `sanction.applied`/`sanction.lifted`; module game phát `game_profile.updated` cho `ref.player_profiles`.
- F100 là feature đầu tiên của module competition nên dựng skeleton module (giống F014 với module game).
