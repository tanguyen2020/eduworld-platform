# Feature Specification: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Feature Branch**: `116-chat-moderation-pipeline` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` · **Mốc**: M4a · **Phụ thuộc**: F115
**Input**: Hiện thực bộ kiểm tra nội dung đồng bộ mà F115 bắt buộc gọi: giới hạn kỹ thuật luôn chạy (gồm sanction), normalize tiếng Việt, danh mục từ nhạy cảm, PII, spam, bắt nạt, context rules → `allow | unchecked | mask | block | hold_for_review | block_and_alert`; danh mục quản lý được (category, term, allowlist, version, test hồi quy, reload không downtime); công tắc kiểm duyệt platform/tenant chỉ System Admin; quick message song ngữ, không dịch máy chat. FR nguồn: FR-CHAT-005, FR-MOD-001, FR-MOD-009…015, NFR-SAFE-002, NFR-I18N-004.

## Clarifications

### Session 2026-11-16

- Q: Ai bật/tắt category cho tenant (`allow_tenant_toggle`)? → A: Theo ADR-0072 và contract hiện hành: chỉ Platform Safety Team/System Admin (`chat.tenant_category.toggle`, endpoint gateway nội bộ `toggleTenantSensitiveCategory`) theo yêu cầu của tenant; Tenant Admin chỉ xem (`listTenantSensitiveCategories`). FR-MOD-013/AC-MOD-022 và MODERATION §11.3 còn ghi Tenant Admin tự bật/tắt → cần sửa tài liệu.
- Q: Ai ghi từ bổ sung của tenant? → A: Chỉ `safety_moderator` (`chat.tenant_terms.write`, ADR-0072; ADMIN_PORTAL §2.5). API_DESIGN ghi `chat.sensitive_terms.write` cho nhóm endpoint này → theo ADMIN_PORTAL/MODERATION.
- Q: `hold_for_review`/`block_and_alert` tạo moderation case ở đâu? → A: F116 lưu tin `held`, phát `chat.message_held` (EVENT_DESIGN §4.9) và hạn chế tạm thời theo MODERATION §6.3; tạo case, on-call, hàng đợi là F121.
- Q: Sanction ở F116 gồm gì? → A: Bảng `chat.sanctions`, kiểm tra sanction trong giới hạn kỹ thuật và auto-mute hệ thống của bước spam (MODERATION §3.4); sanction thủ công và các loại khác → F121.
- Q: Bước `async_analysis`? → A: F116 chỉ ghi trạng thái bật/tắt bước vào envelope và decision; worker phân tích bất đồng bộ (`chat-moderation-async`) thuộc F121.

### Session 2026-11-17

- Q: PII/grooming luôn chạy, công tắc có thời hạn và phê duyệt hai người (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md). Edge case cũ "tắt `pii` vẫn cho phép" (ADR-0030) bị thay bởi ADR-0078.

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

Owner đã duyệt trước các mặc định [Proposed]; khi tài liệu thiếu/mâu thuẫn, chọn phương án an toàn nhất cho trẻ:

- Q: `rate.burst` và `slow_mode_seconds` (F115 để lại cho F116) nghĩa chính xác? → A: Giới hạn kỹ thuật luôn chạy: tối đa `burst` tin của một người gửi trong một channel trong mọi cửa sổ 10 giây; slow mode = khoảng cách tối thiểu giữa hai tin của cùng người gửi trong channel. Vi phạm → `CHAT_RATE_LIMITED` + `Retry-After`; tin bị từ chối không chiếm lượt.
- Q: "tương tự ≥ 0,9" của bước spam đo thế nào? → A: Bằng nhau sau normalize (dấu, teencode, leetspeak, ký tự lặp, ký tự chen, hoa/thường); Valkey chỉ giữ hash, không giữ nội dung (R14).
- Q: Bộ đếm "3 lần block trong 10 phút" tính loại block nào, có phụ thuộc công tắc `spam`? → A: Mọi tin bị `block` (danh mục, PII, spam), luôn chạy như sanction (không tắt được); đạt ngưỡng thì đếm lại từ đầu. Tái phạm trong 24 giờ → `auto_mute_spam` 1 giờ.
- Q: `block_and_alert` hạn chế người gửi bao lâu? → A: Sanction `mute` hệ thống `scope = all`, không có `ends_at` (đến khi Safety Team xử lý ở F121); không tạo trùng khi đã có.
- Q: "Phát hiện grooming luôn chạy" gồm gì ở F116? → A: Category `grooming_scam` của danh mục (vẫn khớp khi `sensitive_terms` hoặc `enabled_all` tắt) và mẫu hẹn gặp ngoài đời của bước PII (`hold_for_review`, mã `grooming_scam`).
- Q: Tham số quick message bị `mask`? → A: Chuyển thành `block` (không hiển thị một phần); tham số phải đúng các chỗ trống `{…}` của câu đã duyệt, sai → `VALIDATION_FAILED`.
- Q: Lưu bản biên dịch/snapshot danh mục ở đâu? → A: Mỗi `chat.sensitive_catalog_versions` có cột `snapshot` (jsonb bất biến: category, term đang bật, allowlist) + `checksum`; mỗi instance biên dịch trong bộ nhớ, hoán đổi nguyên tử, đối chiếu mỗi 30 s + Valkey pub/sub `ew:chat:sensitive:version`; không cache bản biên dịch trong Valkey (biên dịch từ snapshot đủ nhanh). Instance chưa nạp được danh mục platform → readiness fail, tin fail closed.
- Q: Category mặc định và `allow_tenant_toggle`? → A: 10 category seed (`profanity` low, `insult` medium, `violence` medium, `sexual` high, `drugs` medium, `self_harm` critical, `grooming_scam` critical, `politics_religion` medium, `pii_pattern` medium, `context_watch` watch); chỉ `politics_religion`, `context_watch` cho phép tenant bật/tắt. Version 1 của danh mục platform không có term (R15).
- Q: Ngưỡng spam, timeout RE2, thời gian lan truyền đọc ở đâu? → A: Hằng số code trong `internal/domain/moderation` (đăng ký CONFIG_DEFAULTS), không có biến môi trường (F116-AC-26).
- Q: Rule `validation.profanity/pii/…` và `sensitive_term_policy.action_overrides` của ChatPolicy? → A: F116 chỉ áp `validation.strictness` (nâng một mức với `very_strict`); hành động theo severity/override của term; ghi đè theo policy để F120 (mặc định platform hiện trùng ánh xạ severity).
- Q: F100 đã thêm operation facade `moderateDisplayText` (tên party/đội) chờ module chat hiện thực — có thuộc F116? → A: Phía chat là cùng pipeline (mục "Ngoài phạm vi": F100/F120 gọi cùng port), nên F116 hiện thực bên nhận: normalize + danh mục (platform + tenant, công tắc hiệu lực, PII/grooming luôn chạy), không cửa sổ spam; chỉ `allow`/`unchecked` → `allowed`, `mask` trở lên → `blocked` (tên không bao giờ hiển thị bị che một phần); lỗi → bên gọi dùng tên mặc định. Tenant của body phải là tenant của scope người gọi.
- Q: Telex/VNI chưa gõ xong và NER tên trường + tên người (MODERATION §3.1, §3.3)? → A: Ngoài các biến thể của F116-AC-02; không làm ở F116, ghi việc tiếp theo.
- Q (US2): allowlist của tenant? → A: Không có — tenant chỉ siết chặt (MODERATION §11.3); `createSensitiveAllowlistEntry` luôn ghi allowlist platform (bỏ qua `X-Tenant-Id`), snapshot tenant có `allowlist` rỗng.
- Q (US2): event cho category/allowlist? → A: Chỉ term có schema (`sensitive_term.*`); thay đổi category/allowlist chỉ ghi audit (`chat.sensitive_category.*`, `chat.sensitive_allowlist.*`), không nội dung.
- Q (US2): giới hạn sửa category? → A: `code` bất biến; category `grooming_scam` không xóa, không tắt, không cho tenant bật/tắt (ADR-0078); category còn term live không xóa được (`409 CONFLICT`); snapshot platform gồm mọi category live kèm cờ `enabled` (term của category tắt không khớp, trừ khi tenant được phép bật).
- Q (US2): `≤ 200 pattern/tenant` tính thế nào? → A: Mỗi phạm vi riêng (platform ≤ 200, mỗi tenant ≤ 200); vượt → `SENSITIVE_TERM_INVALID`.
- Q (US2): bộ hồi quy đo gì? → A: Mẫu dương tính (PII/hẹn gặp bịa + câu sinh từ term high/critical của danh mục ứng viên theo mẫu câu, viết hoa, chấm chen; không sinh cho `pattern`) được tính phát hiện khi decision ≥ `block`; câu sạch là dương tính giả khi decision khác `allow` (kể cả `mask`); chạy với mọi bước bật, strictness `standard`. Bộ placeholder nhúng `seed/regression/corpus.v1.json`; bộ vận hành ≥ 2.000 câu cắm qua `regression.Source` (ngoài repo).
- Q (US2): mã `412` của `If-Match`? → A: `VERSION_MISMATCH` (pkg/problem không có `PRECONDITION_FAILED`); `ETag: W/"<version>"` trả ở tạo/sửa. Contract `SensitiveTerm`/`SensitiveTermCategory` chưa có `version` nên danh sách chưa cho ETag từng dòng → câu hỏi mở cho F122.
- Q (US2): `testSensitiveTerms` trả gì? → A: Chỉ các khớp danh mục (schema bắt buộc `severity`); PII vẫn tính vào `decision`. Không ghi tin hay decision log; audit `chat.sensitive_terms.tested` (metadata, không văn bản).
- Câu hỏi mở (US2): identity `ResolvePrincipal` chỉ đưa System Admin vào tenant không phải thành viên; `safety_moderator` gửi `X-Tenant-Id` hiện nhận `TENANT_MISMATCH` (fail closed) cho tới khi identity phân giải role platform có `chat.tenant_terms.write`/`chat.tenant_category.toggle` vào tenant đích (ADR-0072). Test tích hợp mô phỏng principal đã phân giải. `school_admin` không có `chat.sensitive_terms.read` nên `/tenant/chat/categories` → 403 (contract ghi Tenant Admin, School Admin). Four-eyes trước publish platform (MODERATION §11.4 bước 4) chưa có trong contract/AC → chưa làm.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Mọi tin nhắn được kiểm tra trước khi phát (Priority: P1)

Học sinh gửi tin; tin bình thường đi ngay, từ thô bị che, số điện thoại/link/mạng xã hội bị chặn kèm lời nhắc thân thiện, tin nguy hiểm bị giữ để Safety Team xem. Học sinh không bao giờ biết từ nào bị khớp.

**Why this priority**: Là điều kiện để bật chat cho trẻ 6–15 tuổi (ADR-0020, NFR-SAFE-001); F115 không được bật nếu thiếu.

**Independent Test**: Bộ câu mẫu có nhãn (có dấu, không dấu, teencode, leetspeak, chèn ký tự, SĐT viết bằng chữ, "kết bạn zalo") gửi qua `chat.message.send` → quyết định đúng nhãn; phản hồi không chứa từ đã khớp; tin `block` không có dòng trong PG.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F116-AC-01 | KHI nhận nội dung mới (text, tham số quick message), module chat PHẢI chạy theo thứ tự: giới hạn kỹ thuật → công tắc → normalize → định dạng → danh mục → PII → spam → bắt nạt → context rules, và trả một quyết định trong `allow`/`unchecked`/`mask`/`block`/`hold_for_review`/`block_and_alert` | FR-CHAT-005 · FR-MOD-001 · [MODERATION §3](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-003 |
| F116-AC-02 | KHI so khớp danh mục, module chat PHẢI khớp trên mọi biến thể đã normalize (bỏ dấu, teencode, leetspeak, ký tự lặp, ký tự chen giữa, homoglyph) theo ranh giới từ và loại trừ cụm trong allowlist trước | FR-MOD-009 · AC-MOD-020 · [MODERATION §3.1, §11.5](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-011 |
| F116-AC-03 | KHI có khớp, module chat PHẢI chọn action nặng nhất theo `action_override` của term hoặc ánh xạ severity của category (`watch`→`allow`, `low`→`mask`, `medium`→`block`, `high`→`hold_for_review`, `critical`→`block_and_alert`), nâng một mức khi `validation.strictness` của channel yêu cầu, và `mask` chỉ che các đoạn khớp | FR-MOD-010 · ADR-0030 | TC-CHAT-011 |
| F116-AC-04 | KHI phát hiện SĐT (kể cả viết bằng chữ, tách dấu), email, URL ngoài allowlist, mạng xã hội/app, mã định danh, module chat PHẢI `block`; địa chỉ/hẹn gặp PHẢI `hold_for_review` theo bảng MODERATION §3.3 | FR-MOD-001 · AC-CHAT-004 | TC-CHAT-010 |
| F116-AC-05 | NẾU quyết định là `block`, THÌ module chat PHẢI trả `CHAT_CONTENT_BLOCKED` chỉ kèm `category_code` (hoặc `pii`/`spam`/`bullying`), không cấp `seq`, không lưu nội dung gốc; phản hồi PHẢI không chứa từ đã khớp, vị trí khớp hay nội dung danh mục | FR-CHAT-005 · FR-MOD-010 · AGENTS R15 | TC-CHAT-015 |
| F116-AC-06 | KHI quyết định là `hold_for_review`/`block_and_alert`, module chat PHẢI lưu tin `moderation_state = held` (không phát cho người khác), trả `chat.message.held` cho người gửi, phát `chat.message_held` (`message_id`, `channel_id`, `reasons[]`, `catalog_version`) và với `block_and_alert` PHẢI tạm hạn chế chat của người gửi (sanction `mute` hệ thống) đến khi Safety Team xử lý | FR-MOD-010 · [MODERATION §6.3](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-003 |
| F116-AC-07 | KHI bước spam phát hiện tin lặp (≥ 3 tin tương tự trong 60 giây) hoặc cùng nội dung ở ≥ 4 channel trong 2 phút, module chat PHẢI `block` tin đó; khi người gửi bị `block` 3 lần trong 10 phút PHẢI áp sanction `mute` hệ thống theo `auto_mute_spam` và ghi audit | FR-MOD-001 · AC-MOD-008 | TC-CHAT-007 |
| F116-AC-08 | TRONG KHI người gửi có sanction `mute` còn hiệu lực, module chat PHẢI từ chối mọi tin với `CHAT_MUTED` ở bước giới hạn kỹ thuật, kể cả khi công tắc kiểm duyệt tắt | FR-MOD-012 · AC-MOD-021 | TC-CHAT-014 |
| F116-AC-09 | HỆ THỐNG PHẢI ghi một decision log cho mọi tin (`decision`, `category_codes`, `steps_run`, `steps_skipped`, `catalog_version_platform`, `catalog_version_tenant`, `switch_snapshot_id`, `latency_ms`), không chứa nội dung tin | FR-MOD-011 · [MODERATION §12.4](../../docs/03-social-multiplayer/MODERATION.md) · AGENTS R14 | TC-CHAT-013 |
| F116-AC-10 | HỆ THỐNG PHẢI giữ p95 toàn pipeline dưới ngưỡng `moderation_pipeline_latency` với tin 200 ký tự trên benchmark CI | NFR-SAFE-002 · AC-MOD-004 | TC-CHAT-012 |
| F116-AC-11 | HỆ THỐNG PHẢI cho cùng nội dung ở cùng loại channel và cùng policy cùng quyết định với mọi học sinh; pipeline PHẢI không nhận tham số khối/lớp | FR-CHAT-005 · AC-CHAT-003 · AGENTS R15 | TC-CHAT-010 |

---

### User Story 2 — Danh mục từ nhạy cảm quản lý được, có hiệu lực không cần deploy (Priority: P1)

System Admin thêm từ, thử câu mẫu, publish; trong một phút mọi tin chứa từ đó (kể cả bản không dấu/teencode) bị xử lý. Safety Team thêm từ bổ sung cho một tenant theo yêu cầu của tenant.

**Why this priority**: Không hardcode từ nhạy cảm (R15); tiếng lóng thay đổi liên tục.

**Independent Test**: `sa.admin` tạo term trong category `insult` → `testSensitiveTerms` thấy `block` → `publishSensitiveCatalog` → ≤ 60 s sau, tin chứa term ở `th-demo-a` và `pgd-demo` bị chặn; `sm.antoan` thêm term tenant cho `pgd-demo` → chỉ `pgd-demo` bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F116-AC-12 | KHI System Admin (`chat.sensitive_terms.write`, gateway nội bộ, MFA) tạo/sửa/xóa mềm category, term, allowlist, module chat PHẢI lưu ở trạng thái nháp, kiểm `match_type = pattern` là RE2 ≤ 200 ký tự không backreference/lookaround (sai → `SENSITIVE_TERM_INVALID`), ghi audit và phát `sensitive_term.created/updated/deleted` không chứa nội dung term | FR-MOD-009 · AC-MOD-019 · AGENTS R21 | TC-CHAT-016 |
| F116-AC-13 | KHI người có `chat.sensitive_terms.read` gọi `testSensitiveTerms`, module chat PHẢI trả danh sách `category_code`, `severity`, `action`, span và decision mà không ghi tin nào | FR-MOD-009 · [MODERATION §11.4](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-011 |
| F116-AC-14 | KHI gọi `publishSensitiveCatalog`, module chat PHẢI chạy bộ test hồi quy; NẾU recall với `high`/`critical`/PII hoặc false positive không đạt `filter_quality_gate`, THÌ PHẢI trả `422 SENSITIVE_CATALOG_REGRESSION_FAILED` và không tạo version; đạt thì tạo `catalog_version` tăng đơn điệu, lưu `checksum`, `regression_recall`, `regression_fp_rate`, phát `sensitive_catalog.published` | FR-MOD-015 · ADR-0030 | TC-CHAT-012 |
| F116-AC-15 | KHI có version mới, mọi instance module chat PHẢI biên dịch ở nền và hoán đổi nguyên tử trong ≤ 60 giây (pub/sub + đối chiếu định kỳ); tin đang xử lý dùng bản cũ, tin sau thời điểm hiệu lực dùng bản mới, không lỗi và p95 không vượt trần | FR-MOD-015 · AC-MOD-020 · AC-MOD-012 | TC-CHAT-012 |
| F116-AC-16 | KHI `safety_moderator` (`chat.tenant_terms.write`) thêm/sửa/xóa mềm term bổ sung cho tenant đích (`X-Tenant-Id`) và publish, module chat PHẢI chỉ cho thêm vào category có sẵn (sửa term platform → `SENSITIVE_TERM_INVALID`) và term chỉ hiệu lực ở tenant đó trong ≤ 60 giây | FR-MOD-013 · AC-MOD-029 · ADR-0072 | TC-CHAT-015 |
| F116-AC-17 | KHI Safety Team/System Admin (`chat.tenant_category.toggle`) bật/tắt category cho tenant, module chat PHẢI yêu cầu `reason`, chỉ cho khi `allow_tenant_toggle = true` (ngược lại `403 TENANT_TOGGLE_NOT_ALLOWED`), ghi audit, phát `chat_policy.tenant_category_toggled` | FR-MOD-013 · ADR-0072 | TC-CHAT-015 |
| F116-AC-18 | NẾU role tenant (`tenant_admin`, `school_admin`, `academic_manager`) hoặc token game gọi bất kỳ endpoint ghi danh mục/công tắc nào, THÌ HỆ THỐNG PHẢI không có route trên gateway public và module chat PHẢI trả `403 PERMISSION_DENIED`; Tenant Admin chỉ đọc được `listTenantSensitiveCategories` (tên, mô tả, mức độ, trạng thái — không term) | FR-MOD-013 · AC-MOD-022 · AGENTS R20 | TC-CHAT-015 |
| F116-AC-19 | HỆ THỐNG PHẢI không gửi danh mục (term, allowlist, bản biên dịch) xuống game client, không ghi nội dung term vào log/audit/event | FR-MOD-009 · AC-MOD-013 | TC-CHAT-015 |

---

### User Story 3 — Công tắc kiểm duyệt có kiểm soát (Priority: P1)

System Admin đề xuất tắt một bước (ví dụ `spam`) cho một tenant khi có lý do, có thời hạn ≤ 24 giờ; yêu cầu chỉ có hiệu lực khi một System Admin khác phê duyệt; bước PII và phát hiện grooming không bao giờ tắt được; tin vẫn bị giới hạn kỹ thuật; mọi người liên quan thấy cảnh báo; hết hạn tự bật lại.

**Why this priority**: ADR-0030 cho phép tắt bước kiểm tra; ADR-0078 giữ PII/grooming luôn chạy và buộc thời hạn + hai người duyệt để không vô tình để trẻ không được bảo vệ.

**Independent Test**: `sa.admin` yêu cầu tắt `steps.pii` → `422 MODERATION_SWITCH_STEP_NOT_ALLOWED`; `sa.admin` `setModerationSwitches` tắt `steps.spam` cho `pgd-demo` với `reason`, `expires_at` +5 phút → `pending_approval`, chưa hiệu lực; `sa.admin` tự duyệt → `403 MODERATION_SWITCH_SELF_APPROVAL`; `sa.admin2` duyệt → tin spam ở `pgd-demo` gửi được (`steps_skipped=[spam]`), ở `th-demo-a` vẫn bị chặn, tin có SĐT ở `pgd-demo` vẫn bị chặn; sau 5 phút tự bật lại, có audit `auto_expire`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F116-AC-20 | KHI System Admin (`chat.policy.toggle`) gọi `setModerationSwitches` cho platform hoặc tenant, module chat PHẢI lưu `enabled_all`, `steps.{sensitive_terms,spam,bullying,async_analysis}`, `reason`, `expires_at`, ghi `moderation_switch_changes` (before/after), audit, phát `chat_policy.moderation_switch_changed`, và giá trị hiệu lực PHẢI áp trên mọi instance trong ≤ 60 giây sau khi có hiệu lực (yêu cầu tắt chỉ hiệu lực sau phê duyệt, F116-AC-32; bật lại hiệu lực ngay) | FR-MOD-011 · AC-MOD-021 | TC-CHAT-016 |
| F116-AC-21 | NẾU tắt bất kỳ bước nào mà thiếu `reason` (< 10 ký tự), THÌ module chat PHẢI trả `422 MODERATION_SWITCH_REASON_REQUIRED`; NẾU người gọi không có `chat.policy.toggle`, THÌ PHẢI trả `403 PERMISSION_DENIED` | FR-MOD-011 · AC-MOD-016 | TC-CHAT-015 |
| F116-AC-22 | HỆ THỐNG PHẢI tính bước hiệu lực = `platform.enabled_all ∧ platform.steps[x] ∧ tenant.enabled_all ∧ tenant.steps[x]` (tenant chưa cấu hình kế thừa platform); NẾU cache công tắc mất, THÌ PHẢI đọc PostgreSQL và không bao giờ mặc định tắt | FR-MOD-011 · ADR-0030 · AGENTS R15 | TC-CHAT-013 |
| F116-AC-23 | TRONG KHI một bước bị tắt cho tenant, module chat PHẢI bỏ qua đúng bước đó, ghi `steps_skipped`, decision `unchecked` khi không còn bước nội dung nào chạy, và tenant khác không bị ảnh hưởng | FR-MOD-011 · AC-MOD-021 | TC-CHAT-013 |
| F116-AC-24 | TRONG KHI `enabled_all` tắt, module chat PHẢI vẫn áp độ dài tối đa, rate limit, quyền channel, sanction, block/mute của người dùng | FR-MOD-012 · AC-MOD-021 | TC-CHAT-014 |
| F116-AC-25 | KHI tới `expires_at`, job `moderation-switch-expirer` PHẢI bật lại bước, ghi `source = auto_expire` + audit, phát event; metric `eduworld_chat_policy_disabled{tenant,step}` PHẢI = 1 trong suốt thời gian tắt | FR-MOD-014 · FR-MOD-011 | TC-CHAT-016 |
| F116-AC-26 | HỆ THỐNG PHẢI không có cờ tắt kiểm duyệt nào trong code, biến môi trường hay cấu hình triển khai; công tắc chỉ đọc từ cấu hình hiệu lực trong `eduworld_chat` | FR-MOD-011 · AGENTS R15 | TC-CHAT-013 |
| F116-AC-30 | HỆ THỐNG PHẢI chạy bước PII và phát hiện grooming (đồng bộ và luật grooming bất đồng bộ) trên 100% tin ở mọi channel, mọi tenant, kể cả khi `enabled_all` tắt; NẾU `setModerationSwitches` yêu cầu tắt `pii` hoặc grooming ở bất kỳ cấp nào, THÌ module chat PHẢI trả `422 MODERATION_SWITCH_STEP_NOT_ALLOWED`; cấu hình cũ `steps.pii = false` PHẢI bị bỏ qua khi resolve | NFR-SAFE-004 · FR-MOD-012 · AC-MOD-030 · [MODERATION §12.2](../../docs/03-social-multiplayer/MODERATION.md#122-quy-tắc-thao-tác) · ADR-0078 | TC-CHAT-044 |
| F116-AC-31 | NẾU yêu cầu tắt một bước hoặc `enabled_all` thiếu `expires_at` hoặc `expires_at` vượt `now + moderation_switch_max_duration`, THÌ module chat PHẢI trả `422 MODERATION_SWITCH_EXPIRY_REQUIRED` | FR-MOD-014 · [MODERATION §12.2](../../docs/03-social-multiplayer/MODERATION.md#122-quy-tắc-thao-tác) · ADR-0078 | TC-CHAT-045 |
| F116-AC-32 | KHI System Admin gửi yêu cầu tắt hợp lệ, module chat PHẢI lưu yêu cầu `pending_approval` (chưa đổi giá trị hiệu lực) và chỉ áp khi một System Admin khác gọi `approveModerationSwitchChange` (đủ `moderation_switch_approvals`), ghi audit người đề xuất + người duyệt; NẾU người đề xuất tự duyệt, THÌ PHẢI trả `403 MODERATION_SWITCH_SELF_APPROVAL`; yêu cầu không được duyệt trong 24 giờ PHẢI tự hết hạn; yêu cầu bật lại PHẢI hiệu lực ngay không cần duyệt | FR-MOD-011 · [MODERATION §12.2](../../docs/03-social-multiplayer/MODERATION.md#122-quy-tắc-thao-tác) · ADR-0078 | TC-CHAT-045 |

---

### User Story 4 — Quick message song ngữ, không dịch máy chat (Priority: P2)

Học sinh lớp 2 chạm "Cố lên!"; bạn chọn tiếng Anh thấy "Go for it!". Tin tự gõ hiển thị nguyên văn; tin tiếng Anh tục tĩu vẫn bị chặn dù người gửi dùng giao diện tiếng Việt.

**Why this priority**: Kênh giao tiếp an toàn nhất cho học sinh nhỏ (US-CHAT-001); song ngữ theo ADR-0056.

**Independent Test**: `listQuickMessages` trả đủ `text.vi`/`text.en`; An (`vi`) gửi `quick_message_id`, Bình (`en`) nhận đúng mã và hiển thị bản `en`; tham số quick chứa SĐT bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F116-AC-27 | KHI học sinh gọi `listQuickMessages` (`channel_type?`), module chat PHẢI trả danh mục quick message đã duyệt có đủ `vi` và `en`, giống nhau cho mọi lớp | NFR-I18N-004 · US-CHAT-001 · ADR-0056 | TC-I18N-008 |
| F116-AC-28 | KHI gửi `kind = quick`, module chat PHẢI kiểm `quick_message_id` tồn tại và đang hoạt động (sai → `VALIDATION_FAILED`), phát tin bằng mã + tham số (không phát chuỗi đã dịch), và tham số PHẢI qua pipeline như text | NFR-I18N-004 · FR-CHAT-005 | TC-I18N-008 |
| F116-AC-29 | HỆ THỐNG PHẢI không dịch máy tin tự do; pipeline PHẢI so khớp cả danh mục `vi` và `en` trên mọi tin, không phụ thuộc ngôn ngữ giao diện của người gửi | NFR-I18N-004 · [CHAT_SYSTEM §8.6](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-I18N-009 |

### Edge Cases

- NẾU pattern RE2 chạy quá 5 ms, THÌ bỏ qua pattern đó an toàn (không chặn nhầm cả tin), tăng metric và ghi log không nội dung (TC-CHAT-011).
- NẾU bản biên dịch danh mục không nạp được khi khởi động và không có bản cache, THÌ instance không nhận lưu lượng (readiness fail), không chạy với danh mục rỗng.
- NẾU "ngủ ngon" (có dấu) chứa chuỗi trùng "ngu", THÌ không bị khớp (bản không dấu chỉ áp khi văn bản gốc không dấu) [Confirmed] (AC-MOD-002).
- NẾU cấu hình cũ có `steps.pii = false`, THÌ bị bỏ qua khi resolve và migration đưa về bật (ADR-0078, F116-AC-30).
- NẾU yêu cầu tắt `pending_approval` quá 24 giờ chưa được duyệt, THÌ tự hết hạn, không bao giờ có hiệu lực (F116-AC-32).
- NẾU Valkey pub/sub mất thông báo version, THÌ đối chiếu `ew:chat:sensitive:current` mỗi 60 s.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-005 | Pipeline đồng bộ mọi `kind`, quyết định, `block` không lưu nội dung | [MODERATION §3](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-001 | Lọc profanity, PII, anti-spam (gồm auto-mute), bắt nạt (rule đồng bộ) | [MODERATION §3.1–§3.5](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-009 | Danh mục category/term/allowlist/version, biến thể tự sinh, test chuỗi | [MODERATION §11](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-010 | Action theo severity, không lộ từ khớp | [MODERATION §11.2](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-011 | Công tắc tổng/từng bước, chỉ System Admin, lý do, audit, metric/alert | [MODERATION §12](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-012 | Giới hạn kỹ thuật và sanction vẫn chạy khi tắt | [MODERATION §12.3](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-013 | Từ bổ sung tenant (Safety Team), bật/tắt category khi `allow_tenant_toggle` | [MODERATION §11.3](../../docs/03-social-multiplayer/MODERATION.md) · ADR-0072 |
| FR-MOD-014 | `expires_at` tự bật lại | [MODERATION §12.2](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-015 | Test hồi quy trước publish, reload ≤ 60 s | [MODERATION §11.4, §11.6](../../docs/03-social-multiplayer/MODERATION.md) |
| NFR-SAFE-002 | Ngân sách độ trễ pipeline | [MODERATION §3](../../docs/03-social-multiplayer/MODERATION.md) |
| NFR-I18N-004 | Quick message theo ngôn ngữ người xem, không dịch máy, danh mục `vi` + `en` | [CHAT_SYSTEM §8.5–§8.6](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |

### Key Entities

- **SensitiveTermCategory / SensitiveTerm / SensitiveAllowlistEntry / SensitiveCatalogVersion**: danh mục platform và phần bổ sung tenant, có version bất biến.
- **TenantCategorySetting**: trạng thái bật/tắt một category trong tenant.
- **ModerationSwitch / ModerationSwitchChange**: công tắc hiệu lực theo platform/tenant và lịch sử thay đổi.
- **ModerationDecision**: metadata quyết định của mỗi tin, không nội dung.
- **Sanction**: hạn chế gửi của một học sinh (F116: `mute` hệ thống).
- **QuickMessage**: câu soạn sẵn song ngữ đã duyệt.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `moderation_switches` | Giá trị mặc định công tắc (bật tất cả) |
| `moderation_pipeline_latency` | Ngân sách p95/trần pipeline |
| `filter_quality_gate` | Ngưỡng recall/false positive khi publish |
| `auto_mute_spam` | Thời hạn auto-mute khi vi phạm liên tiếp |
| `chat_max_length`, `chat_rate_per_user` | Giới hạn kỹ thuật luôn chạy |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không hardcode từ nhạy cảm trong code/test/seed (fixture placeholder); không trả từ khớp | AGENTS R15 · ADR-0030 |
| Không cờ tắt kiểm duyệt trong code/env | AGENTS R15 |
| Endpoint quản trị chỉ gateway nội bộ, `azp = eduworld-portal` + MFA + role platform | AGENTS R20 · ADR-0027 |
| Xóa = soft-delete; version danh mục append-only | AGENTS R21 · ADR-0028 |
| Pipeline in-process, p95 theo `moderation_pipeline_latency` | NFR-SAFE-002 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Trên bộ ≥ 2.000 câu mẫu gán nhãn, recall với `high`/`critical`/PII và false positive đạt `filter_quality_gate`.
- **SC-002**: 100% phản hồi chặn/che trong bộ test không chứa từ đã khớp hay vị trí khớp.
- **SC-003**: Term mới có hiệu lực trên mọi instance trong ≤ 60 giây sau publish, đo trong load test có tải chat.
- **SC-004**: 100% lần bật/tắt công tắc có lý do, audit và metric cảnh báo; 0 tenant bị ảnh hưởng ngoài tenant đích.

## Ngoài phạm vi

- Tạo moderation case, hàng đợi Safety Team, sanction thủ công, report, phân tích bất đồng bộ (`async_analysis` worker), thông báo nhà trường → F121.
- Giao diện Portal danh mục/công tắc/nhóm từ tenant → F122; UI phản hồi kiểm duyệt trong game → F119.
- Chạy lại pipeline khi sửa/forward/quote → F118 (dùng lại bộ kiểm tra của F116).
- Tên party/đội/study group qua pipeline → F100/F120 gọi cùng port.
- Mô hình ML/LLM phân loại bắt nạt: không có trong Release 1.0 (owner chốt 2026-11-20); làm sau cần ADR mới theo MODERATION §4.

## Assumptions

- Bảng teencode/leetspeak dùng cho normalize nạp từ cấu hình có version trong `src/backend/modules/chat/seed/normalization/` (không chứa từ nhạy cảm); API quản lý chưa có trong contract → ghi "Cần thêm" ở contracts §6 [Confirmed].
- Bộ test hồi quy ≥ 2.000 câu là dữ liệu vận hành lưu ngoài repo (object storage, quyền `chat.sensitive_terms.read`); CI dùng bộ placeholder không chứa từ thật (R15) [Confirmed].
- Thời gian hiệu lực ≤ 60 s, timeout RE2 5 ms, ngưỡng spam (3 tin/60 s, 4 channel/2 phút, 3 block/10 phút) chưa có tên trong CONFIG_DEFAULTS → task Polish đề xuất đăng ký [Confirmed].
- Ánh xạ severity → action và chi tiết pipeline là [Confirmed] theo PRD FR-MOD-010, FR-CHAT-005.
