# Feature Specification: F070 — Hồ sơ người chơi, nickname, tạo avatar lần đầu, trang phục, emote

**Feature Branch**: `070-player-profile-avatar` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `game` (+ `src/game/`) · **Mốc**: M2 · **Phụ thuộc**: F014, F062
**Input**: Học sinh vào game lần đầu tạo nhân vật (preset, dáng, màu da/tóc, trang phục mặc định), chọn nickname từ danh sách sinh tự động; sau đó đổi trang phục, phụ kiện, emote trong Tủ đồ; người khác chỉ thấy nickname/avatar/level. FR nguồn: FR-GAME-004, FR-GAME-005, FR-GAME-015.

## Clarifications

### Session 2026-11-16

- Q: GameProfile được tạo khi nào? → A: Trong transaction `StartGameSession` của F014 (bước `EnsureGameProfile`, cùng cập nhật `last_game_login_at`), không qua event identity, để không có độ trễ trước màn tạo nhân vật. Profile mới có `avatar_config = NULL`, `nickname = NULL`.
- Q: Shape `AvatarConfig` theo OpenAPI (`hair_item_id` UUID, `emote_item_ids` ≤ 8) hay CHARACTER_SYSTEM §5.1 (`item_id` mã chuỗi, `emote_slots` ≤ 6, slot phụ kiện)? → A: Theo tài liệu sở hữu [CHARACTER_SYSTEM §5.1](../../docs/01-game-design/CHARACTER_SYSTEM.md#51-data-model-khớp-domain_model-5); `item_id` là mã `varchar(64)` như `item_ledger`, `quest.schema.json`. Sửa OpenAPI ở Phase 2 (contracts §6).
- Q: Nickname duy nhất theo trường (DATABASE_DESIGN §2.8 [Confirmed]) hay theo tenant (`suggestNicknames`, STUDENT_GAME_UX_CORE S02)? → A: Theo tenant (chặt hơn, đúng với tương tác liên trường trong tenant `unit`, ADR-0054); cập nhật DATABASE_DESIGN ở Polish.
- Q: Danh mục preset, dáng người, bảng màu, đồ mặc định, bộ từ sinh nickname lưu ở đâu? → A: Nội dung platform có version (ADR-0043) `CharacterCatalog`, seed bởi `src/backend/modules/game/seed/`; không hardcode trong client (R2). Biên tập trên Game Operations Console → F078.
- Q: Giới hạn đổi nickname/tháng? → A: CHARACTER_SYSTEM §5.3 nêu "giới hạn số lần/tháng theo policy trường" nhưng chưa có giá trị; dùng tham số mới `nickname_change_monthly_limit` [Confirmed], đăng ký CONFIG_DEFAULTS ở Polish.

### Session 2026-11-17

- Q: Độ dài và phạm vi duy nhất của nickname? → A: Tối đa 32 ký tự ở mọi nơi (DB `varchar(32)`, OpenAPI/AsyncAPI `maxLength: 32`), duy nhất trong tenant ([CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname), RB-302).

### Session 2026-11-20 (owner chốt)

- Facade vòng đời học sinh của module game (OQ F016-2, F045-3) → **Chốt:** F070 (tạo `game_profiles`) hiện thực `cleanupStudentData`/`restoreStudentData` (soft-delete/khôi phục GameProfile, giữ chặn khi có session active), `anonymizeUserData` (nickname → giá trị ẩn danh, xóa avatar tùy biến không cần cho thống kê) và `mergeUserData` (chọn một GameProfile — profile user đích giữ, profile nguồn soft-delete; idempotent theo `merge_id`); gộp ledger XP ở F074.
- OQ-16 (asset 3D) → đã trả lời (owner 2026-11-20, OPEN_QUESTIONS): stylized low-poly, asset pack có license thương mại + tự làm nhân vật chính.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tạo nhân vật lần đầu (Priority: P1)

Học sinh lớp 2 đăng nhập lần đầu, chọn một trong các nhân vật mẫu bằng hình lớn, đổi màu tóc, chọn nickname gợi ý, xem nhân vật vẫy tay rồi vào Quảng trường Tia Sáng (US-GAME-001, AC-GAM-011).

**Why this priority**: Không có avatar thì không vào được Main Hub; là bước đầu của mọi phiên chơi.

**Independent Test**: Tài khoản `hs.an.3a` mới → `getGameProfile` có `setup_required = true` → `getCharacterCatalog` → `updateAvatar` + `updateGameProfile` → `setup_required = false`, đúng một `avatar.created`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F070-AC-01 | KHI học sinh gọi `getGameProfile` mà chưa có avatar hoặc nickname, module game PHẢI trả `setup_required = true`, `avatar_config = null`; game client PHẢI mở màn Tạo nhân vật (S02) trước Main Hub | FR-GAME-015 · AC-GAM-011 | TC-GAM-011 |
| F070-AC-02 | KHI học sinh gọi `getCharacterCatalog`, module game PHẢI trả danh mục đang publish gồm số preset theo `avatar_creation_presets`, dáng người, bảng màu da/tóc, trang phục/tóc/emote mặc định, tên hiển thị đủ `vi` và `en` | FR-GAME-015 · NFR-I18N-002 · [CHARACTER_SYSTEM §5.0](../../docs/01-game-design/CHARACTER_SYSTEM.md#50-tạo-avatar-lần-đầu) | TC-GAM-011 |
| F070-AC-03 | KHI học sinh lưu avatar lần đầu bằng `updateAvatar` hợp lệ, module game PHẢI lưu `avatar_config`, đặt `avatar_created_at` một lần, phát `avatar.created` đúng một lần (gọi lại không phát lại) | FR-GAME-015 · AC-GAM-011 | TC-GAM-011 |
| F070-AC-04 | NẾU `avatar_config` chứa `body_type`, `skin_tone`, màu tóc ngoài danh mục miễn phí, THÌ module game PHẢI trả `422 VALIDATION_FAILED` và không lưu | FR-GAME-015 · [CHARACTER_SYSTEM §5.1](../../docs/01-game-design/CHARACTER_SYSTEM.md#51-data-model-khớp-domain_model-5) | — |
| F070-AC-05 | Ở NƠI học sinh thuộc grade band `g1_3`, game client PHẢI cho hoàn tất tạo nhân vật trong ≤ 3 bước (mẫu → màu tóc → tên → xác nhận) không cần gõ phím, nút ≥ 64 px, có lồng tiếng hướng dẫn | FR-GAME-015 · FR-GAME-010 · [STUDENT_GAME_UX_CORE S02](../../docs/05-ux/STUDENT_GAME_UX_CORE.md#s02--tạo-avatar-lần-đầu) | TC-GAM-011 |
| F070-AC-06 | KHI học sinh đổi bất kỳ lựa chọn nào ở S02, game client PHẢI cập nhật xem trước 3D trong ≤ 100 ms trên máy tầm trung mà không gọi API | FR-GAME-015 · NFR-COMP-001 | — |

---

### User Story 2 — Nickname an toàn (Priority: P1)

Học sinh chọn nickname từ gợi ý sinh tự động (tính từ + con vật + số), không gõ tự do; tên thật không bao giờ hiện cho người chơi khác.

**Why this priority**: An toàn trẻ em (R16, CHILD_SAFETY EP-06) là điều kiện bắt buộc trước khi có multiplayer.

**Independent Test**: `suggestNicknames` → chọn `nickname_option_id` → `updateGameProfile`; học sinh khác cùng tenant chọn cùng nickname → `409`; gửi chuỗi nickname tự do → bị bỏ qua.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F070-AC-07 | KHI học sinh gọi `suggestNicknames`, module game PHẢI trả `count` gợi ý (mặc định 6, tối đa 10) sinh từ bộ từ đã duyệt của danh mục theo ngôn ngữ `Accept-Language`, mỗi gợi ý ≤ 24 ký tự và chưa được dùng trong tenant | FR-GAME-005 · [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname) | TC-GAM-011 |
| F070-AC-08 | KHI học sinh gửi `updateGameProfile` với `nickname_option_id` còn hiệu lực và do chính mình nhận, module game PHẢI đặt nickname đó, ghi lịch sử đổi tên và phát `game_profile.updated` | FR-GAME-005 | TC-GAM-011 |
| F070-AC-09 | NẾU `nickname_option_id` hết hạn, không thuộc học sinh, hoặc nickname vừa bị học sinh khác cùng tenant lấy, THÌ module game PHẢI trả `409 NICKNAME_UNAVAILABLE` và game client PHẢI tự lấy gợi ý mới | FR-GAME-005 · [STUDENT_GAME_UX_CORE S02](../../docs/05-ux/STUDENT_GAME_UX_CORE.md#s02--tạo-avatar-lần-đầu) | — |
| F070-AC-10 | HỆ THỐNG PHẢI không có đường nào đặt nickname bằng chuỗi tự do: trường `nickname` trong body `updateGameProfile` PHẢI bị từ chối `422 VALIDATION_FAILED` | FR-GAME-005 · AGENTS R16 | — |
| F070-AC-11 | NẾU học sinh đổi nickname (sau lần đặt đầu) vượt `nickname_change_monthly_limit` trong tháng, THÌ module game PHẢI trả `409 NICKNAME_CHANGE_LIMIT` kèm thời điểm đổi lại được | FR-GAME-005 · [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname) | — |
| F070-AC-12 | HỆ THỐNG PHẢI không đưa họ tên, username, trường, lớp vào `GameProfile` trả cho người khác và vào payload `game_profile.updated`/`avatar.updated` (chỉ `student_id`, nickname, `avatar_config_hash`, level) | FR-GAME-005 · AGENTS R16 · ADR-0054 | — |

---

### User Story 3 — Tủ đồ: trang phục, phụ kiện, bộ phối đồ (Priority: P2)

Học sinh mở Tủ đồ, mặc trang phục/tóc/phụ kiện mình sở hữu, lưu tối đa 3 bộ phối để đổi nhanh.

**Why this priority**: Phát triển nhân vật là động lực chính (GAME_DESIGN §5) nhưng cần sau khi có avatar.

**Independent Test**: `updateAvatar` với item mặc định → 200; item không sở hữu → `422 ITEM_NOT_OWNED`; item sai slot → `422 EQUIP_SLOT_INVALID`; `saveAvatarPreset` ô 1–3.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F070-AC-13 | KHI học sinh gọi `updateAvatar`, module game PHẢI chấp nhận `hair`, `outfit`, `accessories[]`, `emote_slots[]`, `pet`, `title`, `frame`, `badge_showcase[]` chỉ khi mỗi `item_id` là item mặc định của danh mục hoặc có trong inventory của học sinh; NẾU không, THÌ PHẢI trả `422 ITEM_NOT_OWNED` | FR-GAME-004 · AC-ECO-001…008 | TC-ECO-007 |
| F070-AC-14 | NẾU item gắn vào slot khác `equip_slot` của item, hoặc một slot có hơn một item, hoặc `emote_slots` vượt `emote_wheel_slots`, THÌ module game PHẢI trả `422 EQUIP_SLOT_INVALID` | FR-GAME-004 · [INVENTORY_ECONOMY §2.1](../../docs/01-game-design/INVENTORY_ECONOMY.md#21-loại-vật-phẩm) | TC-ECO-007 |
| F070-AC-15 | KHI avatar thay đổi, module game PHẢI tính lại `avatar_config_hash`, phát `avatar.updated` và `game_profile.updated` trong cùng transaction (outbox) | FR-GAME-004 · [CHARACTER_SYSTEM §8](../../docs/01-game-design/CHARACTER_SYSTEM.md#8-đồng-bộ-appearance-trong-multiplayer) | — |
| F070-AC-16 | KHI học sinh gọi `saveAvatarPreset` với `slot` 1…`saved_outfits_max` và `avatar_config` hợp lệ, module game PHẢI lưu đè ô đó; `listAvatarPresets` PHẢI trả tối đa `saved_outfits_max` bộ của chính học sinh | FR-GAME-015 | — |
| F070-AC-17 | NẾU chuỗi trong body vượt giới hạn (`name` của preset > 64, `item_id` > 64, `avatar_config` > 16 KB), THÌ module game PHẢI trả `422 VALIDATION_FAILED` | FR-GAME-004 · ADR-0041 | — |
| F070-AC-18 | NẾU client chưa có visual của item đã trang bị, THÌ game client PHẢI hiển thị item mặc định cùng slot và tải bundle nền, không crash | FR-GAME-004 · [CHARACTER_SYSTEM §5.2](../../docs/01-game-design/CHARACTER_SYSTEM.md#52-item-definitions--hiển-thị) | — |

---

### User Story 4 — Emote mặc định (Priority: P2)

Học sinh có sẵn 6 emote 3D miễn phí (vẫy tay, vỗ tay, nhảy, ăn mừng, cảm ơn, xin lỗi) trên vòng chọn emote và phát được trên nhân vật.

**Why this priority**: Emote là cách giao tiếp an toàn mặc định cho mọi lứa tuổi; phát cho người khác cần realtime (F091).

**Independent Test**: Profile mới có `emote_slots` = 6 emote mặc định; mở emote wheel, chọn emote → nhân vật phát clip một lần rồi về Locomotion.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F070-AC-19 | KHI avatar được tạo, module game PHẢI điền `emote_slots` bằng các emote mặc định của danh mục (`emote_wave`, `emote_clap`, `emote_dance`, `emote_celebrate`, `emote_thank`, `emote_apologize`) | FR-GAME-015 · [CHARACTER_SYSTEM §7](../../docs/01-game-design/CHARACTER_SYSTEM.md#7-emotes) | TC-GAM-011 |
| F070-AC-20 | KHI học sinh chọn emote trên emote wheel, game client PHẢI phát clip one-shot trên nhân vật qua `anim_driver.gd` rồi trở về Locomotion; chỉ emote đang trang bị hiện trên wheel | FR-GAME-004 · FR-GAME-015 | — |

---

### User Story 5 — Bảo vệ hồ sơ: phân quyền và cô lập tenant (Priority: P1)

Chỉ học sinh dùng được các API hồ sơ của mình; dữ liệu của tenant khác không bao giờ lộ.

**Why this priority**: Constitution điều III, V không có ngoại lệ.

**Independent Test**: Gọi mọi route F070 bằng token nhân sự → 403; `hs.dung.7a` (tenant khác) không ảnh hưởng tới gợi ý/độ duy nhất nickname của `hs.an.3a`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F070-AC-21 | HỆ THỐNG PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` cho `getGameProfile`, `updateGameProfile`, `suggestNicknames`, `updateAvatar`, `listAvatarPresets`, `saveAvatarPreset`, `getCharacterCatalog` khi token là nhân sự (`school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin`) | AGENTS R23 · ADR-0031 | TC-AUTH-014 |
| F070-AC-22 | HỆ THỐNG PHẢI đọc/ghi hồ sơ, preset, lịch sử nickname chỉ trong tenant của phiên (`TenantScope`); NẾU `X-Tenant-Id` khác tenant của học sinh, THÌ PHẢI trả `403 TENANT_MISMATCH` | NFR-SEC-006 · AGENTS R8 | TC-TEN-001, TC-TEN-011 |

### Edge Cases

- NẾU Valkey mất dữ liệu gợi ý nickname, THÌ `updateGameProfile` trả `409 NICKNAME_UNAVAILABLE` (F070-AC-09), client lấy gợi ý mới; không mất dữ liệu bền.
- NẾU item đã trang bị bị thu hồi qua bút toán bù (F075), THÌ lần `updateAvatar` kế tiếp bỏ item đó về mặc định; hiển thị hiện tại không lỗi (F070-AC-18).
- NẾU học sinh bị dọn/khôi phục (F016), THÌ profile soft-delete/khôi phục cùng dữ liệu game; nickname giữ chỗ theo partial unique `WHERE deleted_at IS NULL`.
- NẾU danh mục có version mới khi học sinh đang ở S02, THÌ client dùng bản đã tải; server kiểm theo version hiện hành và trả `422 VALIDATION_FAILED` nếu lựa chọn bị bỏ — client tải lại danh mục.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-004 | Customization skin/outfit/hair/accessory/emote, kiểm tra sở hữu và slot; hiển thị level do server tính (đọc, F074 cập nhật); inventory chỉ đọc để kiểm sở hữu (kho đồ đầy đủ F075) | [CHARACTER_SYSTEM §5–§7](../../docs/01-game-design/CHARACTER_SYSTEM.md) |
| FR-GAME-005 | Nickname từ danh sách sinh tự động, duy nhất trong tenant, giới hạn đổi; không lộ tên thật | [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname), [CHILD_SAFETY](../../docs/10-security/CHILD_SAFETY.md) |
| FR-GAME-015 | Luồng tạo avatar lần đầu, equipment slot, preset phối đồ, 6 emote mặc định | [CHARACTER_SYSTEM §5.0, §7](../../docs/01-game-design/CHARACTER_SYSTEM.md) |

### Key Entities

- **GameProfile**: hồ sơ game của học sinh trong tenant: nickname, avatar, level/XP (đọc), khu cuối, cài đặt game, thời điểm tạo avatar.
- **AvatarConfig**: cấu hình ngoại hình theo slot (dáng, màu da, tóc + màu, trang phục, phụ kiện, emote, pet, danh hiệu, khung, huy hiệu trưng bày).
- **CharacterCatalog**: nội dung platform có version: preset, dáng, bảng màu, item mặc định, bộ từ sinh nickname song ngữ.
- **ItemDefinition / InventoryItem**: định nghĩa vật phẩm có `equip_slot`; sở hữu của học sinh (chỉ đọc ở feature này).
- **AvatarPreset**: bộ phối đồ đã lưu theo ô.
- **NicknameChange**: lịch sử đặt/đổi nickname (đầu tiên, học sinh đổi, reset).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `avatar_creation_presets` / `saved_outfits_max` | Số preset tạo nhân vật; số ô phối đồ |
| `emote_wheel_slots` | Số emote tối đa trên wheel |
| `supported_locales` | Ngôn ngữ sinh nickname và tên hiển thị danh mục |
| `api_rate_default` | Rate limit `suggestNicknames`, `updateAvatar` |
| `nickname_change_monthly_limit` (Mới, [Confirmed]) | Số lần học sinh tự đổi nickname mỗi tháng |
| `nickname_suggestion_ttl` (Mới, [Confirmed]) | Thời gian `nickname_option_id` còn hiệu lực |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chỉ học sinh (middleware `game_player`); TenantScope + RLS | AGENTS R8, R23 · ADR-0031 |
| Không log nickname kèm `student_id` ở INFO; không log họ tên | AGENTS R14 |
| Avatar ghép trong ngân sách nhân vật; xem trước S02 theo PERFORMANCE_BUDGET | NFR-COMP-001 · ADR-0057 |
| UI theo Theme game sinh từ `design/game-tokens`, chuỗi UI qua `locale/*.po` | AGENTS R32 · NFR-I18N-001 |
| Không pay-to-win: cosmetic không ảnh hưởng học tập/thi đấu | AGENTS R19 · NFR-SAFE-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: ≥ 90% học sinh pilot hoàn tất tạo nhân vật trong ≤ 2 phút (`g1_3`: ≤ 3 bước, không gõ phím).
- **SC-002**: 0 nickname trùng trong cùng tenant; 0 nickname chứa tên thật (kiểm bằng test sinh 10.000 gợi ý).
- **SC-003**: 100% item không sở hữu/sai slot bị từ chối trong test API.
- **SC-004**: `avatar.created` phát đúng 1 lần cho mỗi học sinh trong test lặp và đồng thời.

## Ngoài phạm vi

- Phát emote/appearance cho người chơi khác (`hub.player.emote`, `hub.player.appearance_changed`, rate limit `emote_rate`) → F091/F092.
- Kho đồ đầy đủ, nhận/thu hồi item, Shop, collection, `listInventory` → F075; XP/level/danh hiệu → F074.
- Reset nickname bởi `school_admin`/`safety_moderator` → F121 (xử lý vi phạm) và F052 (màn trường); biên tập `CharacterCatalog` trên Game Operations Console → F078/F079.
- Thẻ hồ sơ người chơi khác, hiển thị trường/lớp theo người xem → F095; character controller/camera/HUD → F062.

## Assumptions

- F014 đã có `StartGameSession`, projection `ref.students`, `ref.tenants`; F070 thêm bước `EnsureGameProfile` vào transaction đó.
- Danh mục v1 (6 preset, 4 dáng, bảng màu, đồ mặc định, 6 emote, bộ từ nickname vi/en) do content team cung cấp qua seed [Confirmed]; asset 3D theo OQ-16 (đã trả lời 2026-11-20: stylized low-poly, asset pack có license thương mại + tự làm nhân vật chính).
- Gợi ý nickname giữ tạm trong Valkey (không phải dữ liệu quan trọng, ADR-0004); nickname đã chọn lưu PostgreSQL.
