# Quickstart: F107 — Tournament: 6 format, bracket, lịch, thử thách lớp/giải trường từ template

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F104, F105, F106; F045 (gán template) hoặc seed gán; seed template "Giải Toán lớp" (single elim), "Giải vòng tròn", "Thử thách nhân nhanh" (Class Challenge) gán cho `th-demo-a`, một template chưa gán.
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); đồng hồ giả trong test integration.
- Tài khoản demo: `ad.truong.a` (School Admin TH Demo A), `ad.truong.b`, `qlcm.truong.a`, `ad.donvi` (Tenant Admin `pgd-demo`), `sa.admin`, `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`, `hs.dung.7a`. Giải 5–64 học sinh: fixture `testkit`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` `createChallenge` (template "Giải Toán lớp", lớp 3A, lịch hợp lệ); gửi lại cùng key | `201`, giải `draft` ghim template version; cùng Challenge; audit `challenge.created` | F107-AC-01, F107-AC-04 |
| 2 | Tạo với template chưa gán; với `params.reward` | `403 RESOURCE_NOT_ASSIGNED`; `422 VALIDATION_FAILED` | F107-AC-02 |
| 3 | `qlcm.truong.a`, `ad.donvi` tạo; `ad.truong.a` chọn lớp trường khác; `sa.admin` không gửi tenant | `403`; `403`; `404`; `422` | F107-AC-03 |
| 4 | `name` 200 ký tự; `ends_at < starts_at` | `422 VALIDATION_FAILED` | F107-AC-05 |
| 5 | `listChallenges`, `getChallenge`, `cancelChallenge` thử thách chưa chạy | Đúng phạm vi; `cancelled`; không xuất hiện trong báo cáo học tập | F107-AC-06 |
| 6 | `publishTournament`; đồng hồ giả qua `registration_open_at` | `published → registration_open`; gọi `generate-bracket` lúc này → `422 INVALID_STATE_TRANSITION` | F107-AC-07 |
| 7 | `hs.an.3a` đăng ký (2 lần cùng key); `hs.dung.7a` đăng ký; học sinh bị hạn chế; đủ `max_entries` | `registered` (1 entry); `404`; `403 POLICY_DENIED`; `409 CONFLICT` | F107-AC-08 |
| 8 | `ad.truong.a` đăng ký hộ 3A rồi `confirmTournamentEntries`; `hs.binh.3a` rút | Mọi học sinh active `confirmed`; `withdrawn` | F107-AC-09 |
| 9 | Mở check-in, chỉ 4/5 check-in; giải chỉ 2 entry | Entry còn lại `withdrawn`, bye điều chỉnh; giải 2 entry → `cancelled` | F107-AC-10 |
| 10 | `generateTournamentBracket` N = 5 single elim (cùng key 2 lần) | 8 slot, 3 bye hạt giống 1–3, hạt giống 1/2 hai nửa; cùng bracket | F107-AC-11, F107-AC-14 |
| 11 | Giải round robin N = 5 | Mỗi entry 4 trận, không trùng cặp; xếp hạng theo luật | F107-AC-12 |
| 12 | Seeding `rating`, `random` (lớp 3), `separate_class` | Theo μ; seed lưu; không cùng lớp vòng 1 | F107-AC-13 |
| 13 | Tới giờ nút vòng 1 | Proposal trực tiếp (F104), `tournament.match_scheduled` lúc −10 và −1 phút | F107-AC-15 |
| 14 | Một entry không ready 3 phút; nút khác cả hai không vào | `forfeit`, đối thủ đi tiếp; cả hai loại, nút sau bye | F107-AC-16 |
| 15 | Phát `match.completed` 2 lần | Người thắng lên nút kế 1 lần; hết cửa sổ khiếu nại → `confirmed` | F107-AC-17 |
| 16 | `overrideBracketNode` có lý do; không lý do; sau khi nút kế bắt đầu | Đổi người thắng + audit; `422`; `409` | F107-AC-18 |
| 17 | Trận giải `aborted` 3 lần | Lên lịch lại 2 lần, sau đó chờ người tổ chức | F107-AC-19 |
| 18 | Chung kết xác nhận; chạy trao thưởng 2 lần | `tournament_results`; points `tournament`; `tournament.rewarded` mỗi entry × rule 1 lần; `rewarded`; rating không đổi | F107-AC-20, F107-AC-21 |
| 19 | Fixture double elim N = 8; groups N = 12; giải đội 3 người còn 1 | Nhánh thua đúng vị trí; 3 bảng snake top 2; `forfeit` | F107-AC-22, F107-AC-23, F107-AC-24 |
| 20 | `hs.an.3a` `getTournamentBracket`; `hs.dung.7a` cùng giải; `ad.truong.a` xem portal | Chỉ nickname; `404`; tên thật | F107-AC-25 |
| 21 | `ad.truong.a` gọi `registerTournamentEntry` bằng token game; `hs.an.3a` gọi `generate-bracket`; header tenant sai | `403 GAME_ACCESS_STUDENT_ONLY`; `403 PERMISSION_DENIED`; `403 TENANT_MISMATCH` | F107-AC-26 |
| 22 | Class Challenge "Thử thách nhân nhanh 3A": `hs.an.3a` `joinChallenge` trong khung, ngoài khung; `hs.chi.3b` join | Vào room Timed Quiz; `409`; `404` | F107-AC-27 |
| 23 | Tạo challenge với `params.match_chat = false` | `422` (không có khóa này trong `allowed_params`) | F107-AC-28 |
| 24 | Test integration gọi facade `assignTenantResources` với caller `identity`; fixture module `game` import facade đó | Dòng gán + audit; `make arch-check` fail | F107-AC-29 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=competition
make lint MOD=competition
make test MOD=competition
make test-integration MOD=competition
make test-contract
make migrate-lint MOD=competition
```

## 4. Bằng chứng cần nộp

- Số test pass/fail `tournament`, `challenge`; property test bracket N = 2..64; trace_id kịch bản 1, 10, 15, 18; log audit override.
