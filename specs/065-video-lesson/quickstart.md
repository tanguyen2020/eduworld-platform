# Quickstart: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F030, F027, F031, F063.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app EXCEPT=core-api`, `make gen MOD=learning`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); Godot `-- --env=local`.
- Tài khoản demo (mật khẩu `Demo@12345`): `hs.an.3a`, `hs.binh.3a` (th-demo-a), `hs.dung.7a` (pgd-demo), `qlcm.truong.a`, `ad.truong.a`.
- Dữ liệu: video mẫu 7:30 đã transcode Theora 480p/720p (F027), phụ đề vi/en, câu chèn bắt buộc tại 01:30, chương 3; một bản có 2 nhánh; `ad.truong.a` giao activity cho 3A.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Lưu activity với `at_ms` > `duration_ms` qua `updateCmsActivityDraft` | `422 ACTIVITY_CONFIG_INVALID` | F065-AC-01 |
| 2 | Start khi video đang transcode | `422 VIDEO_NOT_READY`; Godot hiện `ui.video.not_ready` | F065-AC-04, F065-AC-22 |
| 3 | `hs.an.3a` start trong Godot | Payload có biến thể, phụ đề, cue không đáp án; video 720p (PC Trung bình) | F065-AC-02, F065-AC-03, F065-AC-15 |
| 4 | Bật phụ đề vi → en → tắt | Phụ đề đúng thời điểm, theo Theme | F065-AC-16 |
| 5 | Kéo thanh tua tới cuối | Không qua được mép đã xem, toast | F065-AC-17 |
| 6 | Gửi heartbeat giả nhảy tới cuối (Postman) | Phần vượt bị cắt, integrity signal; position vượt mép → `422 VIDEO_SEEK_NOT_ALLOWED` | F065-AC-05, F065-AC-06 |
| 7 | Xem tới 01:30 | Video dừng, câu hỏi hiện, không bỏ qua; trả lời → chấm server, phát tiếp | F065-AC-18, F065-AC-08 |
| 8 | Thoát ở 03:12, mở lại | "Xem tiếp từ 03:12?" → tiếp tục | F065-AC-12, F065-AC-20 |
| 9 | Submit khi chưa trả lời cue bắt buộc (API) | `422 CUE_ANSWER_REQUIRED` | F065-AC-10 |
| 10 | Xem hết + trả lời cue → hết video | Submit; màn "Hoàn thành!"; `activity.completed`; `video.progressed` 4 lần | F065-AC-21, F065-AC-10, F065-AC-07 |
| 11 | Submit lại cùng key / khác key | Cùng kết quả / `409 ATTEMPT_ALREADY_SUBMITTED` | F065-AC-11 |
| 12 | `hs.dung.7a` gọi progress cho attempt của `hs.an.3a` | `404` | F065-AC-13 |
| 13 | Video nhánh: chọn "vào nước" | Phát đoạn đúng; `video_branch_choices` có 1 dòng; hoàn thành ở `is_ending` | F065-AC-24, F065-AC-25 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
make godot-test
```

## 4. Bằng chứng cần nộp

- Kết quả test Go + GdUnit4 (pass/fail), output `atlas migrate lint`, trace_id kịch bản 3, 7, 10, ảnh phụ đề vi/en trong Godot, đo FPS 720p trên DEV-AND-MID1.
