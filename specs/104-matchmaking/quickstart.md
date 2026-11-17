# Quickstart: F104 — Matchmaking 8 chế độ: queue, ticket, proposal, ready check, ràng buộc khối

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F090 (realtime), F100 (party), F095 (block), F014 (game session).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`, `make run-worker PROFILE=jobs` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); `EW_MM_TICK=500ms`.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (lớp 3A, TH Demo A), `hs.chi.3b` (3B), `hs.dung.7a` (tenant `pgd-demo`), `ad.truong.a` (School Admin). Học sinh lớp 1/9 và party lớp 6–7: fixture `testkit` (không có tài khoản demo).
- Mỗi học sinh đăng nhập game, tạo game session, lấy realtime ticket, mở WS (hoặc dùng bot WS trong `tests/integration`).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` và `hs.binh.3a` gọi `createMatchmakingTicket` (`battle_1v1`, Toán, `skill`) | Ticket `queued`; ≤ 10 s nhận `matchmaking.proposal.created` (chỉ nickname/avatar/tier) | F104-AC-01, F104-AC-04, F104-AC-05 |
| 2 | Cả hai gửi `matchmaking.proposal.accept` | `matchmaking.match.assigned`, `match_id` = `proposal_id`; `matches` 1 dòng `lobby`, event `match.created` | F104-AC-05 |
| 3 | Lặp bước 1, `hs.binh.3a` decline | `hs.an.3a` về queue (`requeued = true`, giữ thời gian chờ); `hs.binh.3a` ticket `cancelled` | F104-AC-06 |
| 4 | `hs.an.3a` tạo ticket thứ hai | `409 MM_TICKET_ALREADY_ACTIVE` | F104-AC-07 |
| 5 | Fixture học sinh lớp 1 và lớp 9 cùng rating, cùng trường; chờ 120 s | Không bao giờ có proposal giữa hai em | F104-AC-02 |
| 6 | `hs.an.3a` (TH Demo A) và fixture lớp 3 trường khác cùng tenant | Không ghép (phạm vi trường); bật `cross_school_matchmaking` tenant `unit` → ghép được | F104-AC-03 |
| 7 | Hai học sinh đã block nhau cùng queue | Không cùng proposal; học sinh có `abandon_penalty` tạo ticket → `403 POLICY_DENIED` | F104-AC-08 |
| 8 | Chạy 2 replica `worker-jobs` (matchmaker), 200 ticket bot | Không ticket nào thuộc 2 trận | F104-AC-09 |
| 9 | `cancelMatchmakingTicket`; ticket để quá `mm_ticket_ttl` | `cancelled` (`user`); `expired` | F104-AC-10 |
| 10 | `valkey-cli FLUSHALL` khi đang chờ | Client nhận `cancelled` (`system`); không có trận mới trong PG | F104-AC-11 |
| 11 | Chờ quá `mm_bot_after` một mình rồi `startBotMatch` | `bot_offer_available = true`; trận `is_bot = true`, `ranked = false` | F104-AC-12 |
| 12 | Tạo ticket 31 lần/giờ | `429 RATE_LIMITED` + `Retry-After` | F104-AC-13 |
| 13 | Party `competition_team` lớp 6–7 (fixture): member tạo ticket; leader tạo khi một member offline; member rời party sau khi tạo | `403 PARTY_NOT_LEADER`; `409 PARTY_MEMBER_NOT_ONLINE`; mọi member nhận `party_changed` | F104-AC-14, F104-AC-15 |
| 14 | Ghép 2 party 3 người với 2 ticket lẻ (`allow_fill`) | Party giữ đội; ticket lẻ chia snake draft | F104-AC-16 |
| 15 | `hs.an.3a` tạo ticket `team_battle`; `listCompetitionModes` | `403 POLICY_DENIED`; danh sách không có `team_battle` | F104-AC-17, F104-AC-22 |
| 16 | Ticket `topic` thiếu `topic_id`; ticket `friend`; `mode` = `abc` | `422 VALIDATION_FAILED` | F104-AC-18, F104-AC-26 |
| 17 | `hs.an.3a` `createDuel` tới bạn `hs.binh.3a`, bạn `acceptDuel` trong 60 s; lặp lại và chờ 61 s | Trận `ranked = false`; `410 DUEL_EXPIRED`; tới người đã block → `403 SOCIAL_SCOPE_VIOLATION` | F104-AC-19, F104-AC-20 |
| 18 | Test integration gọi `CreateDirectProposal` cho 2 entry | Proposal không qua queue, trận có `tournament_node_id` | F104-AC-21 |
| 19 | `ad.truong.a` gọi `createMatchmakingTicket` | `403 GAME_ACCESS_STUDENT_ONLY` | F104-AC-23 |
| 20 | `hs.an.3a` gọi `getMatchmakingTicket` của `hs.binh.3a`; gửi `X-Tenant-Id` của `pgd-demo` | `404 NOT_FOUND`; `403 TENANT_MISMATCH` | F104-AC-24 |
| 21 | Gửi lại `createMatchmakingTicket` cùng key; cùng key khác body; thiếu key | Cùng ticket; `422 IDEMPOTENCY_KEY_REUSED`; `400 IDEMPOTENCY_KEY_REQUIRED` | F104-AC-25 |
| 22 | `k6` TC-PERF-004 trên staging | Wait p90 đạt NFR-PERF-007; loop < 1 s | F104-AC-27 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=competition
make lint MOD=competition
make test MOD=competition
make test-integration MOD=competition
make migrate-lint MOD=competition
```

## 4. Bằng chứng cần nộp

- Số test pass/fail package `matchmaking`, `duel`; trace_id kịch bản 1–2, 10, 17; báo cáo k6 TC-PERF-004; ảnh dashboard metric `eduworld_mm_*`.
