# Feature Specification: F082 — Game Studio — quản lý sticker/GIF và dịch nội dung game (`/game-admin/translations*`)

**Feature Branch**: `082-game-studio-chat-assets-translations` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/apps/portal` (workspace A `game-ops`) + module `game` (trạng thái bản dịch, job xuất/nhập file dịch) · **Mốc**: M4b · **Phụ thuộc**: F079, F118 (và F078 workflow, F053 `BilingualField`)
**Input**: Trong Game Operations Console (platform-only, gateway nội bộ + MFA): màn "Tài nguyên chat" quản lý sticker pack và thư viện GIF nội bộ trên backend module chat của F118 (upload presigned, alt text/tag song ngữ, mở khóa bằng phần thưởng, duyệt người khác người upload, publish/rollback); màn "Dịch nội dung game" với trạng thái bản dịch (`complete`/`partial`/`outdated`) cho mọi cấu hình module game, biên tập hai cột vi/en, xuất/nhập XLIFF 2.1/CSV qua `/api/v1/game-admin/translations*`. FR nguồn: FR-CHAT-014, FR-CNT-018, NFR-I18N-002, FR-CMS-006, FR-CMS-011.

## Clarifications

### Session 2026-11-16

- Q: Backend sticker/GIF đã có chưa? → A: Có — F118 (module chat, `game-admin/{sticker-packs|gifs}` qua `x-service-routing`, kiểm tệp, duyệt, version). Presign asset chat và JSON Schema payload là delta owner F118 ([contracts](contracts/README.md) §6); F082 tạo contract nếu F118 chưa làm, không viết lại backend.
- Q: Backend dịch nội dung game thuộc feature nào? → A: Chưa có (F078 ghi ngoài phạm vi). F082 hiện thực trong module game: `exportGameTranslations`, `importGameTranslations`, `getGameTranslationJob` (contract có sẵn) và tính `translation_status` khi lưu draft.
- Q: File dịch có gồm sticker/GIF không? → A: Không — `TranslationExportRequest.content_types` là `config_type` của module game; tên pack, alt text, tag GIF soạn song ngữ trực tiếp trong form.
- Q: Ngôn ngữ gốc của văn bản game? → A: [Confirmed] `vi` (ADR-0056 mục 1); cả `vi` và `en` bắt buộc trước khi gửi duyệt (`game_text_required_languages`).
- Q: File nhập dịch upload thế nào? → A: [Confirmed] `presignGameMediaUpload` với `kind = document` (XLIFF/CSV); file không gắn `media_usages`, không phát hành CDN.
- Q: Nhập dịch khi item đã có version mới hơn version trong khóa? → A: Bỏ dòng, báo `stale_version` ([research](research.md) R2).

### Session 2026-11-20 (owner chốt)

- Game Dashboard → **Chốt:** thuộc F079 (đếm item cần dịch theo `translation_status`).
- Platform-ops (gán/clone/Thùng rác) cho sticker pack/GIF của module chat (OQ F045-3) → **Chốt:** F082 hiện thực facade platform-ops của module chat cho hai loại này (hộp gán ở F057, saga ở F045).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Quản lý sticker pack (Priority: P1) 🎯 MVP

Game Designer tạo pack "Bạn đồng hành Toán học" 8 sticker, alt text vi/en, mở khóa khi hoàn thành quest; gửi duyệt; người khác duyệt; Live-ops publish; System Admin gán cho tenant bằng hộp gán chung.

**Why this priority**: FR-CHAT-014 — sticker là kênh biểu đạt chính của học sinh lớp nhỏ; không có UI thì thư viện không có nội dung.

**Independent Test**: Trên module chat F118 thật: tạo pack, upload 2 webp, thiếu alt text `en` → bị chặn gửi duyệt; tệp quá lớn → lỗi tại tệp; publish → học sinh `listStickerPacks` thấy pack (khi đã gán).

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F082-AC-01 | KHI người có `game.chat_assets.write` mở "Tài nguyên chat → Sticker pack", Portal PHẢI hiển thị `listGameConfigs` (`config_type = sticker-packs`) với mã, tên vi/en, số sticker, ảnh đại diện, kiểu mở khóa, trạng thái (Tag token), version; mục menu "Tài nguyên chat" PHẢI chỉ hiện khi có permission đó | FR-CHAT-014 · [ADMIN_PORTAL §3.3, §10](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F082-AC-02 | KHI soạn pack, Portal PHẢI dùng `ContentEditorShell` với `code`, tên `vi`/`en` (`BilingualField`), `unlock_rule` (mặc định / reward rule / item — chọn từ `reward-rules`, `items` đã publish, không nhập ID tự do), danh sách sticker kéo-thả thứ tự, alt text `vi`/`en` từng sticker; NẾU thiếu bản `vi` hoặc `en`, THÌ PHẢI khóa Gửi duyệt và liệt kê trường thiếu (server vẫn kiểm) | FR-CHAT-014 · NFR-I18N-002 · ADR-0034 · ADR-0056 | TC-I18N-001 |
| F082-AC-03 | KHI tải sticker hoặc GIF, Portal PHẢI xin presign qua `presignChatAssetUpload` với `Idempotency-Key` và `sha256` tính ở trình duyệt, PUT tệp lên `upload_url` không kèm `Authorization`, hiển thị tiến trình từng tệp; NẾU server trả `413 MEDIA_TOO_LARGE`, `415 MEDIA_TYPE_NOT_ALLOWED` hoặc `422 VALIDATION_FAILED`, THÌ PHẢI báo lỗi tại đúng tệp theo thông điệp server (Portal không tự đặt giới hạn số) và cho tải tệp khác | FR-CHAT-014 · ADR-0044 · AGENTS R1, R33 | — |
| F082-AC-04 | KHI chuyển trạng thái pack/GIF, Portal PHẢI gọi `transitionGameConfigVersion`, `publishGameConfigVersion` (ngay/hẹn giờ), `rollbackGameConfig`/`unpublishGameConfig` (lý do bắt buộc) với `Idempotency-Key` và `updateGameConfigDraft` với `If-Match`; version đã publish PHẢI chỉ đọc; nút Duyệt PHẢI ẩn với tác giả/người upload; nút "Gán tenant" PHẢI chỉ mở hộp gán chung của F057 khi có `platform.tenant_data.assign` | FR-CMS-006 · ADR-0043 · ADR-0029 | TC-CMS-001 |

---

### User Story 2 — Thư viện GIF nội bộ (Priority: P1)

Game Designer upload 20 GIF chủ đề học tập, gắn tag vi/en; người duyệt khác duyệt; GIF lỗi thời được lưu trữ.

**Why this priority**: FR-CHAT-014 — GIF chỉ từ thư viện đã duyệt, không dịch vụ ngoài.

**Independent Test**: Người upload mở GIF của mình → không có nút Duyệt; gọi API duyệt trực tiếp → server từ chối; giao diện không có ô URL/tìm GIF ngoài.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F082-AC-05 | KHI mở thư viện GIF, Portal PHẢI hiển thị lưới ảnh tĩnh `preview_url` (bấm để phát), lọc theo tag, trạng thái, người upload; cho upload nhiều tệp (theo F082-AC-03) và gắn tag `vi`/`en` mỗi tag ≤ `maxLength` của schema | FR-CHAT-014 · [CHAT_SYSTEM §7.5](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | — |
| F082-AC-06 | TRONG KHI người dùng là người upload GIF, Portal PHẢI ẩn Duyệt; NẾU server từ chối duyệt (`403 PERMISSION_DENIED`/`422 VALIDATION_FAILED`), THÌ PHẢI hiển thị lý do theo `code`; Portal PHẢI không có ô nhập URL ảnh ngoài hay tìm kiếm dịch vụ GIF bên thứ ba | FR-CHAT-014 · ADR-0034 · AC-CHAT-013 | TC-CHAT-033 |
| F082-AC-07 | KHI xem trước pack/GIF, Portal PHẢI hiển thị như học sinh thấy (lưới sticker, alt text theo `vi`/`en`, nhãn khóa/mở khóa) chỉ từ URL ký do module chat trả, và PHẢI không dùng component/Theme game ngoài `GamePreviewFrame` | ADR-0047 · AGENTS R32 | TC-UI-010 |

---

### User Story 3 — Trạng thái bản dịch nội dung game (Priority: P1)

Người dịch lọc "Cần dịch" trong Quest Builder, mở quest, điền cột `en` cạnh cột `vi` "Gốc"; sửa bản gốc của một câu thoại NPC đã dịch → trường đó hiện "Lỗi thời" kèm diff.

**Why this priority**: FR-CNT-018, NFR-I18N-002 — nội dung game không thể thiếu bản dịch khi publish; cần biết chỗ thiếu trước khi gửi duyệt.

**Independent Test**: Lưu draft thiếu `en` → `partial` + `missing_fields`; điền đủ → `complete`; sửa `vi` sau khi đã dịch → `outdated` + `outdated_fields`; lọc danh sách theo trạng thái.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F082-AC-08 | KHI lưu draft (`createGameConfig`, `updateGameConfigDraft`) của `config_type` module game, module game PHẢI tính `translation_status` (`complete`/`partial`/`outdated`) theo [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) trên mọi JSON pointer văn bản game, lưu cùng version, và trả `translation_status`, `missing_fields[]`, `outdated_fields[]` {`pointer`, `translated_in_version`} trong `GameConfigVersion` | FR-CNT-018 · NFR-I18N-002 | TC-I18N-010 |
| F082-AC-09 | KHI gọi `listGameConfigs` với `translation_status`, module game PHẢI lọc theo trạng thái version mới nhất của item; Portal PHẢI có bộ lọc "Cần dịch" (`partial`), "Bản dịch lỗi thời" (`outdated`) và huy hiệu Đủ/Thiếu/Lỗi thời ở danh sách và editor | FR-CNT-018 · [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F082-AC-10 | KHI mở editor cấu hình game, Portal PHẢI hiển thị mọi trường `localized_text` bằng `BilingualField` hai cột (`vi` có nhãn "Gốc" đứng trước; màn hẹp thành tab), và với trường lỗi thời PHẢI cho xem diff bản gốc giữa `translated_in_version` và version hiện tại qua `diffGameConfigVersions`; HỆ THỐNG PHẢI không dịch máy tự động | FR-CNT-018 · ADR-0056 · [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-I18N-010 |
| F082-AC-11 | NẾU version còn `missing_fields`, THÌ Portal PHẢI khóa Gửi duyệt/Publish và liệt kê trường thiếu bấm để nhảy tới (server vẫn trả `422 TRANSLATION_INCOMPLETE`); NẾU chỉ có `outdated_fields`, THÌ PHẢI hiển thị cảnh báo trong hộp xác nhận và vẫn cho gửi | FR-CNT-018 · NFR-I18N-002 · [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) | TC-I18N-001 |

---

### User Story 4 — Xuất/nhập file dịch (Priority: P2)

Người dịch xuất 120 quest/NPC dialogue cần dịch ra XLIFF 2.1, gửi đối tác dịch, nhập lại: chạy thử xem báo cáo, rồi nhập thật → mỗi item khớp khóa có Draft mới.

**Why this priority**: FR-CNT-018 (xuất/nhập file dịch); giảm chi phí dịch hàng loạt, sau US3.

**Independent Test**: Xuất → file có khóa `<config_type>/<config_id>/<version>/<json-pointer>`; nhập `dry_run` → không có version mới; nhập thật → Draft mới, version published không đổi; dòng sai khóa → báo cáo, job vẫn `succeeded`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F082-AC-12 | KHI người có `game.content.translate` gọi `exportGameTranslations` với `Idempotency-Key`, module game PHẢI tạo job `202` (`queued`), worker PHẢI sinh XLIFF 2.1 hoặc CSV chỉ gồm trường văn bản game của version mới nhất mỗi item (mặc định chỉ trường thiếu/lỗi thời), khóa ổn định `<config_type>/<config_id>/<version>/<json-pointer>`, lưu vào object storage `ew-exports`; `getGameTranslationJob` PHẢI trả `download_url` ký có hạn khi `succeeded` | FR-CNT-018 · [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-I18N-010 |
| F082-AC-13 | KHI gọi `importGameTranslations` (`asset_id` từ `presignGameMediaUpload`), module game PHẢI chuẩn hóa NFC, với mỗi khóa khớp version mới nhất PHẢI tạo version Draft mới dựa trên version đó (hoặc ghi vào chính Draft đang mở) qua cùng command lưu draft của workflow, không bao giờ sửa version đã publish, tính lại `translation_status`, ghi audit; với `dry_run = true` PHẢI chỉ trả báo cáo, không ghi | FR-CNT-018 · ADR-0043 · AGENTS R11 | TC-I18N-010 |
| F082-AC-14 | NẾU dòng có khóa không tồn tại, version cũ hơn version mới nhất (`stale_version`), version đang `in_review`, `config_type` không thuộc module game, hoặc chuỗi vượt `maxLength` của schema, THÌ module game PHẢI bỏ dòng và ghi `report.errors[]` (`ref`, `message`) mà job vẫn chạy tiếp; NẾU file sai định dạng, THÌ job PHẢI `failed` có thông điệp; cùng `Idempotency-Key` PHẢI trả lại cùng job | FR-CNT-018 · AGENTS R10 | TC-I18N-010 |
| F082-AC-15 | KHI mở "Dịch nội dung game", Portal PHẢI có dialog Xuất (loại `content_types`, item `content_ids` ≤ 500, ngôn ngữ đích, định dạng, chỉ trường thiếu), theo dõi job, link tải kèm thời hạn; dialog Nhập: upload file → chạy thử (`dry_run`) → xem báo cáo → xác nhận nhập; mỗi ý định một `Idempotency-Key` giữ khi thử lại | FR-CNT-018 · AGENTS R10 | TC-I18N-010 |

---

### User Story 5 — Bảo vệ, phân quyền, giới hạn (Priority: P1)

**Why this priority**: FR-CMS-011 — role tenant không chạm Game Studio; constitution điều III, V.

**Independent Test**: `ad.truong.a`, `sm.antoan` mở `/game-ops/chat-assets`, `/game-ops/translations` → 403, không tải chunk; gọi API dịch bằng token thiếu MFA → `403 MFA_REQUIRED`; khóa trỏ tới sự kiện trường của tenant → báo lỗi như không tồn tại.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F082-AC-16 | NẾU người gọi `/game-admin/translations*` không có `game.content.translate`, thiếu MFA, `azp` khác `eduworld-portal` hoặc không có role platform, THÌ module game PHẢI trả `403 PERMISSION_DENIED`/`403 MFA_REQUIRED` và ghi `authz.denied`; `getGameTranslationJob` PHẢI chỉ trả job của chính người tạo (System Admin xem mọi job), còn lại `404 NOT_FOUND` | AGENTS R7, R20 · ADR-0027 | TC-CMS-WS-02 |
| F082-AC-17 | module game PHẢI chỉ xuất/nhập cấu hình dữ liệu platform; khóa hay `content_ids` trỏ tới bản ghi của tenant (vd. sự kiện trường) PHẢI bị xử lý như không tồn tại (không đọc, không ghi, không lộ tồn tại); route `/game-admin/translations*` gọi qua gateway public PHẢI bị chặn tại APISIX | AGENTS R8, R27 · ADR-0005 | TC-CMS-WS-03 |
| F082-AC-18 | KHI người dùng không có `game.chat_assets.write`/`game.content.translate` mở URL màn tương ứng, Portal PHẢI hiển thị trang 403; role tenant PHẢI không tải được chunk workspace A; NẾU API trả `403 MFA_REQUIRED`, THÌ PHẢI step-up MFA; NẾU gateway nội bộ không kết nối được, THÌ PHẢI hiển thị "Cần kết nối mạng quản trị (VPN)" | FR-CMS-011 · ADR-0027 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-01 |
| F082-AC-19 | HỆ THỐNG PHẢI giới hạn `content_types` ≤ 20, `content_ids` ≤ 500, tag GIF, tên pack, alt text và chuỗi trong file dịch theo `maxLength`/`maxItems` của schema, trả `422 VALIDATION_FAILED` chỉ rõ field; Portal PHẢI hiển thị bộ đếm ký tự và lỗi theo `code` + `trace_id` | ADR-0041 · AGENTS R28 | TC-TEN-024 |
| F082-AC-20 | KHI menu/action của feature (`menu.game_ops.chat_assets`, `menu.game_ops.translations`, `menu.game_ops.media` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU sticker pack bị gỡ gán khỏi tenant hoặc unpublish, THÌ tin cũ vẫn hiển thị (asset bất biến, F118); UI cảnh báo trong hộp xác nhận unpublish.
- NẾU reward rule/item được chọn làm `unlock_rule` bị unpublish sau đó, THÌ editor pack hiển thị cảnh báo "Tham chiếu không còn được phát hành" theo lỗi server; không tự đổi rule.
- NẾU upload bị gián đoạn, THÌ thử lại dùng cùng `Idempotency-Key` presign (server trả `duplicate_of` nếu trùng sha256).
- NẾU nhập file có cả khóa đúng và khóa sai, THÌ job `succeeded` với báo cáo từng dòng (F082-AC-14).
- NẾU hai người nhập dịch cùng item đồng thời, THÌ một job ghi trước, job sau báo `stale_version` cho các dòng đó.
- NẾU loại cấu hình không có trường văn bản game (vd. `currencies` chỉ mã), THÌ `translation_status = complete` và không xuất hiện trong file xuất.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-014 | UI Game Studio cho sticker pack (mở khóa bằng phần thưởng) và thư viện GIF nội bộ đã duyệt | [CHAT_SYSTEM §7.5](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · [ADMIN_PORTAL §3.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) · ADR-0034 |
| FR-CNT-018 | `translation_status` cho nội dung game, chặn khi thiếu bản bắt buộc, xuất/nhập file dịch | [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) · [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| NFR-I18N-002 | Văn bản game đủ `vi` + `en` trước khi publish (phần sticker/GIF và biên tập dịch) | [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) · ADR-0056 |
| FR-CMS-006 | Workflow, version, publish, rollback cho sticker pack/GIF trên UI | [ADMIN_PORTAL §1.4, §4–§6](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-011 | Lớp chặn bundle + 403/MFA cho màn mới của workspace A | [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **StickerPack / Sticker**: pack platform có version, tên song ngữ, quy tắc mở khóa, sticker có alt text song ngữ (module chat).
- **GifAsset**: GIF nội bộ có preview tĩnh, tag song ngữ, người upload ≠ người duyệt (module chat).
- **TranslationState**: trạng thái bản dịch của một version cấu hình game, trường thiếu/lỗi thời.
- **TranslationJob**: job xuất/nhập file dịch, file kết quả, báo cáo từng dòng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `game_text_required_languages` | Ngôn ngữ bắt buộc của văn bản game khi tính trạng thái/khóa gửi duyệt |
| `report_link_ttl` | [Confirmed] Thời hạn link tải file dịch (dùng chung với link báo cáo) |
| `idempotency_record_ttl` | Thời gian giữ kết quả `Idempotency-Key` của job |
| `portal_session_sso` | Phiên platform, step-up MFA |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Workspace A chỉ gateway nội bộ + MFA; chunk lazy-load không phải lớp bảo mật | AGENTS R20 · ADR-0027 |
| Không upload của học sinh, không dịch vụ GIF bên thứ ba | ADR-0034 · [CHAT_SYSTEM §7.5](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| Không sửa version đã publish; nhập dịch tạo Draft | AGENTS R11, R31 · ADR-0043 |
| File chỉ ở object storage, không blob trong DB | AGENTS R31 · ADR-0044 |
| Không business logic trong UI (giới hạn tệp, trạng thái dịch tính ở server) | AGENTS R1 |
| WCAG 2.1 AA; alt text bắt buộc | NFR-A11Y-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Game Designer tạo pack 8 sticker có alt text song ngữ và gửi duyệt trong ≤ 10 phút.
- **SC-002**: Vòng xuất → sửa → nhập 100 item: 100% khóa hợp lệ tạo đúng một Draft; 0 version đã publish bị thay đổi.
- **SC-003**: 0 sticker/GIF ngoài thư viện đã duyệt; 0 ô nhập URL ảnh ngoài trong giao diện (kiểm tự động).
- **SC-004**: 0 role tenant tải được chunk workspace A hoặc gọi được `/game-admin/translations*` (E2E + API).
- **SC-005**: Người dịch tìm được mọi item "Cần dịch" của một loại trong ≤ 2 thao tác lọc.

## Ngoài phạm vi

- Backend sticker/GIF (kiểm tệp, version, duyệt, gửi `kind = sticker|gif`, mở khóa projection): F118. Phần thưởng mở khóa (reward rule, inventory): F075.
- Hộp gán sticker pack/GIF cho tenant và saga gán: F057/F045.
- Workflow chung `/game-admin/{config_type}`, chặn `TRANSLATION_INCOMPLETE` khi gửi duyệt: F078; `ContentEditorShell`, builder từng loại: F079.
- Dịch nội dung học tập (`/cms/learning/translations*`): F025/F053. Chuỗi UI game (`.po`): F060. Dịch máy: không làm (ADMIN_PORTAL §1.5).
- Bảng điều khiển Game Dashboard (đếm số item cần dịch): F079 (owner chốt 2026-11-20), đếm theo `translation_status` của F082.

## Assumptions

- [Confirmed] Route `/game-ops/chat-assets/sticker-packs`, `/game-ops/chat-assets/gifs`, `/game-ops/translations` trong chunk `game-ops` (F051).
- [Confirmed] Duyệt sticker/GIF dùng `game.chat_assets.write` (người khác tác giả) theo ADMIN_PORTAL §3.3; publish theo bảng quyền của F118.
- [Confirmed] Thời hạn link tải dùng `report_link_ttl`; file kết quả giữ theo cấu hình bucket `ew-exports` (đề xuất đăng ký tham số riêng cho file dịch ở Polish).
- [Confirmed] Component dialog dịch của F053 (`TranslationExportDialog`, `TranslationImportDialog`) được tách phần trình bày sang `ops-ui` để workspace A dùng lại.
