# Legal Compliance — pháp lý dữ liệu cá nhân và game cho trẻ em

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: Product Manager + Identity & Security Engineer (cần tư vấn pháp lý) · Cập nhật: 2026-11-15
> Liên quan: [ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) · [ADR-0077 (Proposed, hoãn)](../adr/ADR-0077-mo-hinh-consent-va-vai-tro-du-lieu.md) · [ADR-0078 (Proposed)](../adr/ADR-0078-mac-dinh-an-toan-cho-tre-nho.md) · [ADR-0070](../adr/ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [OPEN_QUESTIONS](../12-project/OPEN_QUESTIONS.md) (OQ-12, OQ-24) · [RISKS](../12-project/RISKS.md) · [README](README.md)

> [!WARNING]
> **Deferred — không triển khai (ADR-0082).** Version này không có rà soát pháp lý, mô hình consent (thu/thu hồi/thực thi, `consent_status`), DPIA, thời hạn thông báo vi phạm theo luật hay tuân thủ quy định game cho người dưới 18 tuổi. AI coding agent không đọc tài liệu này để triển khai; không tạo cột, endpoint, cờ hay màn hình nào từ nội dung dưới đây. Rủi ro pháp lý khi chạy với dữ liệu trẻ em thật được chấp nhận ở version này ([RISKS](../12-project/RISKS.md) RISK-03, RISK-45, RISK-50). Mở lại phải có ADR mới, chuyển nội dung về cây chính và viết feature spec.
>
> Tài liệu này là thiết kế tham khảo, **không phải ý kiến pháp lý**. Mọi tham chiếu văn bản pháp luật cần pháp chế xác nhận số hiệu, ngày hiệu lực và văn bản hướng dẫn tại thời điểm mở lại.

## 1. Mục đích và phạm vi

Lưu các nội dung pháp lý tách khỏi [DATA_PRIVACY](../10-security/DATA_PRIVACY.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md), [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md), DATABASE_DESIGN và tài liệu vận hành ngày 2026-11-15. Các **nguyên tắc kỹ thuật** về bảo vệ dữ liệu (tối thiểu hóa, không log PII, định danh giả trong analytics, mã hóa, retention job, soft-delete và ẩn danh hóa, audit, cô lập tenant) vẫn áp dụng và nằm ở tài liệu chính.

| Chủ đề | Mục |
|---|---|
| Rà soát văn bản pháp luật | §2 |
| Vai trò dữ liệu và cơ sở pháp lý xử lý theo nhóm dữ liệu | §3 |
| Mô hình consent | §4 |
| Quyền chủ thể theo luật | §5 |
| DPIA | §6 |
| Thông báo vi phạm dữ liệu cá nhân theo luật | §7 |
| Chuyển dữ liệu ra nước ngoài, lưu trữ trong nước | §8 |
| Xóa dữ liệu theo luật | §9 |
| Quy định game trực tuyến cho người dưới 18 tuổi | §10 |
| Câu hỏi mở, rủi ro, điều kiện mở lại | §11 |

## 2. Văn bản pháp luật cần rà soát (OQ-12)

| Văn bản / lĩnh vực | Điểm cần đánh giá |
|---|---|
| Luật Bảo vệ dữ liệu cá nhân 2025 và các nghị định hướng dẫn [TBD — pháp chế xác nhận số hiệu, hiệu lực] | Vai trò bên kiểm soát/bên xử lý giữa trường và EduWorld (§3); dữ liệu cá nhân của trẻ em là dữ liệu nhạy cảm/đặc thù; điều kiện đồng ý của cha mẹ/người giám hộ; nghĩa vụ của bên kiểm soát/bên xử lý; DPIA; chuyển dữ liệu ra nước ngoài; thông báo vi phạm |
| Nghị định 13/2023/NĐ-CP về bảo vệ dữ liệu cá nhân (và văn bản thay thế/sửa đổi nếu có) | Hồ sơ đánh giá tác động xử lý dữ liệu; xử lý dữ liệu trẻ em; thời hạn thông báo vi phạm |
| Luật Trẻ em 2016 và văn bản hướng dẫn | Quyền bí mật đời sống riêng tư của trẻ em; bảo vệ trẻ em trên môi trường mạng |
| Nghị định 147/2024/NĐ-CP về quản lý, cung cấp, sử dụng dịch vụ Internet và thông tin trên mạng (trò chơi điện tử) [TBD — pháp chế xác minh] | Game giáo dục có thuộc diện cấp phép/phân loại độ tuổi không; giới hạn thời gian chơi cho người dưới 18 tuổi (§10) |
| Luật An ninh mạng và văn bản hướng dẫn | Lưu trữ dữ liệu tại Việt Nam (§8) |
| Quy định của Bộ GD&ĐT về chuyển đổi số, CSDL ngành, học bạ/dữ liệu học sinh | Tích hợp SIS, ranh giới dữ liệu với CSDL ngành |

Kết quả rà soát (khi mở lại) ghi vào nhật ký trả lời của OPEN_QUESTIONS và cập nhật tài liệu sở hữu.

## 3. Vai trò dữ liệu và cơ sở pháp lý xử lý

**Đề xuất của [ADR-0077](../adr/ADR-0077-mo-hinh-consent-va-vai-tro-du-lieu.md) (Proposed, hoãn theo ADR-0082):** trường là bên kiểm soát dữ liệu học sinh phục vụ học tập, EduWorld là bên xử lý; riêng Game Analytics EduWorld là bên kiểm soát và chỉ dùng dữ liệu định danh giả; thay "lợi ích hợp pháp" bằng hợp đồng hoặc sự đồng ý; consent là một chiều của policy resolution.

Cơ sở xử lý dự kiến theo nhóm dữ liệu của [DATA_PRIVACY §2](../10-security/DATA_PRIVACY.md#2-data-inventory) (cần pháp chế xác nhận):

| Nhóm dữ liệu | Cơ sở xử lý dự kiến |
|---|---|
| Định danh học sinh, ngày sinh, mã định danh học sinh | Thỏa thuận với nhà trường + đồng ý của cha mẹ/người giám hộ qua nhà trường |
| Thông tin nhân sự (`staff`) | Hợp đồng với trường / hợp đồng lao động của EduWorld |
| Định danh đăng nhập, lịch sử đăng nhập game | Thỏa thuận với trường |
| Learning records, tiến độ video/mô phỏng, kết quả kiểm tra | Thỏa thuận cung cấp dịch vụ giáo dục |
| Dữ liệu game, thi đấu, quan hệ xã hội | Thỏa thuận |
| Nội dung chat gameplay, nội dung chat | Thỏa thuận + nghĩa vụ bảo vệ trẻ em |
| Báo cáo & kiểm duyệt | Nghĩa vụ bảo vệ trẻ em; case nghiêm trọng có thể phải giữ lâu hơn theo yêu cầu pháp lý |
| Telemetry client, logs & traces | Lợi ích hợp pháp/thỏa thuận (ADR-0077 đề xuất bỏ "lợi ích hợp pháp") |
| Game Analytics | Thỏa thuận với nhà trường/lợi ích hợp pháp [TBD OQ-12] |
| Audit log | Nghĩa vụ trách nhiệm giải trình |
| SIS staging & CSV | Thỏa thuận |
| Ghi âm giọng nói (deferred, [SPEAKING_ASSESSMENT](SPEAKING_ASSESSMENT.md)) | Thỏa thuận + đồng ý của nhà trường/phụ huynh |

## 4. Mô hình consent

Thiết kế tham khảo (không triển khai trong version này):

- EduWorld ký thỏa thuận xử lý dữ liệu với nhà trường/Sở; nhà trường thu thập **đồng ý của cha mẹ/người giám hộ** bằng mẫu do EduWorld cung cấp; trường xác nhận trạng thái đồng ý theo lớp/học sinh trong portal.
- Dữ liệu dự kiến: `org.students.consent_status` (`unknown`/`granted`/`withdrawn`), `consent_recorded_at`, `consent_version`; projection `ref.students.consent_status` ở các service để policy resolution đọc; event `student.consent_changed`; endpoint portal ghi nhận/thu hồi đồng ý theo lớp/học sinh. Keycloak giữ `consentRequired=false` vì đồng ý thu qua nhà trường.
- Học sinh chưa có consent `granted`: được dùng chức năng học tập cốt lõi nếu thỏa thuận với trường cho phép, nhưng **tắt** tính năng social (chat tự do, friend, DM, leaderboard ngoài lớp) cho đến khi có consent — cấu hình được ở cấp trường. Chat gameplay always-on (ADR-0020 mục 6) cần đánh giá riêng.
- Thu hồi đồng ý: `users.status=locked` hoặc cờ `processing_restricted` (ngừng analytics cá nhân) trong 72 giờ.
- Học sinh từ một độ tuổi theo luật định (ví dụ từ 7 tuổi trở lên trong một số quy định về dữ liệu trẻ em) có thể cần được thông báo và đồng ý thêm — **cần tư vấn pháp lý xác nhận**.
- Thông báo quyền riêng tư phiên bản dành cho trẻ em (ngôn ngữ đơn giản, hình minh họa) hiển thị trong game; phiên bản đầy đủ cho phụ huynh/trường.
- Tenant `common` (không có trường): không có trường thu consent ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) SAF-04) — cần kênh thu đồng ý trực tiếp của phụ huynh.
- Nghiên cứu hiệu quả học tập: phụ huynh được thông báo và có quyền từ chối cho con tham gia phần phân tích nghiên cứu.

## 5. Quyền chủ thể theo luật

Chủ thể là học sinh (trẻ em) — quyền thường được thực hiện qua cha mẹ/người giám hộ và nhà trường. Cơ chế kỹ thuật (xuất dữ liệu, ẩn danh hóa, khóa xử lý) ở [DATA_PRIVACY §3](../10-security/DATA_PRIVACY.md#3-yêu-cầu-về-dữ-liệu-học-sinh); phần cần pháp chế xác nhận: danh mục quyền áp dụng (được biết, truy cập, chỉnh sửa, xóa, hạn chế xử lý, rút đồng ý, phản đối), SLA theo luật, ai là đầu mối nhận yêu cầu.

## 6. DPIA

Đề xuất trước đây: DPIA và runbook thông báo vi phạm hoàn thành **trước Phase 4** (trước khi bật chat gameplay), cập nhật và phê duyệt lại trước Phase 6 Production Release; data inventory review mỗi quý. Hoãn theo ADR-0082.

## 7. Thông báo vi phạm dữ liệu cá nhân theo luật

Sự cố SEV1 lộ dữ liệu học sinh ([SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) incident response) cần thông báo cơ quan có thẩm quyền theo quy định pháp luật; thời hạn thông báo (ví dụ 72 giờ theo một số quy định) và mẫu thông báo phải được tư vấn pháp lý xác nhận (OQ-12). Runbook thông báo vi phạm theo luật hoãn.

## 8. Chuyển dữ liệu ra nước ngoài, lưu trữ trong nước

- Thiết kế kỹ thuật đặt dữ liệu cá nhân tại Việt Nam ([DATA_PRIVACY §5](../10-security/DATA_PRIVACY.md#5-luồng-dữ-liệu)). Nghĩa vụ pháp lý về lưu trữ trong nước (Luật An ninh mạng) chưa được rà soát.
- Dịch vụ bên thứ ba xử lý dữ liệu cá nhân ở nước ngoài → cần đánh giá tác động chuyển dữ liệu ra nước ngoài theo pháp luật hiện hành [TBD OQ-12].

## 9. Xóa dữ liệu theo luật

- Release 1.0 đáp ứng yêu cầu xóa bằng **ẩn danh hóa** (ADR-0028, ADR-0070). Ẩn danh hóa có đủ theo luật hay không chưa được đánh giá ([RISKS](../12-project/RISKS.md) RISK-45). Nếu tư vấn pháp lý yêu cầu xóa vật lý → ADR ngoại lệ có kiểm soát (theo mẫu ADR-0033).
- Snapshot backup dài hạn (cuối học kỳ): nếu pháp luật yêu cầu xóa triệt để, loại trừ schema `chat` khỏi snapshot dài hạn hoặc chạy lại danh sách ẩn danh khi restore ([BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md)).
- Thời hạn lưu giữ ở DATA_PRIVACY là tham số kỹ thuật [Proposed]; xác nhận theo luật hoãn.

## 10. Quy định game trực tuyến cho người dưới 18 tuổi (OQ-24)

- Câu hỏi: EduWorld có thuộc diện trò chơi điện tử trên mạng phải cấp phép/phân loại độ tuổi theo Nghị định 147/2024/NĐ-CP không? Yêu cầu giới hạn thời gian chơi của người dưới 18 tuổi áp dụng thế nào cho sản phẩm học tập dùng trong trường?
- Đề xuất trước đây: coi như phải tuân thủ phần giới hạn thời gian và hiển thị nhãn độ tuổi cho tới khi có ý kiến pháp chế ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) SAF-02).
- Các biện pháp kỹ thuật vì sức khỏe trẻ (nhắc nghỉ, khung giờ truy cập do trường cấu hình, XP cap) vẫn ở [CHILD_SAFETY §4](../10-security/CHILD_SAFETY.md#4-thời-gian-chơi-và-sức-khỏe-oq-17) và không phụ thuộc kết luận pháp lý; giờ giới nghiêm mặc định cấp platform theo [ADR-0078](../adr/ADR-0078-mac-dinh-an-toan-cho-tre-nho.md) (Proposed).

## 11. Câu hỏi mở, rủi ro, điều kiện mở lại

| Mục | Trạng thái |
|---|---|
| OQ-12 (chính sách lưu trữ theo luật, yêu cầu pháp lý, đồng ý của phụ huynh) | Hoãn (ADR-0082) |
| OQ-24 (NĐ 147/2024, giới hạn giờ chơi người dưới 18 tuổi) | Hoãn (ADR-0082) |
| ADR-0077 (consent và vai trò dữ liệu) | Proposed, hoãn |
| RISK-03, RISK-45, RISK-50 | Chấp nhận ở version này ([RISKS](../12-project/RISKS.md)) |
| Checklist trước đây: CS-07 "Đánh giá pháp lý hoàn thành" (CHILD_SAFETY), "Đánh giá pháp lý dữ liệu (OQ-12) đã có kết luận" (DEPLOYMENT, ROADMAP Phase 5/6) | Deferred (ADR-0082) |

Điều kiện mở lại: ADR mới phê duyệt phạm vi pháp lý; pháp chế xác nhận văn bản áp dụng; cập nhật DATA_PRIVACY, CHILD_SAFETY, DATABASE_DESIGN (cột consent), specs (OpenAPI, event) và viết feature spec; TC pháp lý trong TEST_STRATEGY chuyển từ Deferred về Proposed.
