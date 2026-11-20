# Feature Specification: F027 — Media pipeline, công thức SVG, preview session

**Feature Branch**: `027-media-math-preview` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F020
**Input**: Upload media qua presigned URL vào object storage (floci), pipeline quét virus/kiểm magic bytes/xóa EXIF/chuyển đổi biến thể, khóa content-addressed bất biến (ADR-0044, ADR-0071); kiểm tra LaTeX + sanitize SVG công thức do portal render (ADR-0055); phiên preview nội dung học tập theo version bất kỳ, không tạo attempt. FR nguồn: FR-CNT-006, FR-CNT-010, FR-CNT-017, FR-CMS-003, FR-CMS-006.

## Clarifications

### Session 2026-11-16

- Q: Server biết file đã upload xong bằng cách nào? → A: Portal gọi `completeMediaUpload` sau khi PUT (delta contract); server kiểm object trong `ew-media-quarantine` rồi phát `media.uploaded`.
- Q: Ai render SVG công thức? → A: Portal (MathJax 4 trên trình duyệt) gửi SVG + speech text kèm nội dung; module learning chỉ kiểm tra allowlist, sanitize, lưu (ADR-0055 mục 4). Không có worker render.
- Q: "Thử làm" trong preview chấm bằng gì? → A: Endpoint preview riêng (delta `submitPreviewResponse`) dùng scorer của Question Bank (F022), không lưu attempt.
- Q: Ảnh có bắt buộc `alt_text`? → A: Có, khi presign ảnh (ADMIN_PORTAL §3.11) [Confirmed].

### Session 2026-11-17

- Q: Độ dài `license` của media? → A: `varchar(255)` / `maxLength: 255` (D17, RB-210 đã chốt); contract đổi ở task T004.

### Session 2026-11-19 (triển khai)

- OQ F027-1: Ảnh xử lý bằng Go thuần trong `src/backend/pkg/media` (`golang.org/x/image` giải mã WebP + co ảnh, `github.com/HugoSmits86/nativewebp` mã hóa WebP lossless, `image/png`) thay binding libvips (cgo) của plan/research R1: không cần cgo, chạy được trên máy dev/CI; xuất từ pixel nên không còn EXIF/GPS/XMP/ICC; hướng EXIF (Orientation) được áp trước khi bỏ metadata; chặn ảnh > 50 megapixel. Âm thanh/video dùng `ffmpeg`/`ffprobe` (process con, timeout); image `worker` chuyển sang Alpine + ffmpeg. → [Confirmed].
- OQ F027-2: ADMIN_PORTAL §2.2 xếp `media.upload` trong nhóm `learning_content.*`, nhưng `pkg/authz/roles.go` (owner F002, module identity) mở rộng theo tiền tố nên `content_admin`, `academic_manager`, `tenant_admin` **không** có `media.upload`; chỉ `tenant_content_editor` và `system_admin` có. Feature không sửa catalog role (ngoài module); test dùng `tenant_content_editor` (tenant) và System Admin (platform). Đề xuất owner identity bổ sung. → [Confirmed].
- OQ F027-3: F027-AC-14 ghi khối `{math_ref, speech_text, latex}`; render payload cho học sinh giữ hợp đồng F022 + ADR-0055 §5 (`math_ref` + `speech_text`, **không** gửi LaTeX); `mathx.Block(r, withLatex=true)` dành cho view CMS/snapshot (F026). → [Confirmed].
- OQ F027-4: `math_assets` của câu hỏi (input) thêm `svg` (≤ 256 KB = `math_svg_max`), `svg_asset` thành tùy chọn: có `svg` → kiểm LaTeX, sanitize, lưu `math/<sha256>.svg` (bucket `ew-media-public`, `Cache-Control: immutable`) + dòng `content.math_svgs`; chỉ có `svg_asset` → phải là SVG đã lưu; thiếu cả hai → `422 MATH_RENDER_FAILED` kèm JSON pointer. SVG công thức của câu đề thi cũng ở `ew-media-public` (khóa băm, không đoán được). → [Confirmed].
- OQ F027-5: `submitPreviewResponse` trả schema riêng `PreviewResponseResult` (`is_correct`, `score`, `max_score`, `parts`, `explanation` rich content, `feedback[]` theo phương án đã chọn, `hint_levels[]`) thay `ActivityResponseResult` + `hint_levels` của contracts README, vì giải thích/phản hồi/gợi ý của Question Bank là rich content, không phải chuỗi. `misconception_tag` không trả. → [Confirmed].
- OQ F027-6: Preview hỗ trợ `content_type = question` (loại nội dung duy nhất đã có); `question_set`, `lesson`, `learning_material` bổ sung khi F024 có bảng, loại khác → `422 VALIDATION_FAILED`. `display_language` (`vi-VN`/`en-US`) chọn bản dịch của `i18nText` trong render (thiếu bản dịch → bản còn lại). Phiên lưu Valkey `ew:learning:preview:<id>`, gắn principal + tenant phiên: người khác/tenant khác → 404. → [Confirmed].
- OQ F027-7: `content.media_assets` và `content.media_usages` là bảng H (`entx.HybridContent`, `resource_type = media`); `media_usages` có khóa thay thế `media_usage_id` + `tenant_id`, `owner_scope`, `item_id`, `version_no` (soft-delete khi bản nháp bỏ tham chiếu); `content.math_svgs` là G, PK `varchar(64)` (thay `char(64)`, ADR-0041), chỉ INSERT (REVOKE UPDATE khỏi role ứng dụng). DATABASE_DESIGN §3.13 đã cập nhật. → [Confirmed].
- OQ F027-8: Bước worker (`ProcessMedia`) chạy trong transaction của `outbox.Consumer` (`learning.media_worker`, MaxDeliver 8, backoff 30 s → 30 phút, AckWait 15 phút); video dài giữ transaction lâu — chuyển sang job queue khi đo được. Consumer, route, adapter S3/CDN/clamd chưa nối vào binary (cùng OQ F010-2/F020-3); test gọi `ProcessMedia` dưới scope của event. → [Confirmed].
- OQ F027-9: Test tích hợp dùng floci thật (SigV4 bắt buộc) + Valkey thật; clamd là TEST DOUBLE (nhận chuỗi EICAR) — ClamAV thật chạy ở quickstart (compose profile `media`). Máy dev không có ffmpeg: test chuyển đổi âm thanh `TestFFmpegAudio` tự bỏ qua khi không có ffmpeg trên PATH. → [Confirmed].
- OQ F027-10: T003 (`.env.example`) không làm được trong phiên này (file `.env*` bị chặn với agent); biến cấu hình ghi trong quickstart §1: `EW_S3_ENDPOINT`, `EW_S3_REGION`, `EW_S3_ACCESS_KEY`, `EW_S3_SECRET_KEY`, `EW_CDN_BASE_URL`, `EW_CDN_SIGNING_KEY`, `EW_CLAMAV_ADDR`, `EW_MEDIA_SCAN`. → [Confirmed].
- OQ F027-11: `media_presign_ttl` (15 phút), `media_signed_url_ttl` (10 phút), `preview_token_ttl` (30 phút), `media_max_bytes_by_kind`, `latex_max_length`/`latex_max_depth`/`math_svg_max` dùng hằng mặc định CONFIG_DEFAULTS (đã đăng ký) vì module learning chưa đọc platform settings (như OQ F022-9). → [Confirmed].
- OQ F027-12: Signed URL của `ew-media-private` theo `secure_link` của nginx (md5 base64url của `"<expires><uri> <EW_CDN_SIGNING_KEY>"`, `deploy/cdn/nginx.conf` qua template envsubst); test kiểm bằng `CdnSigner.Verify` (hiện thực tham chiếu của quy tắc nginx), CDN thật kiểm ở quickstart bước 7. → [Confirmed].
- OQ F027-13: Tham chiếu media trong nội dung (`media_refs` + `media_ref`/`audio_ref`/`background_media_ref`/`image_media_ref`) phải là asset có trong thư viện của phạm vi (tenant khác/không tồn tại → `422 /media_refs/<i>`); ghi `media_usages` mới làm cho câu hỏi — loại nội dung khác ghi khi feature của chúng dùng `command.MediaRefs`. `kind = document` (file import) được quét + kiểm magic bytes rồi lưu nguyên (`original`) ở `ew-media-private`; `storage.AssetReader` là adapter thật của OQ F021-4. → [Confirmed].
- OQ F027-14: Test tích hợp đặt phẳng trong `tests/integration/{media_*,math,preview}_test.go` (dùng chung `TestMain`/cluster, như OQ F022-2). → [Confirmed].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Upload media an toàn (Priority: P1)

Biên tập viên kéo-thả ảnh/âm thanh/video vào thư viện; file được quét, làm sạch, chuyển đổi rồi mới dùng được trong nội dung.

**Why this priority**: Câu hỏi có hình và audio đọc đề `g1_3` (F025) cần media `ready` trước khi gửi duyệt.

**Independent Test**: Presign → PUT → complete ảnh JPG có EXIF GPS → asset `ready`, biến thể WebP/PNG không còn EXIF; file đổi đuôi sai magic bytes → `rejected`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F027-AC-01 | KHI người có `media.upload` gọi `presignMediaUpload` hợp lệ, module learning PHẢI tạo asset `quarantined` và trả presigned PUT tới `ew-media-quarantine` có hạn; NẾU cùng phạm vi đã có asset trùng `sha256`, THÌ PHẢI trả `duplicate_of` và không cấp URL mới. | FR-CNT-006 · ADR-0044 | TC-CNT-008 |
| F027-AC-02 | NẾU `mime_type` ngoài danh sách cho phép của `kind` THÌ module learning PHẢI trả `415 MEDIA_TYPE_NOT_ALLOWED`; NẾU `size_bytes` vượt giới hạn của `kind` THÌ PHẢI trả `413 MEDIA_TOO_LARGE`; NẾU ảnh thiếu `alt_text` THÌ PHẢI trả `422 VALIDATION_FAILED`. | FR-CNT-006 · [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-008 |
| F027-AC-03 | KHI gọi `completeMediaUpload`, module learning PHẢI kiểm object tồn tại, kích thước và `sha256` khớp, chuyển `processing` và ghi outbox `media.uploaded`; NẾU không khớp, THÌ PHẢI chuyển `rejected` (`type_mismatch` hoặc `too_large`). | ADR-0044 · [EVENT_DESIGN §4.2](../../docs/07-backend/EVENT_DESIGN.md) | TC-CNT-008 |
| F027-AC-04 | KHI media worker xử lý `media.uploaded`, worker PHẢI quét virus (ClamAV), kiểm magic bytes khớp `mime_type`; NẾU nhiễm virus hoặc sai magic bytes, THÌ PHẢI đặt `rejected` với `reject_reason` (`virus`/`type_mismatch`) và phát `media.rejected`. | FR-CNT-006 · AC-CNT-004 | TC-CNT-008 |
| F027-AC-05 | KHI file sạch, worker PHẢI xóa EXIF/GPS, sinh biến thể theo [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md), ghi khóa `media/<sha256>/<variant>.<ext>` (`Cache-Control: immutable`), đặt `ready` + `variants`, phát `media.processed`; lỗi chuyển đổi → `rejected` (`transcode_failed`). | FR-CNT-006 · AC-CNT-004 | TC-CNT-008 |
| F027-AC-06 | HỆ THỐNG PHẢI lưu biến thể media nội dung platform ở `ew-media-public` và media riêng tenant hoặc của đề kiểm tra ở `ew-media-private`, trả URL private dạng signed URL có hạn; URL hết hạn PHẢI bị CDN từ chối. | ADR-0044 mục 2 | TC-CNT-008 |
| F027-AC-07 | NẾU người dùng tenant presign `owner_scope = platform`, đọc asset của tenant khác (→ `404`), hoặc token game gọi API media (→ `403 PERMISSION_DENIED`), THÌ module learning PHẢI từ chối; System Admin ghi PHẢI gửi `X-Tenant-Id` đích. | R7 · R8 · R23 · ADR-0044 mục 3 | TC-TEN-001 |
| F027-AC-08 | HỆ THỐNG PHẢI không có thao tác ghi đè file của asset: thay media = tạo asset mới; version nội dung cũ vẫn trỏ asset cũ và vẫn hiển thị media cũ. | ADR-0044 mục 4 · ADR-0043 | TC-CNT-008 |

---

### User Story 2 — Thư viện media và nơi sử dụng (Priority: P2)

Biên tập viên tìm media theo loại/trạng thái/tên, xem biến thể, lý do bị từ chối và version nào đang dùng.

**Why this priority**: Tránh upload trùng, biết ảnh nào đang dùng trước khi thay.

**Independent Test**: Hai câu hỏi dùng cùng ảnh → `getMediaAsset.used_in` liệt kê 2 version; lọc `status=rejected` thấy file virus.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F027-AC-09 | KHI gọi `listMediaAssets`, module learning PHẢI lọc theo `kind`, `status`, `q` (tên gốc, `alt_text`) trong tenant phiên (System Admin: `tenant_ids`, kèm `tenant_id`), phân trang cursor. | FR-CMS-003 · [ADMIN_PORTAL §3.11](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-TEN-01 |
| F027-AC-10 | KHI gọi `getMediaAsset`, module learning PHẢI trả trạng thái pipeline, `reject_reason`, biến thể (URL signed nếu private) và `used_in` (loại item, version). | FR-CMS-003 | TC-CNT-008 |
| F027-AC-11 | KHI lưu một version nội dung, module learning PHẢI cập nhật bảng nơi sử dụng media của version đó; NẾU version tham chiếu quá `assets_per_version_max` asset, THÌ PHẢI trả `422 VALIDATION_FAILED`. | ADR-0044 mục 4, 6 | — |

---

### User Story 3 — Công thức Toán/Lý/Hóa hiển thị bằng SVG (Priority: P1)

Biên tập viên nhập công thức bằng MathLive; portal gửi LaTeX + SVG + speech text; server kiểm tra an toàn; game chỉ hiển thị SVG. AC-CNT-008.

**Why this priority**: Câu hỏi Toán có công thức là kịch bản demo M-C2.

**Independent Test**: Lưu câu hỏi có `\frac{3}{4}` + SVG hợp lệ → `math/<sha256>.svg`; render payload có `math_ref`; LaTeX có `\href` → `MATH_RENDER_FAILED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F027-AC-12 | KHI lưu nội dung chứa LaTeX, module learning PHẢI kiểm allowlist (chặn `\href`, `\url`, `\html…`, `\class`, `\style`, `\require`, `\unicode`, macro tự định nghĩa), độ dài ≤ `latex_max_length`, độ sâu ≤ `latex_max_depth`; vi phạm → `422 MATH_RENDER_FAILED` kèm JSON pointer. | FR-CNT-017 · ADR-0055 mục 7 | TC-MATH-002 |
| F027-AC-13 | NẾU thiếu SVG, SVG không parse được hoặc vượt `math_svg_max`, THÌ module learning PHẢI trả `422 MATH_RENDER_FAILED`; KHI hợp lệ, PHẢI sanitize SVG (bỏ `<script>`, thuộc tính sự kiện, tham chiếu ngoài) rồi lưu `math/<sha256>.svg` cùng `speech_text`. | FR-CNT-017 · ADR-0055 mục 4 | TC-MATH-002 |
| F027-AC-14 | KHI dựng render payload hoặc snapshot, module learning PHẢI thay LaTeX bằng khối `{math_ref, speech_text, latex}` để client game không cần MathJax. | FR-CNT-017 · AC-CNT-008 | TC-MATH-003 |
| F027-AC-15 | HỆ THỐNG PHẢI giữ SVG bất biến theo `sha256` và không render lại SVG của version đã publish. | ADR-0055 mục 9 · ADR-0043 | TC-MATH-001 |

---

### User Story 4 — Preview nội dung theo version (Priority: P2)

Reviewer xem câu hỏi/bài học như học sinh thấy (vi/en) và "thử làm" có chấm, gợi ý, phản hồi theo phương án, mà không tạo attempt. US-CMS-002.

**Why this priority**: Reviewer phát hiện lỗi hiển thị trước khi duyệt (F025).

**Independent Test**: Tạo preview draft v2 câu hỏi → nhận render không đáp án; thử làm đúng/sai → có kết quả; DB không có attempt/learning event mới.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F027-AC-16 | KHI người có `learning_content.read` gọi `createLearningPreviewSession` cho version bất kỳ (kể cả draft) trong scope, module learning PHẢI trả `preview_session_id`, render (không đáp án, theo `display_language`) và `expires_at`. | FR-CNT-010 · FR-CMS-006 · [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F027-AC-17 | HỆ THỐNG PHẢI không tạo attempt, learning event, XP, game session hay outbox event học tập khi tạo hoặc dùng phiên preview. | ADR-0031 · [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F027-AC-18 | KHI gọi `submitPreviewResponse` trong phiên còn hạn, module learning PHẢI chấm server-side và trả đúng/sai, giải thích, phản hồi theo phương án và gợi ý theo cấp; NẾU phiên hết hạn, THÌ PHẢI trả `404 NOT_FOUND`. | US-CMS-002 · ADR-0016 | — |
| F027-AC-19 | NẾU nội dung thuộc tenant khác THÌ module learning PHẢI trả `404`; NẾU thiếu `learning_content.read` THÌ PHẢI trả `403 PERMISSION_DENIED`. | R7 · R8 · ADR-0005 | TC-TEN-001 |

### Edge Cases

- PUT không bao giờ xảy ra: asset ở `quarantined`, không dùng được (`MEDIA_NOT_READY` ở F025).
- `completeMediaUpload` gọi hai lần (Idempotency-Key): lần hai trả trạng thái hiện tại, không phát lại event.
- ClamAV không khả dụng: worker retry theo backoff, asset giữ `processing`, không bao giờ chuyển `ready` khi chưa quét (trừ local `EW_MEDIA_SCAN=disabled`).
- SVG hợp lệ nhưng LaTeX khác SVG (portal lỗi): server không render lại; visual regression (TC-MATH-001) bắt lỗi.
- Upload cùng file ở hai tenant: hai asset riêng (khóa unique theo phạm vi), cùng object `media/<sha256>/…` nếu cùng bucket.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-006 | Presign, quarantine, ClamAV, magic bytes, xóa EXIF, biến thể, content-addressed, public/private + signed URL | [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) · ADR-0044 |
| FR-CNT-010 | Phần preview theo version (xem/diff ở F026) | [CONTENT_MODEL §5.1](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-017 | Allowlist LaTeX, sanitize/lưu SVG, `math_ref` trong render payload | [CONTENT_MODEL §9.1](../../docs/02-learning/CONTENT_MODEL.md) · ADR-0055 |
| FR-CMS-003 | Phần media library (lesson/question set ở F024, question ở F022) | [ADMIN_PORTAL §3.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-006 | Phần preview (workflow ở F025, publishing ở F026) | [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **MediaAsset**: metadata file trên object storage (loại, mime, kích thước, sha256, bucket, biến thể, alt text, license, trạng thái pipeline).
- **MediaUsage**: asset nào được version nội dung nào tham chiếu.
- **MathSvg**: SVG công thức đã sanitize, khóa theo sha256, kèm LaTeX và speech text.
- **PreviewSession**: phiên xem trước ngắn hạn của một version, không phải attempt.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `latex_max_length` / `latex_max_depth` / `math_svg_max` | Giới hạn công thức |
| `assets_per_version_max` | Số asset tối đa mỗi version |
| `request_body_max` | Giới hạn body API |

Giới hạn dung lượng theo loại media, hạn presigned URL, TTL signed URL (ADR-0044 mục 2, 6) và TTL preview (ADMIN_PORTAL §5) chưa có tên trong CONFIG_DEFAULTS → task Polish đăng ký `media_max_bytes_by_kind`, `media_presign_ttl`, `media_signed_url_ttl`, `preview_token_ttl`.

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| File không lưu trong PostgreSQL; khóa `media/<sha256>/…` bất biến | R31 · ADR-0044 |
| floci ở mọi môi trường; endpoint qua `EW_S3_ENDPOINT` | ADR-0071 · R3 |
| LaTeX/SVG là dữ liệu không tin cậy | ADR-0055 mục 7 |
| Preview không tạo game session | R23 · ADR-0031 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% ảnh `ready` không còn metadata EXIF/GPS (quét tự động biến thể).
- **SC-002**: 0 asset chuyển `ready` khi chưa qua quét virus ở staging/CI.
- **SC-003**: 95% ảnh ≤ 10 MB đạt `ready` trong 30 giây sau `completeMediaUpload` [Confirmed].
- **SC-004**: 0 SVG lưu trữ chứa `<script>` hoặc thuộc tính `on*` (quét `math/`).

## Ngoài phạm vi

- Media nội dung game (`/game-media`), sticker/GIF → module game/module chat (F078, F118).
- Màn hình thư viện media, MathLive/MathJax trên portal → F053, F054.
- `GamePreviewFrame` (Godot Web) cho nội dung game → F078/F079.
- Upload của học sinh (`ew-student-submissions`) — ngoài Release (ADR-0072, ADR-0082).
- Soft-delete/Thùng rác media → F045.

## Assumptions

- Preview "thử làm" cần scorer và render của F022 → đề xuất thêm F022 vào phụ thuộc của F027.
- Danh sách mime cho phép theo bảng CONTENT_MODEL §7; `license` là `varchar(255)` (D17 đã chốt, DATABASE_DESIGN §3.13).
- Mô hình 3D (`model3d`) chỉ content admin platform; F027 chỉ lưu nguyên + checksum.
