# Research: F101 — Party Lobby, Start Session, leader/thành viên mất kết nối

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Kiểm tra điều kiện và tạo phòng cho `quest`/`boss`

- **Decision**: module competition gọi facade `game` (trong tiến trình qua `pkg/modcall`, TenantScope của party): `listPartyActivityCandidates` (cho `listPartyEligibleActivities`), `checkPartyActivityEligibility` (khi chọn hoạt động và ở bước 3 của start), `startPartyActivity` (bước 4, `Idempotency-Key = party_activity_id`) → `room_id` (encounter hoặc phiên quest co-op), `room_type`, `scene_ref`, `min_members`, `max_members`, `allow_continue_on_disconnect`. Kết thúc phiên nhận qua event `boss.encounter_completed`/`quest.party_completed`.
- **Rationale**: quest/boss, tiên quyết, mở khóa, `party_size` là dữ liệu module game (CHALLENGE_BOSS_SYSTEM §4, QUEST_SYSTEM §6); PARTY_SYSTEM §12.1 đã ghi "eligibility quest/boss của module game (facade)". Start cần kết quả đồng bộ (trả `room_id`, `409` có lý do) nên không dùng event; phản ứng phụ (kết thúc, thưởng) đi qua event (R29).
- **Alternatives**: (a) leader gọi thẳng `createBossEncounter` của module game với `party_id` — bỏ qua luồng 9 bước, party không chuyển `in_activity`, hai cơ chế Ready; (b) module competition nhân bản projection quest/boss — vi phạm một nguồn sự thật.
- **Contract**: ba operation mới trong `game.internal.v1.yaml` (`x-allowed-callers: [competition]`), F103 hiện thực; F101 khai báo trước (contract-first) và có facade test phía bên gọi (fake facade).

## R2. Thứ tự gọi module game và commit

- **Decision**: (1) khóa `ew:party:<pid>:lock` (5 s, NX) + `SELECT … FOR UPDATE` party, kiểm `expected_version`, leader, Ready, số lượng; (2) sinh `party_activity_id` (UUIDv7) và lưu cùng `Idempotency-Key`; (3) gọi `checkPartyActivityEligibility` → không đạt thì `409`; (4) gọi `startPartyActivity` (idempotent theo `party_activity_id`); (5) commit transaction (activity, membership, party `in_activity`, `state_version++`, outbox); (6) publish `party.session.started`.
- **Rationale**: module game idempotent nên thử lại an toàn; nếu (5) thất bại, phòng của module game không có ai vào và tự đóng sau `disconnect_grace` (F103), không cần saga bù.
- **Alternatives**: commit trước rồi gọi module game — party `in_activity` khi chưa có phòng, phải rollback bằng command bù.

## R3. Phát hiện mất kết nối

- **Decision**: worker `party-presence-sweeper` (chu kỳ 5 s, khóa `ew:lock:job:party-presence-sweeper`) duyệt ZSET `ew:party:active` (party có thành viên online gần đây), đọc `ew:presence:user:<id>` (hết hạn = `disconnected`); cập nhật `ew:party:<pid>:members`, đặt `ew:party:<pid>:leader_grace` (TTL `party_leader_grace`) khi leader mất kết nối, xử lý hết grace → command `AutoTransferLeader`, dormant → `DisbandParty(inactivity)`, thiếu người trong phiên → `EndPartyActivity(timeout)`.
- **Rationale**: PARTY_SYSTEM §9 yêu cầu timer không phụ thuộc node gateway; gateway đã ghi presence (VALKEY_DESIGN §3.1), không cần event mới.
- **Alternatives**: keyspace notification Valkey — không bảo đảm giao, phụ thuộc cấu hình node; event presence qua NATS — tải cao, chưa có trong EVENT_DESIGN.
- **Hệ quả**: key mới `ew:party:active`, `ew:party:<pid>:lock`, `ew:party:<pid>:leader_grace` — cập nhật VALKEY_DESIGN (thay `ew:pty:*` đề xuất ở PARTY_SYSTEM §12.2).

## R4. Một enum loại hoạt động party

- **Decision**: `PartyActivityType` trong `specs/schemas/common/enums.schema.json`: `explore`, `quest`, `boss`, `learning_challenge`, `coop_challenge`, `team_battle`; OpenAPI `PartyActivitySelection.activity_type`, AsyncAPI `ActivityRef.kind` (party), `PartySessionStarted.room_type` (thêm `boss`) và CHECK `party_activities.activity_type` cùng `$ref`/giá trị.
- **Rationale**: ba enum hiện tại lệch nhau (`explore`/`free_explore`/`exploration`, `team_battle`/`team_competition`/`team_match`) và thiếu `boss`; theo hướng S2 của CODE_PLAN §3 (enum chung).
- **Alternatives**: giữ ba enum và ánh xạ trong code — dễ lệch, trái R33.
