# Quickstart: F069 — Godot — tiến trình, kho đồ, shop, bộ sưu tập, thành tựu, challenge solo

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F062, F063, F070, F074, F075, F076, F092 (cho nghiệm thu Done; trước đó dùng profile `mock` của F060, nhãn MOCK).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); Godot `-- --env=local`.
- Tài khoản demo (mật khẩu seed): `hs.an.3a` (khối 3, `th-demo-a`), `hs.dung.7a` (khối 7, `pgd-demo`); `ad.truong.a` để thử chặn nhân sự.
- Dữ liệu seed module game (dữ liệu mẫu): curve mặc định, 1 challenge `time` khối 3 và 7, 6 mặt hàng shop (1 hết hạn), 1 achievement ẩn, 1 bộ sưu tập 10 món; ví `hs.an.3a` có 150 Xu.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` đăng nhập, vào Hub | HUD có level, danh hiệu, thanh XP, ví Xu đúng `getProgression`/`getWallet` | F069-AC-01 |
| 2 | Làm xong một bài trong quest | Thanh XP tăng ≤ 1 s; khi chạm cap có biểu tượng mặt trăng + tooltip | F069-AC-02 |
| 3 | Dùng lệnh seed cộng XP vượt 2 cấp trong lúc đang chơi challenge `time` | Không có modal giữa challenge; sau màn kết quả hiện một modal "Lên 2 cấp!", không có nút "Nhận" | F069-AC-03, F069-AC-04 |
| 4 | Mở "Hành trình của tôi" | 20 dòng XP gần nhất bằng lời thân thiện, mốc level; không có ID | F069-AC-05 |
| 5 | Bấm `I`, lọc "Tóc", trang bị; thử trang bị item chưa có (fixture) | Xem trước đổi theo response; item chưa có → toast thân thiện, xem trước về cũ | F069-AC-07, F069-AC-08 |
| 6 | Bấm `B`, chọn món 120 Xu → Mua → Xác nhận; ngắt mạng ngay sau khi bấm rồi bật lại | Hộp xác nhận có số dư sau mua; retry cùng key; ví trừ đúng một lần | F069-AC-09, F069-AC-10 |
| 7 | Mua món giá > số dư; mua món hết hạn | Nút xám "Cần thêm {n} Xu"; món hết hạn → "Sẽ quay lại mùa sau" | F069-AC-11 |
| 8 | Tab Lịch sử (`hs.dung.7a`); `hs.an.3a` không có tab | Dòng mua/quy đổi trùng có nhãn thân thiện | F069-AC-12 |
| 9 | Chạy test quét scene/dependency | 0 IAP, loot box, nút tặng/trao đổi | F069-AC-13 |
| 10 | Mở Núi Thử Thách bằng `hs.an.3a` rồi `hs.dung.7a` | Khối 3 chỉ `easy`–`hard`; khối 7 thêm `expert`/`legendary` | F069-AC-14 |
| 11 | Dùng hết lượt (seed) và mở challenge | "Em đã dùng hết lượt hôm nay…", nút Vào khóa; cooldown có đếm ngược | F069-AC-15, F069-AC-16 |
| 12 | Vào `hard`, chơi 3 hoạt động; đổi giờ Windows +10 phút | Mỗi hoạt động mở `ActivityHost`; đồng hồ vẫn khớp `deadline_at` | F069-AC-17, F069-AC-18 |
| 13 | Tắt game giữa challenge, đăng nhập lại trên thiết bị khác | Tiếp tục đúng điểm/sức bền trước `deadline_at` | F069-AC-20 |
| 14 | Để hết giờ | Màn kết quả "Em đã hoàn thành {n} hoạt động", không "Thất bại" đỏ | F069-AC-19 |
| 15 | Mở Thành tựu, Bộ sưu tập; lưu 3 huy hiệu trưng bày | Thành tựu ẩn chỉ "?"; món chưa có xám + "Cách nhận"; không nút "Nhận" | F069-AC-21…F069-AC-24 |
| 16 | Đổi `vi` ↔ `en`; pseudo-locale `xx`; `lint_theme_overrides.gd` | Chuỗi đổi ngay, không cắt; 0 override | F069-AC-25, F069-AC-26 |
| 17 | Benchmark lưới 500 ô trên Android tầm trung | ≥ 30 FPS, panel mở ≤ 200 ms | F069-AC-27 |
| 18 | Đăng xuất; kiểm `user://`; ngắt mạng khi đang ở Shop | Không file dữ liệu ví/kho; Mua/Trang bị khóa khi mất mạng | F069-AC-28 |
| 19 | `ad.truong.a` đăng nhập game | `GAME_ACCESS_STUDENT_ONLY` → màn thông báo, không vào được S08 | F069-AC-29, F069-AC-30 |
| 20 | Quét log request trong integration test | Không request nào chứa `xp`, `score`, `amount`, `lives_left` | F069-AC-06 |

## 3. Lệnh test tự động

```bash
make godot-test          # GdUnit4 unit + scene + contract + integration headless
make godot-export-check
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Kết quả `make godot-test` (pass/fail), ảnh S07/S08/S09/S06 ở `vi`, `en`, PC và mobile, video kịch bản 3, 6, 12–13, số đo FPS kịch bản 17, trace_id của `purchaseShopItem` ở kịch bản 6.
