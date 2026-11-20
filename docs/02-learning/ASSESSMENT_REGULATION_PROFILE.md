# Hồ sơ báo cáo tham khảo theo quy định đánh giá (TT27/2020, TT22/2021)

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: EdTech Specialist · Cập nhật: 2026-11-15
> Liên quan: [LEARNING_ANALYTICS](LEARNING_ANALYTICS.md) · [EXAM_ENGINE](EXAM_ENGINE.md) · [CURRICULUM_MODEL](CURRICULUM_MODEL.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) (PED-06) · Yêu cầu: FR-ANL-015 ([PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md))

Toàn bộ tài liệu là **[Confirmed]**, trừ ngưỡng mặc định ở §4.1, §4.2 (`assessment_reference_thresholds`, [Confirmed — Technical Architect 2026-11-15]). Mọi mô tả nội dung thông tư dưới đây là **[Ngoài — cần pháp chế/EdTech xác nhận]**: phải đối chiếu văn bản gốc và các văn bản sửa đổi, bổ sung còn hiệu lực trước khi triển khai.

## 1. Mục đích và phạm vi

Nhà trường cần đọc kết quả học tập trên EduWorld theo ngôn ngữ quen thuộc của quy định đánh giá học sinh: Thông tư 27/2020/TT-BGDĐT (tiểu học) và Thông tư 22/2021/TT-BGDĐT (THCS). Tài liệu này định nghĩa **hồ sơ báo cáo tham khảo** (`assessment_profile`) ánh xạ từ mastery và kết quả tự chấm sang các mức của thông tư.

**Nguyên tắc bắt buộc:**

1. Báo cáo **chỉ để tham khảo**, **không thay** đánh giá chính thức của nhà trường theo quy định của Bộ (ADR-0072 mục 2). Không ghi tự động vào học bạ hay hồ sơ chính thức.
2. Mọi màn hình và file xuất có dòng ghi chú: *"Kết quả tham khảo từ hoạt động học trên EduWorld, không phải kết quả đánh giá chính thức."*
3. Không xếp hạng học sinh; không hiển thị nhãn mức thông tư cho học sinh trong game (học sinh chỉ thấy mastery thân thiện, [LEARNING_EXPERIENCE §7](LEARNING_EXPERIENCE.md#7-hiển-thị-tiến-bộ-cho-học-sinh)).
4. Phẩm chất và năng lực chung **không** suy ra tự động từ dữ liệu trong game (cần quan sát của nhà trường); hệ thống chỉ liệt kê bằng chứng LO liên quan qua `competency_ids[]` ([CURRICULUM_MODEL §3.4](CURRICULUM_MODEL.md#34-competency-năng-lực-phẩm-chất-confirmed)).
5. Thiếu dữ liệu thì hiển thị "Chưa đủ dữ liệu", không ép ra một mức.

| Trong phạm vi | Ngoài phạm vi |
|---|---|
| Mức tham khảo theo môn học, theo kỳ, cho trường → lớp → học sinh | Học bạ, sổ điểm chính thức, xét lên lớp/hoàn thành chương trình |
| Ngưỡng ánh xạ cấu hình được, phiên bản hóa | Nhận xét định tính thay nhà trường |

## 2. Mức đánh giá theo thông tư (tóm tắt để ánh xạ) [Ngoài — cần pháp chế/EdTech xác nhận]

| Cấp học | Văn bản | Đối tượng | Mức |
|---|---|---|---|
| Tiểu học (lớp 1–5) | TT27/2020 | Môn học và hoạt động giáo dục (định kỳ) | Hoàn thành tốt · Hoàn thành · Chưa hoàn thành |
| Tiểu học | TT27/2020 | Bài kiểm tra định kỳ một số môn | Điểm số thang 10 |
| Tiểu học | TT27/2020 | Phẩm chất, năng lực | Tốt · Đạt · Cần cố gắng (không ánh xạ tự động, §1) |
| THCS (lớp 6–9) | TT22/2021 | Môn đánh giá bằng nhận xét | Đạt · Chưa đạt |
| THCS | TT22/2021 | Môn đánh giá bằng nhận xét kết hợp điểm số | Điểm thang 10 (đánh giá thường xuyên, giữa kỳ, cuối kỳ; điểm trung bình môn) |
| THCS | TT22/2021 | Kết quả học tập học kỳ/năm | Tốt · Khá · Đạt · Chưa đạt |

Môn nào thuộc nhóm nào, cách tính điểm trung bình môn và điều kiện xếp mức học kỳ/năm phải lấy theo văn bản gốc; EduWorld không tự định nghĩa lại.

## 3. Hồ sơ `assessment_profile`

| Trường | Mô tả |
|---|---|
| `profile_code` | `primary_tt27` (lớp 1–5) hoặc `lower_secondary_tt22` (lớp 6–9) |
| `version` | Phiên bản hóa; thay ngưỡng = version mới, báo cáo cũ giữ version đã dùng |
| `owner_scope` | `platform` (mặc định) hoặc `tenant` (tenant tùy chỉnh ngưỡng trong giới hạn platform) |
| `grade_levels`, `subject_ids` | Phạm vi áp dụng |
| `period` | Kỳ báo cáo: giữa kỳ I, cuối kỳ I, giữa kỳ II, cuối kỳ II (ngày theo năm học của trường) |
| `rules` | Ngưỡng ánh xạ (§4), jsonb có schema, mọi chuỗi có `maxLength` |
| `disclaimer` | Văn bản ghi chú (i18n, ≤ 500 ký tự) |

Lưu ở module learning (`learning.assessment_profiles`); kết quả tính sẵn trong read model `learning.rm_student_subject_reference_levels` (học sinh × môn × kỳ), cập nhật hằng đêm theo cơ chế rollup ([LEARNING_ANALYTICS §5](LEARNING_ANALYTICS.md#5-read-models--aggregation)). Dashboard không tính trực tiếp từ dữ liệu thô.

## 4. Ánh xạ từ dữ liệu EduWorld

**Phạm vi LO của kỳ:** các LO của môn × khối thuộc topic có trong lịch LMS của trường tới cuối kỳ (không có lịch thì theo thứ tự curriculum). **Độ phủ** = tỉ lệ LO trong phạm vi có ≥ 3 evidence.

### 4.1. Tiểu học — `primary_tt27` (ngưỡng mặc định [Confirmed])

| Mức tham khảo | Điều kiện (theo `M_eff` cuối kỳ, [LEARNING_ANALYTICS §3](LEARNING_ANALYTICS.md#3-mastery-model-mặc-định-adr-0017)) |
|---|---|
| Hoàn thành tốt | ≥ 80% LO trong phạm vi ở mức ≥ `proficient`, **và** ≥ 40% ở mức `mastered` |
| Hoàn thành | ≥ 60% LO trong phạm vi ở mức ≥ `developing` |
| Chưa hoàn thành | Còn lại |
| Chưa đủ dữ liệu | Độ phủ < 70% |

Kèm theo: điểm tham khảo thang 10 của các đề `is_official` trong kỳ ([EXAM_ENGINE](EXAM_ENGINE.md)), danh sách LO yếu và LO đã vững để nhà trường viết nhận xét.

### 4.2. THCS — `lower_secondary_tt22` (ngưỡng mặc định [Confirmed])

| Nhóm môn | Mức tham khảo |
|---|---|
| Đánh giá bằng nhận xét | **Đạt** khi ≥ 60% LO trong phạm vi ở mức ≥ `developing`; ngược lại **Chưa đạt**; độ phủ < 70% → Chưa đủ dữ liệu |
| Nhận xét kết hợp điểm số | **Điểm tham khảo** = trung bình thang 10 các đề `is_official` trong kỳ (theo trọng số `exam_type` do tenant cấu hình); kèm tỉ lệ LO ≥ `proficient` |
| Kết quả học kỳ (tổng hợp) | Không tính tự động trong Release: điều kiện xếp Tốt/Khá/Đạt/Chưa đạt phụ thuộc toàn bộ các môn, gồm cả môn không có trên EduWorld |

### 4.3. Lọc dữ liệu

- Evidence `rapid_guess` đã bị loại khỏi mastery ([LEARNING_ANALYTICS §3.1](LEARNING_ANALYTICS.md#31-định-nghĩa)); báo cáo hiển thị thêm tỉ lệ "làm quá nhanh" của học sinh để nhà trường cân nhắc độ tin cậy.
- Kết quả bài đo hiệu quả học tập ([LEARNING_EFFICACY_PLAN](LEARNING_EFFICACY_PLAN.md)) không dùng trong báo cáo này.
- Đề có cảnh báo độ tin cậy thấp ([EXAM_ENGINE §8.1](EXAM_ENGINE.md#81-độ-tin-cậy-của-đề-chính-thức-confirmed)) được đánh dấu bên cạnh điểm tham khảo.

## 5. Hiển thị và quyền

| Ai | Thấy | Ở đâu |
|---|---|---|
| `school_admin` | Mọi lớp của trường: phân bố mức tham khảo theo lớp × môn, từng học sinh | School Learning Dashboard ([SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)) |
| `academic_manager` | Như trên, trong môn được giao phạm vi | Như trên |
| Tenant `unit` | Chỉ số gộp theo trường, ẩn ô < 5 học sinh | Báo cáo tenant |
| Học sinh | Không thấy nhãn mức thông tư | — |

Xuất file (CSV/XLSX) có dòng `disclaimer` ở đầu và chân; mọi lần xuất có audit (`learning.reference_report_exported`).

## 6. Cần pháp chế/EdTech xác nhận trước khi triển khai

- Tên mức, nhóm môn, kỳ đánh giá và cách tính trong TT27/2020 và TT22/2021, cùng văn bản sửa đổi, bổ sung (nếu có) [Ngoài — cần pháp chế/EdTech xác nhận].
- Việc hiển thị nhãn trùng tên mức chính thức có gây hiểu nhầm là kết quả chính thức không; nếu có, đổi nhãn thành "Gần mức Hoàn thành tốt"… [Ngoài — cần pháp chế xác nhận].
- Ngưỡng §4 cần hiệu chỉnh bằng dữ liệu pilot và đối chiếu với đánh giá thật của nhà trường ([LEARNING_EFFICACY_PLAN §5](LEARNING_EFFICACY_PLAN.md#5-đối-chiếu-mastery-với-bài-kiểm-tra-độc-lập-predictive-validity)).

## 7. Acceptance criteria

Yêu cầu (mã PRD): FR-ANL-015 (báo cáo tham khảo), FR-LMS-013 (xuất báo cáo), FR-CUR-009 (`yccd_ref`, competency).

| AC | Kịch bản |
|---|---|
| AC-ARP-001 | Học sinh lớp 4 có 85% LO ≥ `proficient`, 45% `mastered`, độ phủ 90% → mức tham khảo "Hoàn thành tốt" |
| AC-ARP-002 | Độ phủ 50% → "Chưa đủ dữ liệu", không hiện mức |
| AC-ARP-003 | Mọi màn hình và file xuất có dòng disclaimer; xuất file ghi audit |
| AC-ARP-004 | API/học sinh trong game không trả nhãn mức thông tư |
| AC-ARP-005 | Đổi ngưỡng tạo version profile mới; báo cáo kỳ trước giữ nguyên kết quả theo version cũ |
| AC-ARP-006 | `academic_manager` môn Toán không xem được mức tham khảo môn Tiếng Việt |
