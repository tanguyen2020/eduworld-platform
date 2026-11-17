# Quickstart: F035 — Nghiên cứu hiệu quả học tập

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F031, F032, F042 (và F022, F024, F026, F044 mà các feature này dùng).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=jobs` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); secret `EW_EFFICACY_PSEUDONYM_KEY_FILE` trỏ file khóa dev.
- Seed bổ sung: 2 blueprint `efficacy_instrument` dạng A/B Toán lớp 3 (3 LO × 3 câu mỗi dạng) do `cm.noidung` soạn, `cm.duyet` duyệt; `mastery_model_configs` `we-1.1` trạng thái `shadow`.
- Tài khoản demo: `sa.admin` (system_admin), `cm.noidung`/`cm.duyet` (content_admin/reviewer), `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `sm.antoan`, học sinh `hs.an.3a`, `hs.binh.3a` (3A), `hs.chi.3b` (3B).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.noidung` lưu câu `efficacy_instrument` không `exam_only`; câu tham số hóa có cờ; `bt.noidung.dv` (tenant) đặt cờ | `422 VALIDATION_FAILED` cả ba | F035-AC-01 |
| 2 | `hs.an.3a` gọi `startAdHocAttempt` 200 lần cùng LO; gọi facade `createQuestionPlan` (như module `competition`) 200 lần | Không lần nào có câu `efficacy_instrument` | F035-AC-02 |
| 3 | `cm.noidung` thêm câu đo hiệu quả vào question set tĩnh; `qlcm.truong.a` gọi `createExamSession` với blueprint dạng A; `sa.admin` gán blueprint dạng A cho `th-demo-a` | `422` cả ba | F035-AC-03, F035-AC-04 |
| 4 | Quét object CDN và render payload câu dạng A | Không có object CDN, không trường đáp án | F035-AC-05 |
| 5 | `sa.admin` gọi `createResearchAgreement` cho `th-demo-a` không header, rồi có header | `400 TENANT_HEADER_REQUIRED`, rồi `201` + audit | F035-AC-06 |
| 6 | `createEfficacyStudy` `math-g3-2026` với dạng A/B; thêm 3A (`pair_no=1`) và 3B (`pair_no=1`), không đặt arm; `ad.truong.a`, `sm.antoan`, `cm.noidung` gọi cùng API | Study `draft`; role khác → `403 PERMISSION_DENIED` | F035-AC-07, F035-AC-08, F035-AC-10 |
| 7 | Thêm 3A với `X-Tenant-Id` = `pgd-demo` | `404 NOT_FOUND` | F035-AC-08 |
| 8 | `activateEfficacyStudy`; gọi lại; sửa arm 3A | Arm gán theo hash, `form_order` cặp 1 `a_then_b`; gọi lại trả cùng kết quả; sửa → `422 INVALID_STATE_TRANSITION` | F035-AC-09 |
| 9 | `listEfficacyStudies?tenant_ids=th-demo-a` | Có `tenant_id`, audit `cross_tenant.accessed` | F035-AC-11 |
| 10 | `scheduleEfficacyWave` `pre` (cửa sổ bắt đầu sau 1 phút); `post` sớm hơn `min_intervention_weeks` | 2 ExamSession `never`/`manual`, dạng A; `post` sớm → `422` | F035-AC-12, F035-AC-13 |
| 11 | Đợi đợt mở; xem `efficacy_participants` | `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` `active` với arm của lớp | F035-AC-14 |
| 12 | `hs.an.3a` làm và nộp bài; kiểm `objective_masteries`, `progress_daily`, outbox, XP game | Không đổi; không có `exam.*` trong outbox; learning event có `context.efficacy_wave_id` | F035-AC-15 |
| 13 | `hs.an.3a` gọi `getExamResult`; `qlcm.truong.a` gọi `releaseExamResults`, `listExamSessionPapers`; `ad.truong.a` gọi `getExamSession`; `ad.truong.b` gọi cùng session | `403 EXAM_RESULTS_NOT_RELEASED`; `409`; `403`; thấy `is_efficacy = true`; `404` | F035-AC-16, F035-AC-17 |
| 14 | `withdrawEfficacyParticipants` `students=[hs.binh.3a]`, gọi lại cùng key | Participant `withdrawn`, snapshot soft-delete; lần 2 trả kết quả cũ | F035-AC-18, F035-AC-19 |
| 15 | Tạo thử nghiệm `mastery-we-1-1` (control `we-1.0`, `we-1.1` 50/50, `unit=class`, `th-demo-a`); `:start` `shadow`; `hs.an.3a` luyện tập | Hai học sinh 3A cùng arm; `getMyMastery` không đổi; `experiment_mastery_states` có dòng; event có `experiment_arms` | F035-AC-21, F035-AC-22, F035-AC-23, F035-AC-25 |
| 16 | `:start` `live`; so `getMyMastery` (học sinh arm thử) với `getStudentProgress` (`ad.truong.a`) | Học sinh thấy mức theo `we-1.1`; nhà trường thấy theo `we-1.0` | F035-AC-24 |
| 17 | Bơm dữ liệu làm arm giảm chính xác vượt ngưỡng, chạy `worker learning experiment-guardrail` (lệnh vận hành của `cmd/worker`) | Thử nghiệm `stopped` `guardrail_breach`, alert, audit | F035-AC-26 |
| 18 | `getLearningExperimentResults`; `ad.truong.a` gọi | Ô nhỏ bị ẩn; `403` | F035-AC-27 |
| 19 | Seed dataset golden, đợt `post` mở, `createEfficacyStudyReport` | Snapshot `M_eff`; báo cáo khớp giá trị tính tay; ô < 5 ẩn; `descriptive_only = true` | F035-AC-28, F035-AC-29, F035-AC-30, F035-AC-31 |
| 20 | `createPlatformReport` `efficacy_dataset`; tải CSV; xuất cho study thứ hai; bỏ secret rồi xuất | Chỉ mã giả danh, không `hs.binh.3a`; mã khác giữa study; thiếu khóa → `failed` | F035-AC-32, F035-AC-33, F035-AC-34 |
| 21 | `revokeResearchAgreement` của `th-demo-a` | Participant `withdrawn`, đợt chưa mở bị hủy, học sinh rời thử nghiệm | F035-AC-20 |
| 22 | Portal: `sa.admin` đi hết luồng trên workspace D; `ad.truong.a` mở portal; `cm.noidung` mở editor câu ở phạm vi platform, `bt.noidung.dv` ở tenant | Menu chỉ với `sa.admin`; ô cờ chỉ ở platform | F035-AC-35, F035-AC-36 |
| 23 | `sa.admin` tắt mặc định platform `menu.platform_admin.efficacy_studies` và `menu.platform_admin.learning_experiments`, tải lại portal | Hai menu "Nghiên cứu hiệu quả", "Thử nghiệm A/B" biến mất; API nghiên cứu → `403 FEATURE_DISABLED`; ô `efficacy_instrument` trong editor câu/blueprint vẫn theo `action.content_studio.question_bank.edit`/`assessments.edit` | F035-AC-37 |

## 3. Lệnh test tự động

```bash
make test MOD=learning
make test-integration MOD=learning
make test-contract
make web-test
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo package `efficacy`, `efficacy/stats`, `experiment`, `question`; kết quả golden stats; trace_id kịch bản 12, 16, 20; ảnh màn hình workspace D; mẫu CSV xuất (đã kiểm không cột định danh).
