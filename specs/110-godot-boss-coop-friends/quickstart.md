# Quickstart: F110 — Godot — màn boss co-op, thẻ dữ kiện bước `interdependent`, danh sách bạn bè

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F095, F101, F102, F103 (cho nghiệm thu Done; trước đó dùng profile `mock` của F060, nhãn MOCK), F063, F069, F072, F092.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); hai instance Godot `-- --env=local --device-id=<khác nhau>`.
- Tài khoản demo (mật khẩu seed): `hs.an.3a` (A), `hs.binh.3a` (B) cùng khối 3 cùng trường; `hs.chi.3b` (C, cùng khối 3 cùng trường, chưa là bạn); `hs.dung.7a` (D, khác tenant); `ad.truong.a` (nhân sự).
- Dữ liệu seed module game (dữ liệu mẫu): boss khối 3 hai phase (`solo` + `party`), quest co-op khối 3 `collective_count` có một bước `interdependent` 2 vai.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | A chạm cổng boss, bấm "Chơi một mình" | `BossArena` tải; boss chỉ-party chỉ có nút "Rủ bạn" | F110-AC-01 |
| 2 | A tạo party với B (F102), chọn boss, cả hai Ready, Bắt đầu | Cả hai vào `BossArena`, `PartyReadyList` đổi ✓ | F110-AC-02 |
| 3 | Phase 1 bắt đầu | Năng lượng có %, phase 1/2, đồng hồ khớp server; mỗi người câu hỏi riêng | F110-AC-03, F110-AC-04 |
| 4 | A trả lời đúng, B trả lời sai | A thấy tia sáng, B thấy boss "phản đòn" + giải thích; màn A không biết B sai; năng lượng giảm trên cả hai ≤ 1 s | F110-AC-05, F110-AC-06 |
| 5 | Để hết giờ phase 1 | Băng "Thử lại phase với câu hỏi mới", tiến độ phase trước giữ | F110-AC-07 |
| 6 | Ngắt mạng B 30 s rồi bật lại; sau đó B đăng nhập trên thiết bị thứ ba | Màn A: B mờ + nhãn; B về đúng phase và câu đang mở | F110-AC-09 |
| 7 | Hạ boss | "Hạ {boss}!" + phần thưởng của chính mình; quay về lobby khi phiên kết thúc; level-up (nếu có) hiện sau kết quả | F110-AC-08 |
| 8 | Trận khác: B bấm Rời, xác nhận | B về lobby; A tiếp tục | F110-AC-10 |
| 9 | Benchmark `BossArenaBenchmark` trên Android tầm trung | ≥ 30 FPS preset Trung bình | F110-AC-11 |
| 10 | A, B nhận quest co-op từ lobby, A làm 12 câu, B 3 câu | Thanh chung 75%; B thấy chưa đạt đóng góp; A không thấy số câu của B | F110-AC-12, F110-AC-13 |
| 11 | Bước `interdependent` bắt đầu | A thấy vai + thẻ của A, chỉ thấy tên vai của B; A chia sẻ → thẻ hiện ở B ≤ 1 s | F110-AC-14, F110-AC-15, F110-AC-16 |
| 12 | Mỗi người bấm "Trả lời câu cuối" | Hai activity riêng | F110-AC-17 |
| 13 | Ngắt mạng B quá grace | A nhận toast "Em giữ thêm vai…", thẻ đã chia sẻ còn | F110-AC-18 |
| 14 | A bấm `F` | Danh sách bạn có B online (biểu tượng + chữ), không tên thật/trường/khối, không nhóm "bạn cùng lớp" | F110-AC-19, F110-AC-20 |
| 15 | A tạo mã bạn bè; C nhập mã chữ thường | Ô tự viết hoa; lời mời tạo; A thấy ở tab Lời mời; C từ chối → A vẫn thấy "Đang chờ" | F110-AC-21, F110-AC-23 |
| 16 | A bấm "Kết bạn" trên D (qua mock — khác tenant) | Thông điệp "Em chỉ kết bạn được với bạn cùng trường" hoặc lỗi chung, không lộ thông tin D | F110-AC-22 |
| 17 | A mở hồ sơ B: Mời party, Tắt tiếng, Chặn | ≤ 2 chạm mỗi thao tác; sau chặn B biến khỏi danh sách; tab Đã chặn có B, bỏ chặn được | F110-AC-24, F110-AC-25 |
| 18 | A bật "Ẩn trạng thái" | B thấy A offline | F110-AC-26 |
| 19 | Đổi `vi`↔`en`, pseudo-locale `xx`, `lint_theme_overrides.gd` | Không cắt chữ; 0 override; trạng thái có chữ | F110-AC-27, F110-AC-28 |
| 20 | Đăng xuất A; takeover; `ad.truong.a` đăng nhập game | Store xóa, không file `user://`; nhân sự bị chặn `GAME_ACCESS_STUDENT_ONLY` | F110-AC-29, F110-AC-30 |

## 3. Lệnh test tự động

```bash
make godot-test          # GdUnit4 unit + scene + contract + integration headless (2 client)
make godot-export-check
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Kết quả `make godot-test` (pass/fail), video kịch bản 3–7 và 11–13 trên hai thiết bị, ảnh `FriendsScreen`/`BossHud` ở `vi`/`en`, PC/mobile, số đo FPS kịch bản 9.
