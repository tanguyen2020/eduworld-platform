# Quickstart: F137 — Âm thanh game: nhạc theo vùng/màn, âm thanh UI và sự kiện, ambience, phù hợp giáo dục, asset CC0/MIT

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc: F060, F062, F063, F064, F071, F073, F114 (client), F133 (export Web, `scripts/godot/size`); backend local cho các màn cần dữ liệu (bài giao, bài kiểm tra, boss, Arena).
- Lệnh: `make up`, `make seed`, `make up-app`; mở `src/game/` bằng Godot 4.x, chạy `res://scenes/bootstrap/bootstrap.tscn` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); ffmpeg trong image CI cho `tools/audio/normalize.sh` và `lint_audio.mjs` (chạy local bằng container dùng một lần, không cài vào máy).
- Tài khoản demo: `hs.an.3a` (lớp 3, có lồng tiếng `g1_3`), `hs.dung.7a` (lớp 7, có bài kiểm tra mở); tai nghe hoặc loa; một máy không có đầu ra âm thanh.
- Asset: bộ asset thật do người được owner giao tải (T012); trước đó dùng file test tự sinh MIT + tiếng dự phòng.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Mở game trên Web, chạm "Chơi", đăng nhập | Nhạc `menu` chỉ bắt đầu sau lần chạm đầu; tiếng chạm nút nhẹ | F137-AC-03, F137-AC-18 |
| 2 | Vào Quảng trường Tia Sáng rồi qua cổng sang Vương quốc Số | Nhạc `hub` → nhạc vùng Toán crossfade ~1,5 s; ambience đổi; quay lại Hub không phát lại từ đầu | F137-AC-02, F137-AC-27 |
| 3 | Mở World map, Kho đồ, Cài đặt; đổi tab; đóng | Nhạc vùng giữ nguyên; tiếng mở/đóng/tab nhẹ | F137-AC-03, F137-AC-18 |
| 4 | `hs.an.3a` mở bài có lồng tiếng, bấm "Đọc to" | Nhạc nhỏ hẳn trong lúc giọng đọc, trở lại sau | F137-AC-07, F137-AC-09 |
| 5 | Trả lời đúng 3 câu liên tiếp rồi sai một câu (practice) | `correct` ×3 + `streak` một lần; sai → `try_again` nhẹ; luôn có ✓/✗ + chữ | F137-AC-21, F137-AC-23 |
| 6 | `hs.dung.7a` vào bài kiểm tra có giờ | Nhạc và ambience tắt; không tiếng tích tắc/đúng sai/chuông; nộp bài xong nhạc trở lại | F137-AC-08 |
| 7 | Mở thử thách `timed_challenge` | Nhạc tắt suốt thử thách | F137-AC-08 |
| 8 | Hoàn thành quest, nhận thưởng, lên cấp | Stinger tương ứng, nhạc giảm tạm, mỗi cái có hộp/toast hình | F137-AC-22, F137-AC-23 |
| 9 | Bật "Giảm chuyển động", lên cấp lần nữa | Stinger ngắn và nhỏ hơn | F137-AC-24 |
| 10 | Đánh boss, thắng; vào Arena, thua một trận | `boss_hit`/`boss_defeated` mềm; `match_start` không tích tắc; thua có giai điệu nhẹ + lời động viên | F137-AC-22, F137-AC-25 |
| 11 | Bạn gửi tin chat; có thông báo mới trong hộp thư; tắt "Hiện thông báo nhỏ" | Chat không tiếng; thông báo một tiếng chuông một lần; tắt công tắc → không tiếng | F137-AC-20 |
| 12 | Kéo từng thanh âm lượng về 0%, rồi 50% | Có hiệu lực ngay; 0% không phát; có tiếng mẫu khi chỉnh | F137-AC-10, F137-AC-31 |
| 13 | Chuyển tab trình duyệt / đưa app Android về nền trong lúc có thông báo | Nhạc dừng, không tiếng chuông; quay lại nhạc fade-in | F137-AC-11 |
| 14 | Chạy trên máy không đầu ra âm thanh | Không báo lỗi; mọi thông tin vẫn hiện bằng hình | F137-AC-23 |
| 15 | Chạy nhân vật trên đường làng; preset Thấp; tắt "Tiếng bước chân" | Bước chân nhẹ của chính em; Thấp → không; tắt → không | F137-AC-28 |
| 16 | Thêm file `.ogg` thiếu bản ghi manifest / license `CC-BY-4.0`, chạy lint | `lint_audio` exit 1 chỉ đúng file | F137-AC-13, F137-AC-14 |
| 17 | Chạy `normalize.sh` hai lần trên cùng file nguồn | Cùng `sha256`, manifest có số đo | F137-AC-15 |
| 18 | Export Web, chạy `scripts/godot/size` | Tải đầu ≤ 40 MB, âm thanh lõi ≤ `audio_core_budget`, mỗi vùng ≤ `audio_zone_budget` | F137-AC-29 |
| 19 | Android tầm trung, Hub 10 phút đổi vùng 5 lần | ≥ 30 FPS, không giật khi crossfade | F137-AC-30 |
| 20 | Mở Cài đặt → Thông tin → Giấy phép | Mục "Âm thanh" liệt kê tác giả/pack/license từ manifest | F137-AC-17 |

## 3. Lệnh test tự động

```bash
make godot-test                                   # GdUnit4: tests/unit/audio, tests/scene/audio, ma trận khối, kiến trúc
node --test src/game/tools/audio/tests/           # lint_audio + schema catalog/manifest
bash src/game/tools/audio/tests/normalize.test.sh # tái lập sha256 (container ffmpeg dùng một lần)
make godot-export-check                           # export Web + ngân sách
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), `node --test`, log job `audio-lint`; bảng ngân sách (lõi, từng vùng) từ export Web; số FPS thiết bị lab; danh sách file âm thanh đã owner nghe duyệt (`review.calm`) kèm manifest; ảnh tab Âm thanh 4 khối × 1280×720/844×390 (`screenshots/`).

## 5. Bằng chứng

Chưa có — ghi khi triển khai (T041–T047).
