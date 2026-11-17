# Feature Specification: F104 — Matchmaking 8 chế độ: queue, ticket, proposal, ready check, ràng buộc khối

**Feature Branch**: `104-matchmaking` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` · **Mốc**: M4b · **Phụ thuộc**: F090, F100
**Input**: Học sinh vào hàng đợi thi đấu (solo, đội từ party, thách đấu bạn), server ghép theo ràng buộc cứng (khối, nội dung, phạm vi trường, block, trạng thái) **trước** rating, rồi proposal → ready check → giao trận. FR nguồn: FR-MM-001…006, NFR-PERF-007; US-MM-001.

## Clarifications

### Session 2026-11-16

- Q: REST `acceptMatchProposal` dùng `{match_id}` trong khi WS dùng `proposal_id` — hai ID khác nhau? → A: Không. `proposal_id` là UUIDv7 cấp lúc tạo proposal và **được dùng làm `match_id`** khi mọi bên accept; `matches.match_id` là PK nên tạo trận idempotent theo proposal ([MATCHMAKING §6](../../docs/03-social-multiplayer/MATCHMAKING.md#6-matchmaker-worker)) [Proposed].
- Q: Bot luyện tập chạy ở đâu? → A: F104 đề nghị bot (sau `mm_bot_after`) và tạo trận bot unranked qua `startBotMatch`; luật chơi và hành vi bot do F105 thực thi.
- Q: Matchmaking `tournament` có queue không? → A: Không. F104 cung cấp command nội bộ `CreateDirectProposal` (proposal + ready check không qua queue) cho F107; ticket `matchmaking_type = tournament` bị từ chối.
- Q: Phạm vi liên trường đọc ở đâu? → A: Cài đặt tenant (`ref.tenants.settings_subset`), không nằm trong cấu hình matchmaking ([MATCHMAKING §5.1](../../docs/03-social-multiplayer/MATCHMAKING.md#51-cấu-hình-matchmaking--platform-only)).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Ghép 1v1 cùng khối theo rating (Priority: P1) 🎯 MVP

`hs.an.3a` chọn "Đấu Toán 1v1"; server tạo ticket, ghép với `hs.binh.3a` (cùng lớp 3, cùng trường), cả hai bấm "Sẵn sàng" trong ready check rồi được đưa vào phòng trận (US-MM-001).

**Why this priority**: Là đường vào duy nhất của mọi trận ranked; F105, F106 phụ thuộc.

**Independent Test**: Hai học sinh lớp 3 cùng trường tạo ticket `battle_1v1`/Toán → `matchmaking.proposal.created` cho cả hai trong ≤ 10 s; cả hai accept → `matchmaking.match.assigned` có `match_id` = `proposal_id`; bản ghi `matches` đúng một dòng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F104-AC-01 | KHI học sinh gọi `createMatchmakingTicket` hợp lệ, module competition PHẢI tự lấy khối, trường, lớp, tenant, rating (μ, RD) từ dữ liệu server (không nhận từ body), tạo ticket `queued` chỉ trong Valkey và đẩy `matchmaking.ticket.updated` | FR-MM-004 · FR-MM-006 · [MATCHMAKING §4](../../docs/03-social-multiplayer/MATCHMAKING.md#4-ticket) | TC-CMP-002 |
| F104-AC-02 | HỆ THỐNG PHẢI đưa ràng buộc cứng H1–H7 vào khóa queue và kiểm tra **trước** rating: lớp 1–5 cùng `grade_level`; lớp 6–9 cùng `grade_band` với nới ±1 khối theo `grade_window_g6_9`; không bao giờ ghép lớp 1–5 với khối khác, kể cả khi chờ quá `mm_bot_after` | FR-MM-002 · FR-MM-003 · AC-CMP-002 · [MATCHMAKING §2.1](../../docs/03-social-multiplayer/MATCHMAKING.md#21-hard-constraints-thành-phần-của-queue-key--không-nới-theo-thời-gian) | TC-MM-003 |
| F104-AC-03 | HỆ THỐNG PHẢI giới hạn phạm vi ghép cùng `school_id` theo mặc định; Ở NƠI tenant `unit` bật ghép liên trường, HỆ THỐNG PHẢI dùng phạm vi tenant; phạm vi platform chỉ dùng cho proposal do giải/sự kiện platform tạo; không bao giờ ghép hai tenant trong queue thường | FR-MM-003 · ADR-0054 | TC-TEN-012 |
| F104-AC-04 | TRONG KHI ticket chờ, matchmaker PHẢI ghép khi chênh lệch μ giữa hai ticket ≤ `window(t)` theo `mm_rating_window`, ưu tiên ticket chờ lâu nhất và cost nhỏ nhất (rating, RD, đối thủ gặp gần đây, cùng lớp); hết cửa sổ hữu hạn chỉ bỏ qua rating, không nới H1–H7 | FR-MM-005 · [MATCHMAKING §2.2](../../docs/03-social-multiplayer/MATCHMAKING.md#22-soft-scoring-trong-một-queue) | TC-MM-002 |
| F104-AC-05 | KHI matchmaker chọn cặp, HỆ THỐNG PHẢI tạo proposal với deadline `mm_ready_check`, gửi `matchmaking.proposal.created` chỉ chứa nickname, avatar, tier (không tên thật, trường, lớp); KHI mọi bên accept, HỆ THỐNG PHẢI tạo `Match` với `match_id = proposal_id` trong PostgreSQL rồi gửi `matchmaking.match.assigned` | FR-MM-004 · AC-CMP-001 · AGENTS R16 | TC-CMP-001 |
| F104-AC-06 | NẾU một bên decline hoặc hết `mm_ready_check`, THÌ HỆ THỐNG PHẢI hủy proposal, trả ticket của bên đã accept về queue giữ nguyên `enqueued_at` (`requeued = true`) và hủy ticket của bên decline/timeout; decline/timeout ≥ 3 lần trong 10 phút PHẢI kích hoạt cooldown theo `mm_ticket_rate` | AC-MM-004 · [MATCHMAKING §4.2](../../docs/03-social-multiplayer/MATCHMAKING.md#42-lifecycle) | TC-CMP-002 |
| F104-AC-07 | NẾU học sinh đã có ticket active (hoặc thuộc party đang có ticket), THÌ HỆ THỐNG PHẢI trả `409 MM_TICKET_ALREADY_ACTIVE` | AC-MM-003 · `mm_tickets_per_user` | — |
| F104-AC-08 | HỆ THỐNG PHẢI không đặt hai học sinh đã block nhau vào cùng proposal và PHẢI kiểm tra lại trạng thái tài khoản, hạn chế thi đấu (`abandon_penalty`, sanction `restrict_competition`) khi tạo ticket và khi tạo proposal; học sinh bị hạn chế tạo ticket PHẢI nhận `403 POLICY_DENIED` | AC-MM-005 · [MATCHMAKING §2.1](../../docs/03-social-multiplayer/MATCHMAKING.md#21-hard-constraints-thành-phần-của-queue-key--không-nới-theo-thời-gian) | — |
| F104-AC-09 | KHI nhiều instance worker chạy song song, HỆ THỐNG PHẢI xử lý mỗi queue bằng đúng một matchmaker (lock per-queue) và không tạo hai trận cho cùng ticket | AC-MM-006 | — |

---

### User Story 2 — Hủy, hết hạn, mất Valkey, bot luyện tập (Priority: P1)

Học sinh đổi ý rời hàng đợi; hàng đợi vắng thì sau một lúc được đề nghị "Đấu với Robot Học Giỏi"; Valkey khởi động lại thì client biết để tạo ticket mới.

**Why this priority**: Ticket chỉ ở Valkey (FR-MM-006) nên cần đường phục hồi rõ ràng; trải nghiệm chờ quyết định tỉ lệ bỏ cuộc.

**Independent Test**: Tạo ticket rồi `cancelMatchmakingTicket` → `cancelled`; tạo ticket khi queue rỗng, chờ `mm_bot_after` → `bot_offer_available = true`, gọi `startBotMatch` → trận `is_bot = true`, `ranked = false`; `FLUSHALL` Valkey → client nhận `cancel_reason = system`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F104-AC-10 | KHI chủ ticket gọi `cancelMatchmakingTicket` hoặc gửi WS `matchmaking.ticket.cancel`, HỆ THỐNG PHẢI xóa ticket khỏi queue, đẩy `matchmaking.ticket.updated` `cancelled` (`cancel_reason = user`); ticket chờ quá `mm_ticket_ttl` PHẢI chuyển `expired` và được dọn | FR-MM-004 · [MATCHMAKING §4.2](../../docs/03-social-multiplayer/MATCHMAKING.md#42-lifecycle) | TC-MM-001 |
| F104-AC-11 | NẾU Valkey mất dữ liệu ticket, THÌ HỆ THỐNG PHẢI gửi `matchmaking.ticket.updated` `cancelled` (`cancel_reason = system`) cho người chơi có kết nối và PHẢI không tạo trận nào trong PostgreSQL từ ticket đã mất | FR-MM-006 · AC-MM-008 · ADR-0004 | — |
| F104-AC-12 | KHI ticket chờ quá `mm_bot_after`, HỆ THỐNG PHẢI đặt `bot_offer_available = true`; KHI học sinh gọi `startBotMatch`, HỆ THỐNG PHẢI hủy ticket và tạo trận `is_bot = true`, `ranked = false` cùng môn/khối | FR-MM-005 · [MATCHMAKING §9](../../docs/03-social-multiplayer/MATCHMAKING.md#9-bot-luyện-tập-fallback) | — |
| F104-AC-13 | HỆ THỐNG PHẢI giới hạn tạo ticket theo `mm_ticket_rate`; vượt giới hạn hoặc đang cooldown PHẢI nhận `429 RATE_LIMITED` kèm `Retry-After` | [ANTI_CHEAT §5](../../docs/10-security/ANTI_CHEAT.md#5-rate-limit) | — |

---

### User Story 3 — Ticket đội từ party (Priority: P1)

Leader một party `competition_team` 3 người lớp 7 bấm "Tìm trận đội"; cả đội được ghép với đội khác cùng grade band; một thành viên rời party thì ticket đội bị hủy.

**Why this priority**: Team Battle là chế độ ranked thứ hai và là điểm nối party (F100) với thi đấu.

**Independent Test**: Party 2 người lớp 6 + leader tạo ticket `team_battle` → mọi member thỏa H1; member rời party → mọi member nhận `cancel_reason = party_changed`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F104-AC-14 | KHI leader party `competition_team` tạo ticket với `party_id`, HỆ THỐNG PHẢI kiểm tra người gọi là leader (`403 PARTY_NOT_LEADER`), mọi thành viên online (`409 PARTY_MEMBER_NOT_ONLINE`), không ai có ticket riêng, mọi thành viên thỏa H1–H7, và dùng `μ_team = Σμ/n`, `RD_team = sqrt(ΣRD²)/n` để ghép | AC-MM-007 · [MATCHMAKING §8](../../docs/03-social-multiplayer/MATCHMAKING.md#8-party-queue--friend-duel) | TC-MM-004 |
| F104-AC-15 | KHI thành viên rời party có ticket, HỆ THỐNG PHẢI hủy ticket và gửi `matchmaking.ticket.updated` `cancelled` (`cancel_reason = party_changed`) tới mọi thành viên | AC-MM-007 | TC-MM-004 |
| F104-AC-16 | HỆ THỐNG PHẢI giữ nguyên đội của party khi chia đội; ticket lẻ trong team mode chia theo snake draft theo rating; Ở NƠI leader bật `allow_fill`, HỆ THỐNG PHẢI cho phép ghép thêm ticket lẻ cùng queue để đủ đội | [MATCHMAKING §7–§8](../../docs/03-social-multiplayer/MATCHMAKING.md#7-match-proposal-ready-check-lobby-team-assignment) | TC-MM-004 |
| F104-AC-17 | NẾU khối của học sinh không có mode yêu cầu (vd. `team_battle` cho lớp < 4) hoặc party có thành viên khác khối yêu cầu mode ranked, THÌ HỆ THỐNG PHẢI trả `403 POLICY_DENIED` | [COMPETITION_SYSTEM §2, §5](../../docs/01-game-design/COMPETITION_SYSTEM.md#5-công-bằng-theo-khối-lớp) | — |

---

### User Story 4 — Tám kiểu matchmaking và thách đấu bạn (Priority: P2)

Học sinh chọn ghép ngẫu nhiên, theo kỹ năng, theo khối, theo môn, theo chủ đề, theo đội; hoặc thách đấu trực tiếp một bạn bè; giải đấu dùng proposal trực tiếp.

**Why this priority**: Đủ 8 chế độ theo FR-MM-001; không chặn luồng 1v1 chính.

**Independent Test**: `listCompetitionModes` cho học sinh lớp 3 không có `team_battle`; ticket `topic` thiếu `topic_id` → 422; `createDuel` tới bạn đã block → `SOCIAL_SCOPE_VIOLATION`; duel quá 60 s → `410 DUEL_EXPIRED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F104-AC-18 | HỆ THỐNG PHẢI hỗ trợ `matchmaking_type` `random` (FIFO, unranked), `skill` (expanding window, ranked), `grade` (`grade_level` chính xác mọi khối), `subject` (`subject_id` bắt buộc, rating theo môn), `topic` (`topic_id` bắt buộc, rating theo môn, unranked mặc định), `team` (ticket đội); `friend` chỉ qua duel và `tournament` chỉ qua proposal trực tiếp — ticket hai loại này PHẢI nhận `422 VALIDATION_FAILED` | FR-MM-001 · [MATCHMAKING §3](../../docs/03-social-multiplayer/MATCHMAKING.md#3-các-chế-độ-matchmaking) | — |
| F104-AC-19 | KHI học sinh gọi `createDuel` tới bạn bè cùng `grade_band`, không block nhau, HỆ THỐNG PHẢI tạo duel `pending` hết hạn sau 60 s và phát `duel.sent`; NẾU không phải bạn bè, khác grade band hoặc đã block, THÌ PHẢI trả `403 SOCIAL_SCOPE_VIOLATION` | FR-MM-001 · [MATCHMAKING §8](../../docs/03-social-multiplayer/MATCHMAKING.md#8-party-queue--friend-duel) | TC-CMP-010 |
| F104-AC-20 | KHI đối thủ gọi `acceptDuel` trước hạn, HỆ THỐNG PHẢI tạo trận `ranked = false` (không rating, không Competition Points) qua lobby; NẾU đã quá hạn hoặc bị hủy, THÌ PHẢI trả `410 DUEL_EXPIRED`; `declineDuel` PHẢI chuyển `declined` và phát `duel.declined` | TC-CMP-010 · [EVENT_DESIGN §4.8](../../docs/07-backend/EVENT_DESIGN.md#48-competition) | TC-CMP-010 |
| F104-AC-21 | KHI F107 yêu cầu trận giải cho hai entry, HỆ THỐNG PHẢI tạo proposal trực tiếp (không qua queue) có `tournament_node_id`, dùng cùng ready check và tạo trận idempotent theo `proposal_id` | [MATCHMAKING §3](../../docs/03-social-multiplayer/MATCHMAKING.md#3-các-chế-độ-matchmaking) · [COMPETITION_SYSTEM §10.6](../../docs/01-game-design/COMPETITION_SYSTEM.md#106-lịch-thi-đấu) | — |
| F104-AC-22 | KHI học sinh gọi `listCompetitionModes`, HỆ THỐNG PHẢI trả các mode/kiểu ghép khả dụng theo khối, môn của học sinh và cấu hình matchmaking đang hiệu lực | FR-MM-001 | — |

---

### User Story 5 — Phân quyền, cô lập tenant, idempotency, hiệu năng (Priority: P1)

Chỉ học sinh vào hàng đợi; dữ liệu một tenant không lẫn sang tenant khác; gửi lại request không tạo ticket trùng; thời gian chờ đạt mục tiêu.

**Why this priority**: Điều III, IV, V của constitution không được vi phạm.

**Independent Test**: `ad.truong.a` gọi `createMatchmakingTicket` → `403 GAME_ACCESS_STUDENT_ONLY`; `hs.an.3a` gọi `getMatchmakingTicket` của `hs.binh.3a` → 404; gửi lại cùng `Idempotency-Key` → cùng `ticket_id`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F104-AC-23 | HỆ THỐNG PHẢI chỉ cho principal qua middleware `game_player` (`azp = eduworld-game-backend`, `user_type = student`) gọi route matchmaking/duel; nhân sự PHẢI nhận `403 GAME_ACCESS_STUDENT_ONLY` | AGENTS R23 · ADR-0031 | TC-AUTH-014 |
| F104-AC-24 | HỆ THỐNG PHẢI chỉ trả ticket/duel/proposal cho người tham gia trong tenant phiên; ID của học sinh khác hoặc tenant khác PHẢI trả `404 NOT_FOUND`; `X-Tenant-Id` khác tenant phiên PHẢI trả `403 TENANT_MISMATCH` | AGENTS R8 · NFR-SEC-006 | TC-TEN-001, TC-TEN-011 |
| F104-AC-25 | HỆ THỐNG PHẢI bắt buộc `Idempotency-Key` cho `createMatchmakingTicket`, `createDuel`, `acceptDuel`, `startBotMatch` (thiếu → `400 IDEMPOTENCY_KEY_REQUIRED`); gửi lại cùng key PHẢI trả cùng kết quả; cùng key khác body PHẢI trả `422 IDEMPOTENCY_KEY_REUSED`; tạo trận, duel PHẢI ghi audit qua `audit.recorded` | AGENTS R10 · NFR-SEC-004 | — |
| F104-AC-26 | NẾU body có mode/kiểu ghép ngoài enum, `subject_id`/`topic_id` không khả dụng cho khối, hoặc chuỗi vượt `maxLength`, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường | ADR-0041 | TC-TEN-024 |
| F104-AC-27 | KHI queue đủ người cùng ràng buộc, HỆ THỐNG PHẢI đạt thời gian chờ p90 ≤ mục tiêu NFR-PERF-007 và vòng matchmaker < 1 s; metric không gắn `school_id`/`student_id` làm label | NFR-PERF-007 · [MATCHMAKING §10](../../docs/03-social-multiplayer/MATCHMAKING.md#10-metrics--alert) | TC-PERF-004 |

### Edge Cases

- NẾU học sinh mất kết nối WS > 30 s khi đang `queued`, THÌ ticket bị hủy (`cancel_reason = system`) (F104-AC-10).
- NẾU học sinh chuyển lớp/khối hoặc bị khóa khi đang chờ, THÌ proposal kiểm tra lại H1/H5 và loại ticket đó (F104-AC-08).
- NẾU proposal có ticket của party và party đổi thành viên trước khi accept, THÌ proposal hủy, ticket còn lại về queue (F104-AC-06, F104-AC-15).
- NẾU học sinh có game session bị takeover (ADR-0031), THÌ ticket giữ nguyên, sự kiện WS gửi tới thiết bị mới.
- NẾU hai request `createMatchmakingTicket` đồng thời khác key, THÌ chỉ một thành công (`SET NX`), request kia nhận `409 MM_TICKET_ALREADY_ACTIVE` (F104-AC-07).
- NẾU queue > 500 ticket, THÌ matchmaker chia bucket rating trước khi ghép (cùng bảo đảm F104-AC-02).
- NẾU tạo `Match` trong PostgreSQL lỗi sau khi mọi bên accept, THÌ retry dùng cùng `proposal_id`; không có trận trùng (F104-AC-09).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-MM-001 | 8 kiểu ghép: random, skill, grade, subject, topic, team, friend (duel), tournament (proposal trực tiếp) | [MATCHMAKING §3](../../docs/03-social-multiplayer/MATCHMAKING.md#3-các-chế-độ-matchmaking) |
| FR-MM-002 | Ràng buộc cứng H1–H7 trước rating | [MATCHMAKING §2.1](../../docs/03-social-multiplayer/MATCHMAKING.md#21-hard-constraints-thành-phần-của-queue-key--không-nới-theo-thời-gian) |
| FR-MM-003 | Cùng khối (1–5), grade band ±1 (6–9), cùng trường; liên trường theo tenant | [MATCHMAKING §2.1](../../docs/03-social-multiplayer/MATCHMAKING.md#21-hard-constraints-thành-phần-của-queue-key--không-nới-theo-thời-gian), [COMPETITION_SYSTEM §5](../../docs/01-game-design/COMPETITION_SYSTEM.md#5-công-bằng-theo-khối-lớp) |
| FR-MM-004 | Vòng đời Queue → Ticket → Proposal → Ready Check → Team Assignment → giao trận | [MATCHMAKING §4, §7](../../docs/03-social-multiplayer/MATCHMAKING.md#4-ticket) |
| FR-MM-005 | Cửa sổ rating mở rộng; đề nghị bot sau `mm_bot_after` | [MATCHMAKING §2.2, §9](../../docs/03-social-multiplayer/MATCHMAKING.md#22-soft-scoring-trong-một-queue) |
| FR-MM-006 | Ticket chỉ trong Valkey; mất → client tạo lại | [MATCHMAKING §4.2](../../docs/03-social-multiplayer/MATCHMAKING.md#42-lifecycle) |
| NFR-PERF-007 | Wait p90 khi queue đủ người | [PRODUCT_REQUIREMENTS](../../docs/00-overview/PRODUCT_REQUIREMENTS.md) |

### Key Entities

- **Matchmaking Ticket** (tạm, Valkey): người/party chờ, khóa queue (mode, phạm vi, khối, môn, chủ đề), rating, thời điểm vào hàng, trạng thái `queued → proposed → matched | cancelled | expired`.
- **Match Proposal** (tạm): cặp/nhóm ticket được chọn, deadline ready check, cờ accept; ID của proposal trở thành ID trận.
- **Match** (bền vững): trận đã được mọi bên chấp nhận, mode, kiểu ghép, khối thấp nhất, môn/chủ đề, có tính rating hay không, phạm vi, liên kết duel/giải, người tham gia và đội.
- **Duel**: lời thách đấu bạn bè unranked, hết hạn 60 s.
- **Competition Restriction**: hạn chế thi đấu có thời hạn (bỏ trận lặp lại, sanction), có lý do và audit.
- **Matchmaking Config**: cấu hình phiên bản hóa do platform publish (cửa sổ rating, grade band, timeout); F104 chỉ đọc bản hiệu lực.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `mm_rating_window` | Cửa sổ rating theo thời gian chờ |
| `mm_ready_check` | Thời hạn ready check |
| `mm_bot_after` | Mốc đề nghị bot luyện tập |
| `mm_ticket_ttl` / `mm_tickets_per_user` | Hạn ticket, số ticket active |
| `mm_ticket_rate` | Rate limit ticket, cooldown decline |
| `grade_window_g6_9` | Nới ±1 khối lớp 6–9 |
| `glicko2_params` | μ/RD mặc định khi chưa có rating |
| `abandon_penalty` | Đọc hạn chế ranked sau bỏ trận |
| `idempotency_record_ttl` | Hạn bản ghi Idempotency-Key |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Ticket/proposal chỉ ở Valkey; trận bền vững ở PostgreSQL | AGENTS R9 · ADR-0004 |
| Chỉ học sinh; một game session active | AGENTS R23 · ADR-0031 |
| Không lộ tên thật/trường của đối thủ | AGENTS R16 · ADR-0054 |
| Cấu hình matchmaking chỉ platform; tenant không đổi tham số | AGENTS R20 · ADR-0027 |
| Mọi query PostgreSQL có tenant + soft-delete | AGENTS R8, R27 |
| Không log PII ở INFO; metric không label theo học sinh/trường | AGENTS R14 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Trong 10.000 lượt ghép mô phỏng với dữ liệu khối ngẫu nhiên, 0 cặp vi phạm H1 (lớp 1–5 khác khối, lớp 6–9 lệch > 1 khối, khác grade band).
- **SC-002**: Hai học sinh lớp 3 cùng trường, cùng môn, rating lệch 50 được ghép trong ≤ 10 s khi queue chỉ có họ.
- **SC-003**: Chạy 2 matchmaker song song trong 30 phút tải: 0 ticket thuộc hai trận.
- **SC-004**: Wait p90 khi queue đủ người đạt NFR-PERF-007 ở bài tải TC-PERF-004.
- **SC-005**: Sau khi xóa toàn bộ Valkey, 100% client đang chờ nhận thông báo hủy và 0 trận "ma" xuất hiện trong PostgreSQL.

## Ngoài phạm vi

- Luật chơi, state machine trận, bot hành xử, chấm điểm, ghi kết quả — F105.
- Rating Glicko-2, Competition Points, bảng xếp hạng, mùa — F106; F104 dùng rating mặc định tới khi F106 có `RatingReader` thật.
- Giải đấu, lịch, bracket — F107 (gọi `CreateDirectProposal`).
- Soạn/publish cấu hình matchmaking trong Game Operations Console — F108.
- Party, Party Lobby, block/bạn bè — F100, F101, F095; giao diện hàng đợi trong Godot — F109; channel chat `match`/`team` — F117.

## Assumptions

- [Proposed] `proposal_id` dùng làm `match_id` (Clarifications); không cần đổi path REST.
- [Proposed] Cờ liên trường đọc từ `ref.tenants.settings_subset.cross_school_matchmaking`; khung giờ truy cập đọc từ `ref.schools.settings_subset.access_hours` (cần module identity phát trong `school.updated`/`tenant.updated`).
- [Proposed] Sanction `restrict_competition` được ghi vào `competition.competition_restrictions` bởi feature xử lý vi phạm sau này; F104 chỉ đọc.
- Giá trị cửa sổ, timeout là mặc định [Proposed] trong CONFIG_DEFAULTS; giá trị hiệu lực lấy từ cấu hình matchmaking đã publish (seed khi chưa có F108).
