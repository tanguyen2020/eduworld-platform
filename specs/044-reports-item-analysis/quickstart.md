# Quickstart: F044 — Báo cáo, xuất dữ liệu bất đồng bộ, item analysis, chất lượng đề

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F042, F043 (và F041 cho bài giao, F025/F026 cho workflow version).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); bucket `ew-exports` trên floci.
- Dữ liệu: kịch bản demo M-C3 (đề "Phép nhân" 4 biến thể giao 3A, học sinh `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`) + fixture lượt trả lời `src/backend/modules/learning/tests/fixtures/itemanalysis/` (≥ 100 lượt/câu, có `rapid_guess`).
- Tài khoản demo: `qlcm.truong.a` (scope Toán), `ad.truong.a`, `ad.truong.b`, `ad.donvi`, `sa.admin`, `cm.noidung`, `cm.duyet`, `hs.an.3a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `qlcm.truong.a` gọi `createLmsReport {report_type: assignment_results, format: xlsx, assignment_id}` hai lần cùng key | `202`, cùng `report_id` | F044-AC-01 |
| 2 | Poll `getLmsReport` tới `succeeded`; tải file; chờ quá hạn link | File có watermark, chỉ học sinh 3A, không chat; quá hạn → 403; audit `report.exported` | F044-AC-02, F044-AC-03 |
| 3 | Tạo 3 export cùng lúc; tạo export phạm vi vượt giới hạn dòng (fixture); body `format: docx` | Lần 3 `429 EXPORT_LIMIT_REACHED`; `422`; `422 VALIDATION_FAILED` | F044-AC-04, F044-AC-07 |
| 4 | `ad.truong.b` gọi `getLmsReport` của bước 1 | `404` | F044-AC-05 |
| 5 | `sa.admin` gọi `createPlatformReport {tenant_ids:[…]}`; `ad.donvi` gọi `/admin/reports` | File có cột tenant, ô < 5 ẩn, audit `cross_tenant.accessed`; `403` | F044-AC-06 |
| 6 | `ad.truong.a` gọi `getCourseResults` course 3A Toán | Cột = bài graded, điểm theo `score_policy` thang 10, không có XP/rating | F044-AC-08, F044-AC-09 |
| 7 | `qlcm.truong.a` gọi `getCourseResults` course Tiếng Việt; `ad.truong.b` gọi course 3A | `403 PERMISSION_DENIED`; `404` | F044-AC-10 |
| 8 | Chạy job mức tham khảo; gọi `getRegulationReferenceReport` cho 3A Toán; xuất `regulation_reference` | Mức đúng ngưỡng; độ phủ thấp → "Chưa đủ dữ liệu"; disclaimer ở response và đầu/chân file; audit `learning.reference_report_exported` | F044-AC-11, F044-AC-12, F044-AC-15 |
| 9 | `hs.an.3a` gọi mọi API học sinh (`getMyProgress`, `getMyMastery`) | Không có nhãn mức thông tư | F044-AC-13 |
| 10 | Đổi ngưỡng hồ sơ (version mới), tính lại kỳ hiện tại | Kỳ trước giữ version cũ | F044-AC-14 |
| 11 | Chạy job item analysis; `cm.noidung` gọi `getQuestionStats`; `ad.truong.b` gọi stats câu riêng của TH Demo A | Số liệu khớp fixture, loại `rapid_guess`; version publish không đổi; `404` | F044-AC-16, F044-AC-17, F044-AC-18 |
| 12 | Chạy job hiệu chỉnh tuần; `cm.duyet` `acceptCalibrationProposal`; tác giả/role tenant thử accept câu platform | Version mới `change_note = difficulty_calibration` ở draft/chờ duyệt, audit; `403` | F044-AC-19, F044-AC-20, F044-AC-21 |
| 13 | `qlcm.truong.a` tạo exam session 4 biến thể `balance.enabled` (F042) | `expected_p` mỗi biến thể chênh ≤ ngưỡng hoặc cảnh báo kèm số liệu | F044-AC-22 |
| 14 | Session official 40 học sinh (fixture) đóng; gọi `getExamSessionReport`; `ad.truong.b` và `hs.an.3a` gọi | KR-20/alpha, SEM, phổ điểm, p-value loại `rapid_guess`, misconception; cảnh báo nếu < ngưỡng; `404`; `403` | F044-AC-23, F044-AC-24, F044-AC-25 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo package (`reporting`, `regulation`, `itemanalysis`, `examquality`), file xuất mẫu (ảnh watermark, disclaimer), bảng so sánh thống kê với tính tay, báo cáo KR-20 mẫu, bản ghi audit `report.exported`.
