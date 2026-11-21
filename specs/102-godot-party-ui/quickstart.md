# Quickstart: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F092, F100, F101 (cho nghiệm thu Done; trước đó dùng profile `mock` của F060, nhãn MOCK), F095 (bạn bè).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); hai instance Godot `-- --env=local --device-id=<khác nhau>`.
- Tài khoản demo (mật khẩu seed): `hs.an.3a` (A), `hs.binh.3a` (B) là bạn bè; `hs.chi.3b` (C, cùng khối 3 cùng trường nhưng không phải bạn bè), `hs.em.4a` (D, khối 4 — khác khối). Party lớp 1–3 theo PARTY_SYSTEM (ADR-0095): chỉ `friend`/`quest`, cùng khối với leader.
- Dữ liệu: seed module game có 1 quest party khối 3 (dữ liệu mẫu).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | A đăng nhập, bấm `P` | Màn "Mời bạn bè để cùng chơi"; bấm tạo nhóm → lobby với A có vương miện | F102-AC-01 |
| 2 | A mở `InviteFriendPicker` | Thấy B (avatar + nickname), không có nhóm "bạn cùng lớp"; khối 3 không có ô gõ tên | F102-AC-02 |
| 3 | A mời B | B thấy `InvitationToast` có đếm ngược, tự ẩn sau 10 s, mở lại từ chuông | F102-AC-03 |
| 4 | A (party `friend`) mời C (không phải bạn bè, gọi qua fixture vì picker không liệt kê C); A mời D (khác khối) | Toast lỗi thân thiện theo `ui.party.error.<CODE>` mà server trả (`PARTY_INVITE_NOT_ALLOWED`/`PARTY_POLICY_VIOLATION`), không mã thô | F102-AC-05 |
| 5 | B tạo nhóm riêng rồi chấp nhận lời mời của A | Hộp "Rời nhóm hiện tại và tham gia?"; xác nhận → vào lobby A | F102-AC-04 |
| 6 | B bấm Sẵn sàng | Thẻ B trên màn A đổi ≤ 1 s, biểu tượng + chữ | F102-AC-06, F102-AC-07, F102-AC-08 |
| 7 | B mở menu thẻ A | Không có điều khiển leader ở B; ở A nút Bắt đầu xám "Còn 1 bạn chưa sẵn sàng" khi A chưa Ready | F102-AC-09 |
| 8 | A mở `ActivitySelector` | Thẻ hình lớn (khối 3); boss khối 4 có cảnh báo "{nickname} chưa đủ điều kiện" và không chọn được | F102-AC-10 |
| 9 | A chọn quest, cả hai Ready, bấm Bắt đầu | Đếm ngược 3‑2‑1, cả hai tải `scene_ref`; khi phiên kết thúc quay lại lobby, không thấy điểm của bạn | F102-AC-11 |
| 10 | A thu nhỏ lobby; B rời nhóm (xác nhận) | `PartyHud` hiện ở A; B biến khỏi HUD | F102-AC-12 |
| 11 | Ngắt mạng B trong phiên | Màn A: khung B mờ + nhãn; A vẫn chơi | F102-AC-13 |
| 12 | Ngắt mạng A (leader) ở lobby quá grace | Màn B: băng "Trưởng nhóm đang kết nối lại…", sau đó toast trưởng nhóm mới, điều khiển leader xuất hiện | F102-AC-14 |
| 13 | Bật lại mạng A | Băng vàng biến mất, lobby đồng bộ, A là thành viên thường | F102-AC-15 |
| 14 | Đổi ngôn ngữ `vi` ↔ `en` khi đang mở lobby; chạy `lint_theme_overrides.gd` | Chuỗi đổi ngay, không cắt chữ; 0 override | F102-AC-16, F102-AC-17 |
| 15 | Chạy trên Android 2400×1080 | Thẻ ngang, nút lớn dưới, chat trượt 45% (slot) | F102-AC-18 |
| 16 | Đăng nhập A ở thiết bị thứ ba (takeover) | Thiết bị cũ xóa `PartyStore`, về màn đăng nhập | F102-AC-19 |

## 3. Lệnh test tự động

```bash
make godot-test          # GdUnit4 unit + scene + integration headless
make godot-export-check
```

## 4. Bằng chứng cần nộp

- Kết quả `make godot-test` (pass/fail), ảnh lobby PC/mobile ở `vi` và `en`, video kịch bản 9 và 12, đo thời gian kịch bản 6 trên thiết bị tầm trung.
