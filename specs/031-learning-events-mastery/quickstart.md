# Quickstart: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F021, F030 (và F026 để publish version sửa lỗi).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: học sinh `hs.an.3a`, `hs.binh.3a` (`th-demo-a`); `ad.truong.a` (school_admin `th-demo-a`), `ad.truong.b` (school_admin `pgd-demo`), `qlcm.truong.a` (academic_manager, scope môn Toán `th-demo-a`), `cm.noidung` (content_admin), `sa.admin`.
- Dữ liệu: activity `quiz` Toán lớp 3 (LO `MATH.G3.NUM.MUL_TABLE.2`) đã publish, gán `th-demo-a`; `mastery_model_configs` có `we-1.0` `active` (seed).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` làm và nộp quiz (F030) | `learning_events` có `activity.started`, `question.answered` × n, `activity.submitted`, `activity.graded`, đủ envelope, hợp lệ schema | F031-AC-01 |
| 2 | Phát lại `activity.graded` vào consumer; ghi lại event cùng `idempotency_key` | Không thêm dòng/không đổi mastery | F031-AC-02, F031-AC-12 |
| 3 | `UPDATE`/`DELETE` `learning.learning_events` bằng role app | Lỗi quyền | F031-AC-03 |
| 4 | Chạy job partition, liệt kê partition | Có tháng hiện tại + ≥ 2 tháng kế | F031-AC-04 |
| 5 | Ghi event có payload ngoài schema (ví dụ `nickname`) | Bị từ chối | F031-AC-05 |
| 6 | `docker compose stop valkey`, `hs.binh.3a` nộp quiz | Event + evaluation + mastery vẫn ghi | F031-AC-06 |
| 7 | Truy vấn repository không `TenantScope` | 0 dòng | F031-AC-07 |
| 8 | Kiểm `objective_masteries` của `hs.an.3a` sau bước 1 | `value`, `evidence_count`, `model_version = we-1.0`; có `mastery_history` | F031-AC-08 |
| 9 | `make test MOD=learning` (golden test) | Mọi dòng §3.3 khớp | F031-AC-09 |
| 10 | `hs.an.3a` đạt 10/10 một lần trong ngày | Level ≠ `mastered` | F031-AC-10 |
| 11 | Đặt clock giả +104 ngày, `getMyProgress` | Level giảm theo decay, `value` lưu không đổi | F031-AC-11, F031-AC-15 |
| 12 | Phát chuỗi evidence đủ `n_min`/`d_min`; phát thêm sau decay | Một `learning_objective.mastered`; `progressed` mỗi lần đổi | F031-AC-13 |
| 13 | Gửi đồng thời 2 evidence cùng học sinh | Kết quả bằng tuần tự | F031-AC-14 |
| 14 | `ad.truong.a` xem `getStudentProgress` của `hs.an.3a`; `ad.truong.b` xem; `qlcm.truong.a` xem môn Tiếng Việt | `200` + audit `learning.profile_viewed`; `404`; `403 PERMISSION_DENIED` | F031-AC-16 |
| 15 | Kiểm `progress_daily` sau 2 attempt | Bộ đếm đúng, phát lại event không cộng đôi | F031-AC-17 |
| 16 | `cm.noidung` publish version sửa đáp án, `createRegradeRequest` `use_new_key` | `202 queued`; job → evaluation mới `re_evaluation`, cũ `superseded`, `attempt.re_evaluated`; attempt gốc không đổi | F031-AC-18, F031-AC-20 |
| 17 | `ad.truong.a` gọi `createRegradeRequest`; `qlcm.truong.a` cho môn ngoài scope | `403 PERMISSION_DENIED`; `403 PERMISSION_DENIED` | F031-AC-19 |
| 18 | Kiểm mastery LO bị ảnh hưởng sau bước 16 | Tính lại theo kết quả mới, event cũ còn nguyên | F031-AC-21 |
| 19 | Gửi lại cùng key; gửi yêu cầu khác khi đang `running` | Yêu cầu cũ; `409 CONFLICT` | F031-AC-22 |
| 20 | `worker learning mastery-recompute --model-version=we-1.1` (lệnh vận hành của `cmd/worker`) | Báo cáo chênh lệch; `objective_masteries` không đổi | F031-AC-23 |
| 21 | `sa.admin` kích hoạt `we-1.1` | Bản ghi mang `we-1.1`; không có `mastered` lặp | F031-AC-24 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning               # golden test mastery
make test-integration MOD=learning   # PostgreSQL + NATS + Valkey (có kịch bản dừng Valkey)
make migrate-lint MOD=learning
make specs-lint
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo F031-AC-*, kết quả golden test (SC-001), độ trễ mastery p95 (SC-002), trace_id bước 1 và 16.
