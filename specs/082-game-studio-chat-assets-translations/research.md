# Research: F082 — Game Studio — quản lý sticker/GIF và dịch nội dung game (`/game-admin/translations*`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Xác định trường văn bản game cần dịch (JSON pointer)

- **Decision**: `i18ntext.Pointers(schema, payload)` duyệt JSON Schema của `config_type` (`specs/schemas/*.schema.json`) và trả JSON pointer của mọi node `$ref` tới `$defs/localized_text`/`LocalizedName` (kể cả trong mảng: pointer cụ thể theo chỉ số). Loại chưa có schema đăng ký `Pointers` riêng trong module của `ConfigTypeRegistry` (F078). Trạng thái lưu ở bảng riêng `game.config_translation_states` khóa theo `config_version_id`.
- **Rationale**: Một nguồn sự thật (schema) cho "đâu là văn bản game" (R33); bảng riêng tránh migration thêm cột vào ~20 cặp bảng version của các feature khác (F071–F077) và giữ quyền sở hữu bảng rõ.
- **Alternatives**: (a) Thêm cột `translation_status`/`translation_meta` vào mọi bảng version — chạm schema của nhiều feature, migration rải rác; (b) Heuristic "object có đúng khóa `vi`/`en`" — bắt nhầm dữ liệu khác, không ổn định.

## R2. Quy tắc nhập bản dịch theo version

- **Decision**: Khóa `<config_type>/<config_id>/<version>/<json-pointer>`. Với mỗi item trong file: nếu `version` = version mới nhất và version đó `draft`/`rejected` → cập nhật chính Draft đó (`UpdateGameConfigDraft`, kiểm `If-Match` nội bộ theo version); nếu = version mới nhất và `approved`/`published`/`archived` → `CreateGameConfigVersion` (`based_on_version_id` = version đó) rồi ghi bản dịch; nếu version mới nhất `in_review` → bỏ, `version_in_review`; nếu `version` < mới nhất → bỏ, `stale_version`. Chỉ ghi ngôn ngữ đích (`target_language`); không đụng bản gốc. Mỗi item một transaction; lỗi một item không hủy item khác.
- **Rationale**: ADR-0043 (không sửa version đã publish), ADMIN_PORTAL §1.5 ("nhập lại tạo Draft mới"), tránh ghi đè công việc của người khác khi đã có version mới hơn.
- **Alternatives**: (a) Luôn tạo version mới kể cả khi có Draft mở — sinh nhiều Draft rác, xung đột workflow; (b) Merge vào version mới nhất bất kể version khóa — có thể dịch nhầm bản gốc đã đổi.

## R3. Phát hiện bản dịch lỗi thời

- **Decision**: `translation_meta[pointer] = {source_sha256, translated_in_version_id, translated_at}`. Khi lưu draft: nếu văn bản đích của pointer đổi trong lần lưu → cập nhật `source_sha256` = SHA-256 (NFC) của bản gốc hiện tại và `translated_in_version_id` = version đang lưu; nếu văn bản đích không đổi mà SHA-256 bản gốc khác `source_sha256` → pointer `outdated`. Version mới tạo từ `based_on` kế thừa meta.
- **Rationale**: Không lưu thêm văn bản (chỉ hash); diff bản gốc lấy bằng `diffGameConfigVersions` giữa `translated_in_version_id` và version hiện tại.
- **Alternatives**: Lưu bản gốc cũ trong meta — tăng kích thước jsonb, trùng dữ liệu với version cũ.

## R4. Định dạng file và chạy job

- **Decision**: XLIFF 2.1 tối thiểu (`<xliff version="2.1" srcLang trgLang>` → `<file id=config_type>` → `<unit id=khóa>` → `<segment><source/><target/></segment>`) đọc/ghi bằng `encoding/xml` chuẩn, tắt DTD/entity ngoài; CSV UTF-8 có BOM, cột `key,source,target,note`, `encoding/csv`. Job lưu `game.translation_jobs`, `worker` (profile `jobs`) lấy job bằng `SELECT … FOR UPDATE SKIP LOCKED`, ghi file vào `ew-exports/translations/<job_id>.<ext>`, `download_url` presigned.
- **Rationale**: Không thêm thư viện ngoài cho định dạng đơn giản; chống XXE (SECURITY); cùng mô hình job DB của F025 module learning, dễ vận hành.
- **Alternatives**: (a) Thư viện XLIFF bên thứ ba — ít bảo trì cho Go; (b) Work-queue NATS — thừa với tải thấp (job thủ công, vài lần/ngày).
