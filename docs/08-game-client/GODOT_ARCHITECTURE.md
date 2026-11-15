# Godot Client Architecture

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: Senior Godot Developer · Cập nhật: 2026-11-15
> Liên quan: [ADR-0007, ADR-0011, ADR-0013, ADR-0014, ADR-0016](../adr/README.md) · [GAME_NETWORKING](GAME_NETWORKING.md) · [WORLD_RENDERING](WORLD_RENDERING.md) · [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) · [MINIGAME_ENGINE](MINIGAME_ENGINE.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md)

**Mục đích & phạm vi.** Điểm vào kiến trúc client Godot: cấu trúc dự án, autoload, vòng đời ứng dụng, tách UI/gameplay/service, renderer hoạt động học tập, nạp nội dung theo manifest, xử lý lỗi, build/export, kiểm thử. Mạng ở GAME_NETWORKING; thế giới 3D ở WORLD_RENDERING; mini-game ở MINIGAME_ENGINE.

## 1. Nguyên tắc

| # | Nguyên tắc | Nhãn |
|---|---|---|
| G-1 | Godot là **client hiển thị và xử lý tương tác**. Không quyết định điểm, kết quả trận, mastery, reward, quyền truy cập (Master Prompt §13.1, ADR-0016) | [Confirmed] |
| G-2 | Không hardcode nội dung học tập trong scene. Quest, mini-game, NPC dialogue, câu hỏi tải từ backend (data-driven) | [Confirmed] |
| G-3 | Godot 4.x (bản stable mới nhất tại thời điểm Phase 1, khóa phiên bản minor trong `project.godot` và CI), GDScript **static typing bắt buộc** | [Proposed] ADR-0014 |
| G-4 | Tách 3 lớp: **Service** (logic thuần, `RefCounted`, test được) ← **Gameplay** (Node 3D, controller) ← **UI** (Control). UI và gameplay không gọi HTTP trực tiếp | [Proposed] |
| G-5 | Scene không phụ thuộc chặt vào scene khác: giao tiếp qua signal, `EventBus`, hoặc dữ liệu inject khi khởi tạo | [Confirmed] (§21) |
| G-6 | Không giữ answer key, không tự tính XP/reward. Mọi số liệu nghiệp vụ hiển thị lấy từ response server | [Confirmed] |
| G-7 | Không hiển thị thông tin cá nhân nhạy cảm của người chơi khác: chỉ `nickname` + avatar (DOMAIN_MODEL §2) | [Confirmed] |

## 2. Kiến trúc tổng quan

```mermaid
flowchart TB
  subgraph UI[UI layer — Control scenes]
    HUD[World HUD] --- QP[Quest Panel] --- EX[Exercise UI] --- CH[Chat Panel] --- PL[Party Lobby]
  end
  subgraph Gameplay[Gameplay layer — Node3D]
    PC[PlayerController] --- RP[RemotePlayer] --- NPC[NPC] --- IO[Interactable] --- MG[MiniGame host]
  end
  subgraph Services[Service layer — Autoload + RefCounted]
    AS[AuthService] --> API[ApiClient]
    RTC[RealtimeClient]
    CC[ContentCache]
    QS[QuestService] --> API
    LS[LearningService] --> API
    PS[PartyService] --> RTC
    CS[ChatService] --> RTC
    ST[AppState store]
    EB[EventBus]
    SR[SceneRouter]
  end
  UI -- đọc state / gọi intent --> Services
  Gameplay -- intent / signal --> Services
  Services -- signal / EventBus --> UI
  Services -- signal / EventBus --> Gameplay
  API -- HTTPS --> GW[(APISIX /api/v1)]
  RTC -- WSS --> RT[(/realtime/v1)]
  CC -- HTTPS --> CDN[(CDN asset/PCK)]
```

Luồng dữ liệu một chiều: **UI/Gameplay phát intent → Service gọi backend → Service cập nhật `AppState` → phát signal → UI/Gameplay render lại.** Không có đường tắt UI → HTTP.

## 3. Cấu trúc thư mục `godot/`

```text
godot/
├── project.godot
├── export_presets.cfg              # không chứa keystore password (inject từ CI)
├── addons/
│   ├── gdUnit4/                    # test framework
│   └── eduworld_platform/          # plugin native (tùy chọn): certificate pinning, thông tin thiết bị; không lưu token (ADR-0064); không còn deep link/Custom Tabs cho đăng nhập (ADR-0068)
├── scenes/
│   ├── bootstrap/   Boot.tscn, Splash.tscn, VersionCheck.tscn
│   ├── auth/        Login.tscn, LoginWaiting.tscn, TenantChoice.tscn, AccountNotProvisioned.tscn
│   ├── world/       zones/{main_hub,learning_zone_math,...}/Zone.tscn, WorldRoot.tscn, AreaPortal.tscn
│   ├── character/   Player.tscn, RemotePlayer.tscn, AvatarRig.tscn, CustomizationStudio.tscn
│   ├── npc/         NPC.tscn, DialogueBubble.tscn
│   ├── quest/       QuestTracker.tscn, QuestJournal.tscn, QuestMarker3D.tscn
│   ├── learning/    LessonViewer.tscn, ExerciseHost.tscn, ActivityHost.tscn, questions/{SingleChoice,FillBlank,...}.tscn,
│   │                activity_renderers/{quiz,interactive_exercise,written_response,video_lesson,audio_lesson,speaking,simulation,project}/<Type>Renderer.tscn,
│   │                video/VideoLessonPlayer.tscn (VideoStreamPlayer Theora + SubtitleLayer + CueOverlay),
│   │                audio/AudioLessonPlayer.tscn, audio/SpeakingRecorder.tscn (AudioEffectRecord),
│   │                simulation/SimulationHost.tscn (nạp template PCK đã ký), math/MathInline.tscn
│   ├── practice/    PracticeBoard.tscn
│   ├── exam/        ExamLobby.tscn, ExamRunner.tscn, ExamResult.tscn
│   ├── competition/ MatchmakingPanel.tscn, ReadyCheck.tscn, BattleArena.tscn, Leaderboard.tscn
│   ├── minigames/   MiniGameHost.tscn, types/{matching,ordering,drag_drop,...}/
│   ├── chat/        ChatPanel.tscn, MessageList.tscn, MessageBubble.tscn, MessageActionMenu.tscn,
│   │                Composer.tscn, EmojiStickerGifPicker.tscn, MentionAutocomplete.tscn,
│   │                PinnedBar.tscn, TypingIndicator.tscn, ReceiptBadge.tscn, ChatSearch.tscn, QuickMessageWheel.tscn
│   ├── social/      FriendList.tscn, PartyLobby.tscn, InvitationToast.tscn
│   └── ui/          components/{Button,Card,Dialog,Toast,ProgressBar,...}.tscn, debug/UiGallery.tscn
├── scripts/
│   ├── core/            app_state.gd, event_bus.gd, scene_router.gd, config.gd, logger.gd, result.gd,
│   │                    ui_tokens.gd (class_name UiTokens — SINH TỪ design/game-tokens)
│   ├── networking/      api_client.gd, http_request_pool.gd, realtime_client.gd, ws_envelope.gd, backoff.gd
│   ├── authentication/  auth_service.gd, login_form.gd, memory_token_store.gd (chỉ RAM)
│   ├── player/          player_controller.gd, camera_rig.gd, avatar_builder.gd, anim_driver.gd
│   ├── world/           zone_loader.gd, spawn_point.gd, interactable.gd, area_portal.gd, remote_player_sync.gd
│   ├── learning/        learning_service.gd, question_renderer_registry.gd, activity_renderer_registry.gd,
│   │                    answer_models/*.gd, video_progress_reporter.gd, vtt_parser.gd, recording_uploader.gd,
│   │                    simulation_action_log.gd, math_svg_cache.gd
│   ├── multiplayer/     party_service.gd, presence_service.gd, matchmaking_service.gd, battle_client.gd
│   ├── competition/     leaderboard_service.gd
│   ├── chat/            chat_service.gd, chat_store.gd (seq/gap/unread), chat_asset_cache.gd (sticker/GIF)
│   ├── inventory/       inventory_service.gd
│   └── notifications/   notification_service.gd
├── resources/
│   ├── ui/theme/     eduworld_theme.tres + biến thể density g1_3/g4_5/g6_7/g8_9 — SINH TỪ design/game-tokens, không sửa tay
│   ├── art/generated/ preset ShaderMaterial toon/outline, Environment theo zone, Gradient VFX, màu độ hiếm — SINH TỪ design/game-tokens/art3d.json
│   ├── characters/   body_types/*.tres, CharacterCatalog.tres
│   ├── items/        ItemVisual.tres theo item_id (chỉ mô tả hiển thị, không giá trị nghiệp vụ)
│   ├── animations/   AnimationLibrary *.res
│   ├── materials/    toon/*.tres
│   ├── environments/ zone_env/*.tres (WorldEnvironment)
│   └── audio/        bus_layout.tres
├── assets/
│   ├── models/ textures/ audio/ fonts/ icons/
├── locale/           vi.po, en.po (gettext)
└── tests/            unit/, integration/, fixtures/ (JSON response mẫu)
```

Quy tắc đặt tên: file/thư mục `snake_case`, scene `PascalCase.tscn`, class_name `PascalCase`, signal `snake_case` ở thì quá khứ (`quest_progress_updated`), hằng `UPPER_SNAKE`.

## 4. Autoload singletons

Thứ tự khai báo trong `project.godot` (phụ thuộc từ trên xuống):

| # | Autoload | Loại | Trách nhiệm | Không làm |
|---|---|---|---|---|
| 1 | `Config` | Node | Đọc cấu hình môi trường từ `res://config/<env>.json` + override `user://` (debug build): `api_base_url`, `realtime_url`, `oidc_issuer`, `oidc_client_id`, `cdn_base_url`, feature flag mặc định; **cấu hình thiết bị** `device.cfg` (`shared_device` cho máy phòng tin học: không nhớ username, tự đăng xuất — xem §5.1) | Chứa secret (không có secret nào trong client) |
| 2 | `Log` | Node | Structured log, ring buffer 500 dòng gửi kèm crash report, lọc PII | Gửi log liên tục lên server |
| 3 | `EventBus` | Node | Signal toàn cục cho sự kiện cắt ngang (`session_expired`, `network_state_changed`, `notification_received`, `zone_changed`) | Truyền dữ liệu lớn, thay thế signal cục bộ |
| 4 | `AppState` | Node | Store trạng thái phiên: `player` (bootstrap), `school_policies`, `feature_flags`, `party`, `current_zone`, `connectivity` | Gọi mạng |
| 5 | `AuthService` | Node | Đăng nhập username/password bằng form trong game, gọi `POST /auth/game/login` của identity-service (ADR-0068; không trình duyệt, không PKCE/loopback), không chọn trường (ADR-0063); chọn tenant bằng `login_ticket` khi thuộc nhiều tenant; refresh qua `/auth/game/token:refresh`, logout qua `/auth/game/logout`; lưu `tenant_id` của phiên cho `ApiClient` gắn `X-Tenant-Id` (ADR-0065); mỗi lần mở game nhập mật khẩu, chỉ nhớ username (ADR-0064); giữ access/refresh token **chỉ trong RAM**; refresh, reconnect, logout + revoke khi thoát; sau `TENANT_NOT_SELECTED` xin token mới với scope `organization:{tenant_alias}` (SSO); cung cấp access token hợp lệ cho `ApiClient` | Lưu mật khẩu hoặc token xuống đĩa/`user://`/Keychain/Keystore, tự đăng nhập, dùng embedded WebView |
| 6 | `ApiClient` | Node | HTTP REST: pool `HTTPRequest`, header chuẩn (`Authorization`, **`X-Tenant-Id`** — tenant của phiên từ bootstrap, gửi ở mọi request trừ `client-config` và `/auth/game/login`, `tenant:select` (ADR-0065), `Idempotency-Key`, `X-Client-Version`, `X-Device-Id` — UUID sinh lần đầu cài, lưu `user://device_id`, ADR-0031, `traceparent`), parse Problem Details, retry/backoff | Logic nghiệp vụ |
| 7 | `RealtimeClient` | Node | WebSocket `/realtime/v1`, ticket, heartbeat, reconnect/resume, phân phối message theo `type` | Quyết định kết quả trận |
| 8 | `ContentCache` | Node | Cache nội dung data-driven (quest/minigame/dialogue/catalog) theo `version`, tải asset PCK từ CDN, kiểm tra checksum | Cache câu hỏi chấm điểm hay answer key (không tồn tại phía client) |
| 9 | `SceneRouter` | Node | Chuyển scene/zone, màn hình loading, `ResourceLoader.load_threaded_request`, giải phóng zone cũ | Biết logic quest |
| 10 | `AudioDirector` | Node | Nhạc nền/zone, SFX, volume theo settings | — |
| 11 | `Settings` | Node | Settings người dùng (`user://settings.cfg`): âm lượng, chất lượng đồ họa, ngôn ngữ, cỡ chữ, chế độ màu | Lưu dữ liệu nghiệp vụ |
| 12 | `QualityManager` | Node | Chọn renderer và preset Thấp/Trung bình/Cao, benchmark tự phát hiện lần đầu, độ phân giải động, trần FPS, hạ cấp theo nhiệt/pin, tạm dừng render 3D khi mở activity player, phát signal `quality_changed` cho zone/hiệu ứng; mọi giá trị theo [PERFORMANCE_BUDGET](PERFORMANCE_BUDGET.md) §4–§6, §12 | Tự đổi preset giữa trận/bài; lưu lên server |

Service theo domain (`QuestService`, `LearningService`, `PartyService`, `ChatService`, `InventoryService`, `MatchmakingService`...) là `RefCounted` tạo bởi `ServiceLocator` trong `AppState` sau bootstrap, nhận `ApiClient`/`RealtimeClient` qua constructor → test bằng fake.

## 5. Vòng đời ứng dụng

Học sinh đăng nhập **username + password**, **không chọn trường** (ADR-0063) và **mỗi lần mở game đều phải đăng nhập** — không có trạng thái "tự vào bằng refresh token" (ADR-0064). Backend xác định tenant sau khi xác thực; bước chọn tenant chỉ xuất hiện **sau** đăng nhập khi người dùng thuộc nhiều tenant.

```mermaid
stateDiagram-v2
  [*] --> Boot
  Boot --> VersionCheck: Config loaded
  VersionCheck --> ForceUpdate: client_version < min_supported
  VersionCheck --> Login: phiên bản hợp lệ (luôn đăng nhập, không dùng token cũ)
  Login --> Bootstrap: POST /auth/game/login thành công (form trong game, ADR-0068)
  Login --> Login: 401 INVALID_CREDENTIALS / 429 (hàng chờ, thử lại)
  Bootstrap --> TenantChoice: 403 TENANT_NOT_SELECTED + login_ticket (nhiều tenant)
  TenantChoice --> Bootstrap: POST /auth/game/tenant#58;select (không nhập lại mật khẩu)
  TenantChoice --> Login: 401 LOGIN_TICKET_INVALID (quá 2 phút)
  Bootstrap --> NotProvisioned: 403 ACCOUNT_NOT_PROVISIONED (không thuộc tenant chung lẫn tenant SIS)
  NotProvisioned --> Login: xóa token trong RAM (phiên Keycloak đã bị thu hồi)
  Bootstrap --> Login: 403 TENANT_MISMATCH (header X-Tenant-Id không còn khớp membership) → đăng nhập lại
  Bootstrap --> AccountBlocked: 403 ACCOUNT_LOCKED / NO_ACTIVE_ENROLLMENT / TENANT_ACCESS_DISABLED
  Bootstrap --> NotStudent: 403 GAME_ACCESS_STUDENT_ONLY (giáo viên/vận hành)
  NotStudent --> Login: đăng xuất, hướng dẫn dùng trang quản trị
  Bootstrap --> FirstTimeSetup: chưa có game_profile (chọn nickname/avatar)
  Bootstrap --> SessionStart
  FirstTimeSetup --> SessionStart
  SessionStart --> ConfirmTakeover: 409 ACTIVE_SESSION_ON_OTHER_DEVICE
  ConfirmTakeover --> SessionStart: học sinh đồng ý → takeover=true
  ConfirmTakeover --> Login: học sinh từ chối → đăng xuất
  SessionStart --> InWorld: game_session + realtime ticket + zone
  InWorld --> ReplacedNotice: session.session.replaced / close 4409
  ReplacedNotice --> Login: xóa token, về màn hình đăng nhập
  InWorld --> InWorld: chuyển zone / mini-game / exam
  InWorld --> Login: session_expired (refresh lỗi) / mobile ở nền quá 30 phút
  InWorld --> [*]: logout / thoát (POST /game/sessions/#123;id#125;#58;end)
```

| Bước | API |
|---|---|
| VersionCheck | `GET /api/v1/game/client-config?platform=&version=` (public, cache được, không `X-Tenant-Id`) — trả `min_supported_version`, `content_manifest_version`, maintenance flag, URL API đăng nhập |
| Login | Form Godot (S01) → `POST /api/v1/auth/game/login` `{username, password, device_id, client_version, locale}` ([KEYCLOAK_SSO §6](../09-identity/KEYCLOAK_SSO.md#6-luồng-đăng-nhập-game--form-trong-godot-qua-backend-adr-0068), ADR-0068); không trình duyệt, không PKCE; lỗi `401 INVALID_CREDENTIALS`, `426 CLIENT_VERSION_UNSUPPORTED`, `429 RATE_LIMITED`/`LOGIN_QUEUE_BUSY` |
| Bootstrap | Trả cùng response `/auth/game/login` (hoặc `/auth/game/tenant:select`): token + player bootstrap (`tenant_id`, `tenant_type`, `is_common_tenant`, profile, school, class, grade, policies, flags); `ApiClient` lưu `tenant_id` để gắn `X-Tenant-Id`. Lỗi `TENANT_NOT_SELECTED` kèm `choices[]` + `login_ticket`, `ACCOUNT_NOT_PROVISIONED` (phiên Keycloak đã bị backend thu hồi) |
| SessionStart | `POST /api/v1/game/sessions` (header `X-Device-Id`, `X-Tenant-Id`) → `game_session_id`; 409 `ACTIVE_SESSION_ON_OTHER_DEVICE` → `ConfirmTakeover`; `POST /api/v1/game/sessions/{id}/realtime-tickets` → ticket |
| Đăng xuất / thoát | Nút "Đăng xuất" luôn có trong menu; đóng cửa sổ (`NOTIFICATION_WM_CLOSE_REQUEST`) → `POST /game/sessions/{id}:end` (best effort) + `POST /auth/game/logout` (identity-service thu hồi refresh token + phiên Keycloak) + xóa token trong RAM — **mọi thiết bị** (ADR-0064); mobile: app vào nền quá 5 phút → kết thúc game session (best effort), quay lại trong 30 phút → reconnect bằng refresh token trong RAM, quá 30 phút hoặc bị hệ điều hành đóng → đăng nhập lại; máy phòng tin học (`device.cfg` có `shared_device: true`) không nhớ username, 30 phút không hoạt động → đăng xuất |
| Thế giới | `GET /api/v1/game/world/areas`, `GET /api/v1/game/profile`, `GET /api/v1/game/quests?status=active` |

### 5.1. Đăng nhập và xác định tenant (ADR-0063, ADR-0064, ADR-0065, ADR-0068)

Màn đăng nhập (UX: [STUDENT_GAME_UX_CORE S01](../05-ux/STUDENT_GAME_UX_CORE.md#s01--đăng-nhập)) là **một form vẽ bằng Godot**: logo, chọn ngôn ngữ, ô username, ô password (`LineEdit.secret`, nút hiện/ẩn), nút "Đăng nhập", ô "Nhớ tên đăng nhập trên máy này", "Quên mật khẩu? Nhờ thầy cô đặt lại". `AuthService` gửi tới `POST /auth/game/login` (ADR-0068); **không** mở trình duyệt hệ thống, Custom Tabs, `ASWebAuthenticationSession`, WebView; không TCPServer loopback, không PKCE, không `prompt=login`/`login_hint`; game không biết client secret hay endpoint Keycloak. **Không có bước chọn trường, không có ô mã trường, không có cách đăng nhập nhanh** trong Release 1.0 (ADR-0063; thiết kế mở rộng ở [KEYCLOAK_SSO §8.1](../09-identity/KEYCLOAK_SSO.md#81-mở-rộng-sau--ngoài-release-10-adr-0063)).

Trong cùng lời gọi đăng nhập, identity-service xác định tenant theo membership trong DB (ADR-0063 mục 3):

| Membership | Client |
|---|---|
| Đúng 1 tenant (tenant SIS hoặc tenant chung do platform tạo) | Bootstrap thành công, vào game (`is_common_tenant` cho biết tenant chung) |
| Nhiều tenant (hiếm) | `403 TENANT_NOT_SELECTED` kèm `choices[]` + `login_ticket` → `TenantChoice` **sau** đăng nhập → `AuthService.select_tenant(ticket, tenant_id)` gọi `POST /auth/game/tenant:select` (không nhập lại mật khẩu, ticket 2 phút) → Bootstrap |
| Không thuộc tenant chung và không thuộc tenant SIS nào | `403 ACCOUNT_NOT_PROVISIONED` → màn "chưa được kích hoạt", xóa token trong RAM; backend đã ghi audit và thu hồi phiên Keycloak |

Quy tắc token và phiên (ADR-0064):

- Mỗi lần khởi động game: luôn vào `Login` với ô mật khẩu trống → học sinh nhập mật khẩu; không có cookie/phiên nào được dùng lại.
- Access/refresh token **chỉ trong RAM** của `AuthService`; không ghi xuống `user://`, file, Keychain/Keystore, localStorage. Trong lúc game mở, refresh token dùng để gia hạn access token (5 phút) và reconnect, không hỏi mật khẩu.
- Mật khẩu chỉ nằm trong `LineEdit` và body request; xóa ô và biến ngay sau khi nhận phản hồi; không log, không telemetry, không crash report chứa body đăng nhập (ADR-0068). Certificate pinning cho tên miền API [Proposed].
- Chỉ được nhớ **username** (`user://settings.cfg` → `last_username`, tùy chọn, mặc định bật) để điền sẵn ô tên đăng nhập; máy `device.cfg` có `shared_device: true` thì ẩn tùy chọn và không lưu username.
- Thoát game bình thường → `:end` game session + `POST /auth/game/logout`; tắt đột ngột → phiên Keycloak tự hết sau idle 30 phút.
- **Header `X-Tenant-Id` bắt buộc** (ADR-0065): sau bootstrap, `ApiClient` gắn `X-Tenant-Id = bootstrap.tenant_id` vào mọi request REST (trừ `client-config`, `/auth/game/login`, `/auth/game/tenant:select`); backend kiểm tra header khớp membership và game session. Tenant chỉ giữ trong RAM của phiên; không còn `user://tenant.cfg`, không gợi ý tenant trước đăng nhập. Không đổi tenant giữa phiên (muốn đổi → đăng xuất, đăng nhập lại). Nhận `400 TENANT_HEADER_REQUIRED` là lỗi lập trình (báo telemetry, về `Login`).

## 6. Phân tách UI / Gameplay / Service

Ví dụ "học sinh nói chuyện với NPC để nhận quest":

1. `Interactable` (gameplay) phát `interacted(npc_id)`.
2. `NPC` mở `DialogueRunner` với `NPCDialogue` lấy từ `ContentCache` (đã version).
3. Node action `give_quest` → gọi `QuestService.accept(quest_id)` (intent).
4. `QuestService` gọi `POST /api/v1/game/quests/{quest_id}:accept`, nhận `QuestProgress` từ server, cập nhật `AppState.quests`, phát `quest_progress_updated`.
5. `QuestTracker` (UI) render lại. NPC không biết UI, UI không biết NPC.

Quy tắc review code:
- Script trong `scenes/ui/**` chỉ được gọi method public của service và đọc `AppState`.
- Script gameplay không `preload` scene UI; mở UI qua `EventBus.ui_requested(screen_id, args)` hoặc `SceneRouter`.
- Logic có nhánh nghiệp vụ (điều kiện quest, chọn câu hỏi, tính thời gian còn lại) nằm ở server hoặc service thuần, không trong `_process` của scene.

### 6.1. Chat UI component (ADR-0034, ADR-0035)

| Thành phần | Trách nhiệm | Ghi chú |
|---|---|---|
| `ChatService` (autoload service) | Gửi lệnh WS (`chat.message.send/edit/recall/forward`, `chat.reaction.toggle`, `chat.pin.add/remove`, `chat.typing.start/stop`, `chat.receipt.delivered/read`, `chat.sync`), REST fallback, xử lý `system.command.failed` theo mã lỗi | Không chứa logic kiểm duyệt; mọi quyết định ở server |
| `ChatStore` (`RefCounted`, test được) | Danh sách tin theo `channel_id` sắp theo `seq`; trạng thái tin của mình: *pending* (đồng hồ) → *sent* (`chat.message.acked`) → *delivered/read* (`chat.receipt.updated`); **phát hiện seq gap** (seq nhận > last_seq + 1) → gọi `chat.sync`, nếu > 200 tin → REST lịch sử; unread + mention count | Gap do tin bị lỗi gửi được đánh dấu "đã kiểm tra" sau một lần sync để không lặp vô hạn |
| `MessageList` / `MessageBubble` | Render text, emoji, sticker, GIF (AnimatedTexture từ cache), quick message, khung reply/quote (bản đã che), nhãn "Đã chuyển tiếp", "đã chỉnh sửa", placeholder thu hồi, reaction | Virtualized list (chỉ instance bubble trong viewport) để giữ FPS |
| `MessageActionMenu` | Long-press (mobile) / chuột phải hoặc nút ⋯ (desktop): Trả lời, Trích dẫn, Chuyển tiếp, Sao chép, Ghim, Sửa, Thu hồi, Thả cảm xúc, Báo cáo — hiển thị theo `Channel.features` và quyền | Sao chép dùng `DisplayServer.clipboard_set(text_hien_thi)` (bản đã che) |
| `Composer` | Ô nhập + nút emoji/sticker/GIF + quick message; gửi `chat.typing.start` khi gõ (throttle 3 s), `stop` khi gửi/xóa; giữ `client_msg_id` để gửi lại khi mất mạng | Giới hạn độ dài theo policy |
| `EmojiStickerGifPicker` | 3 tab; sticker theo pack (`GET /chat/stickers`, pack khóa có ổ khóa); GIF theo tag (`GET /chat/gifs?tag=`) | Asset tải từ CDN EduWorld, cache theo version |
| `MentionAutocomplete` | Gõ `@` → gợi ý nickname thành viên channel; trả `mentions[]` (user_id) | Không hiển thị username đăng nhập |
| `PinnedBar`, `TypingIndicator`, `ReceiptBadge`, `ChatSearch` | Thanh ghim (≤ 3), "An đang soạn tin…", ✓/✓✓/✓✓ xanh, tìm kiếm (`GET /chat/search`) | Ẩn theo `features` của channel (World/Area không có typing/receipts/pin) |

- Panel chat trong trận/party luôn có (ADR-0020 mục 6): chỉ thu gọn, không tắt.
- Badge unread trên HUD lấy từ `ChatStore`, đồng bộ đa màn hình bằng `chat.unread.updated`.
- Client **không** kết nối NATS/Valkey; chỉ WebSocket `/realtime/v1` và REST.

### 6.2. Activity renderers (ADR-0048)

`ActivityRendererRegistry` (autoload) ánh xạ `activity_type` → scene renderer trong `scenes/learning/activity_renderers/`. `ActivityHost` nhận render payload từ `LearningService`, chọn renderer, chuyển phản hồi/sự kiện tiến độ về service; renderer **không** chấm điểm, **không** giữ đáp án ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)).

| Renderer | Điểm chính |
|---|---|
| `quiz` / `interactive_exercise` | Renderer theo 11 dạng câu hỏi (`scenes/learning/questions/`, chọn qua `question_renderer_registry.gd`), gửi câu trả lời thô, hiển thị phản hồi server ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)) |
| `minigame` | Chọn scene theo `minigame_type` qua `MiniGameRegistry` ([MINIGAME_ENGINE](MINIGAME_ENGINE.md)) |
| `video_lesson` | `VideoStreamPlayer` (Ogg Theora), phụ đề WebVTT tự parse và vẽ theo Theme, dừng tại cue để hiện câu hỏi chèn, heartbeat tiến độ 10 s ([VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md)) |
| `audio_lesson` / `speaking` | `AudioStreamPlayer` (Ogg Vorbis); ghi âm bằng bus có `AudioEffectRecord`, chỉ khi học sinh bấm, hiển thị biểu tượng đang ghi, upload presigned ([AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md)) |
| `written_response` | Panel nhập văn bản lớn, lưu nháp 15 s, nhập đáp số (bàn phím số); **không có ô nhập công thức** ([WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md)) |
| `simulation` | Nạp template PCK đã ký, áp tham số + seed từ server, ghi action log, gửi server kiểm chứng ([SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md)) |
| `project` | Bảng bước, nộp văn bản/ghi âm/ảnh theo cấu hình |

Loại chưa có renderer trên client cũ → màn "Cần cập nhật game", gửi telemetry, không crash.

### 6.3. Hiển thị công thức Toán, Lý, Hóa (ADR-0055)

- Game **chỉ hiển thị** công thức, **không nhập** (không có bàn phím toán).
- Render payload chứa khối `math` với `math_ref` (`math/<sha256>.svg`, SVG đã render bằng MathJax 4 trên portal và sanitize ở server) và `speech_text`.
- `MathInline` tải SVG qua CDN, cache bất biến `user://cache/math/<sha256>.svg` (`math_svg_cache.gd`), tạo texture bằng `Image.load_svg_from_string()` với **scale theo DPI màn hình và cỡ chữ** (density theo grade band), tô màu theo Theme game, chèn vào `RichTextLabel` bằng `add_image()` (inline) hoặc khối riêng (block). Texture được cache theo `(sha256, scale, màu)`.
- Trợ năng: `speech_text` dùng cho TTS và mô tả thay thế; giữ `latex` làm fallback hiển thị chữ khi SVG lỗi.
- Lựa chọn đáp án, thẻ kéo thả, cặp ghép có thể chứa công thức (đều là SVG đã render).


## 7. Signal conventions

| Quy ước | Ví dụ |
|---|---|
| Signal là sự kiện đã xảy ra, thì quá khứ | `answer_submitted(attempt_id, item_id)`, `zone_loaded(area_id)` |
| Typed signal arguments | `signal party_updated(party: PartyModel)` |
| Kết nối trong code bằng `Callable` (`sig.connect(_on_x)`), không kết nối qua editor cho signal xuyên scene | |
| Child → parent: signal; parent → child: gọi method | |
| Signal toàn cục chỉ qua `EventBus`, có danh sách cố định trong `event_bus.gd` (review khi thêm) | `session_expired`, `network_state_changed(state)` |
| Ngắt kết nối khi node rời cây (`CONNECT_ONE_SHOT` hoặc connect với node owner để Godot tự dọn) | |

## 8. Data-driven loading & Content Cache

### 8.1. Loại dữ liệu tải từ backend

| Dữ liệu | Nguồn | Schema | Cache |
|---|---|---|---|
| Content manifest | `GET /api/v1/game/content-manifest` | OpenAPI `ContentManifest` | ETag; tải khi khởi động và khi vào màn mới (§8.2) |
| Level session (version ghim) | `POST /api/v1/game/levels/{level_id}:start` | OpenAPI `LevelSession` | Chỉ trong bộ nhớ trong màn |
| Quest definitions (version) | snapshot CDN trong manifest hoặc `GET /api/v1/game/quests/{quest_id}/versions/{version}` | [quest.schema.json](../../specs/schemas/quest.schema.json) | Theo `(item_id, version)` — bất biến (ADR-0015/0043), cache vô thời hạn |
| Mini-game config | `GET /api/v1/game/minigames/{config_id}/versions/{version}` | [minigame-config.schema.json](../../specs/schemas/minigame-config.schema.json) | Bất biến theo version |
| NPC dialogue | `GET /api/v1/game/world/npcs/{npc_id}/dialogue` | [npc-dialogue.schema.json](../../specs/schemas/npc-dialogue.schema.json) | Bất biến theo version |
| Item catalog, area list | `GET /api/v1/game/world/manifest` (gộp trong content manifest) | — | ETag + `manifest_version` |
| Media (ảnh, âm thanh, video .ogv) | CDN `media/<sha256>/<variant>.<ext>` (signed URL nếu private) | — | `user://cache/media/<sha256>/` bất biến (ADR-0044) |
| Câu hỏi (render payload) | `/api/v1/learning/activity-attempts/...` | OpenAPI | **Không cache bền**; chỉ giữ trong bộ nhớ trong attempt |
| Asset bundles (PCK) | CDN, URL trong manifest | manifest | `user://cache/pck/<sha256>.pck` |

### 8.2. Content manifest & versioning (ADR-0043)

Client tải **content manifest** của tenant `GET /api/v1/game/content-manifest` (header `If-None-Match: <etag>` → `304` nếu không đổi):

```json
{
  "manifest_version": 1287,
  "tenant_id": "01J...",
  "generated_at": "2026-11-15T08:00:00Z",
  "min_client_version": "0.4.0",
  "items": [
    { "item_type": "quest", "item_id": "01J...", "version_id": "01J...", "version_no": 5,
      "snapshot_url": "https://cdn.../content/quest/01J.../5/9f2c….json", "sha256": "9f2c…" },
    { "item_type": "minigame_config", "item_id": "01J...", "version_id": "01J...", "version_no": 3,
      "snapshot_url": "https://cdn.../content/minigame_config/01J.../3/1ab4….json", "sha256": "1ab4…" }
  ],
  "bundles": [
    { "bundle_id": "zone.learning_zone_math", "version": 7, "url": "https://cdn.../bundles/zone.learning_zone_math/7/<sha256>.pck",
      "sha256": "…", "size_bytes": 18452211, "platforms": ["windows","android","ios","web"], "requires_client": ">=0.4.0" }
  ],
  "areas": [ { "area_id": "learning_zone.math", "scene_ref": "res://zones/learning_zone_math/Zone.tscn", "bundle_id": "zone.learning_zone_math" } ]
}
```

**Khi nào tải manifest:** lúc khởi động (sau bootstrap) và **mỗi khi vào màn/level mới**. Không tải lại giữa màn. Sự kiện realtime `content.manifest.updated` chỉ đặt cờ "có bản mới"; bản mới được áp dụng ở màn kế tiếp.

**Vào màn mới — ghim version:**

```mermaid
sequenceDiagram
  participant G as Godot (SceneRouter + ContentCache)
  participant API as game-service
  participant CDN
  G->>API: GET /game/content-manifest (If-None-Match)
  API-->>G: 200 manifest mới | 304
  G->>API: POST /game/levels/{level_id}:start
  API-->>G: level_session_id + content_versions (item → version ghim) + danh sách media/bundle cần
  G->>CDN: tải snapshot/bundle/media còn thiếu theo (item_id, version) / sha256
  G->>G: kiểm tra sha256, preload media của màn, vào màn
  Note over G: Trong màn chỉ dùng content_versions đã ghim
  G->>API: POST /game/levels/{level_session_id}:end (hoặc timeout)
```

- **ContentCache** lưu theo khóa `(item_type, item_id, version_no)` tại `user://cache/content/<item_type>/<item_id>/<version_no>.json` (bất biến, kiểm tra `sha256`), bundle tại `user://cache/pck/<sha256>.pck`, media tại `user://cache/media/<sha256>/<variant>` (ADR-0044). Không bao giờ lẫn version vì khóa có version/hash.
- Học sinh đang ở màn với version cũ tiếp tục thấy nội dung cũ đến khi rời màn, kể cả khi CMS publish bản mới hoặc rollback.
- **Preload màn:** trước khi vào màn, tải đủ snapshot + bundle + media trong danh sách `:start` trả về (ưu tiên biến thể ảnh 1024 px trên mobile, 2048 px trên PC); hiển thị màn hình chờ có tiến trình. Thiếu mạng giữa chừng → thử lại có backoff; không vào màn khi thiếu tài nguyên bắt buộc.
- **Nội dung không cache bền:** câu hỏi đề kiểm tra, render payload của attempt, mọi dữ liệu tenant nhạy cảm — chỉ giữ trong bộ nhớ trong attempt. Answer key không bao giờ xuống client.
- Bundle chỉ chứa **asset và scene** (mesh, texture, audio, scene zone). **Không chứa script thực thi logic mới** [Proposed] — giảm rủi ro an ninh và tương thích; logic mới đi theo bản build client.
- LRU dọn cache khi vượt ngân sách lưu trữ (Android mặc định 1.5 GB [Proposed]); không xóa tài nguyên của màn đang chơi.
- Không cache dữ liệu tenant (danh sách lớp, kết quả bài) xuống đĩa trừ settings và bản sao profile tối thiểu để hiển thị offline screen.

## 9. Error handling & offline

| Tình huống | Hành vi client |
|---|---|
| Mất mạng ngắn (< 30s) | Banner "Đang kết nối lại…", RealtimeClient reconnect, chặn thao tác ghi; avatar đứng yên |
| Mất mạng dài | Màn hình chờ kết nối; không cho làm bài offline [Proposed] (server-authoritative) |
| Đang làm bài khi mất mạng | Câu trả lời đã gửi được server lưu; câu chưa gửi giữ trong hàng đợi bộ nhớ, gửi lại cùng `Idempotency-Key` khi có mạng; attempt resume theo policy (§4.4 Master Prompt) qua `GET /api/v1/learning/activity-attempts/{id}` |
| 401 | `AuthService.refresh()` (refresh token trong RAM) một lần, lặp lại request; thất bại → `EventBus.session_expired` → Login (nhập mật khẩu) |
| 403 `ACCOUNT_NOT_PROVISIONED` | Không thuộc tenant chung và không thuộc tenant SIS nào (ADR-0063): "Tài khoản của em chưa được kích hoạt trong EduWorld. Em hãy nhờ thầy cô kiểm tra nhé."; xóa token trong RAM (backend đã thu hồi phiên Keycloak), về `Login` |
| 403 `ACCOUNT_LOCKED` | "Tài khoản đang tạm khóa. Em hãy liên hệ thầy cô." |
| 403 `ACCOUNT_DEACTIVATED` | "Tài khoản không còn hoạt động." |
| 403 `NO_ACTIVE_ENROLLMENT` | "Em chưa được xếp lớp trong năm học này." (không áp dụng tenant chung) |
| 403 `TENANT_ACCESS_DISABLED` | "Trường của em hiện chưa mở EduWorld." |
| 403 `ACCESS_OUTSIDE_ALLOWED_HOURS` | "Bây giờ chưa đến giờ chơi EduWorld." |
| 401 `INVALID_CREDENTIALS` | "Tên đăng nhập hoặc mật khẩu chưa đúng" (+ gợi ý đợi vài phút từ lần sai thứ 3); xóa ô mật khẩu (ADR-0068) |
| 401 `LOGIN_TICKET_INVALID` | "Phiên chọn trường đã hết, em đăng nhập lại nhé" → `Login` |
| 426 `CLIENT_VERSION_UNSUPPORTED` | Màn `ForceUpdate` với `store_url` |
| 429 `LOGIN_QUEUE_BUSY` | "Đang vào lớp…", tự thử lại theo `Retry-After` + jitter |
| 400 `TENANT_HEADER_REQUIRED` | Lỗi lập trình (thiếu header): gửi telemetry `client_bug`, xóa token, về `Login` "Em hãy đăng nhập lại" |
| 403 `TENANT_NOT_SELECTED` | Hiển thị `TenantChoice` từ `choices[]` trong response; chọn xong gọi `/auth/game/tenant:select` với `login_ticket` (không nhập lại mật khẩu) |
| 403 `TENANT_MISMATCH` | Hiếm (tenant của phiên vừa bị gỡ membership, hoặc header sai): xóa token, về `Login` với thông điệp chung "Em hãy đăng nhập lại"; không tiết lộ tài khoản thuộc trường nào |
| 403 `GAME_ACCESS_STUDENT_ONLY` | Màn hình "Tài khoản này dùng cho giáo viên/cán bộ, không dùng để vào game. Vui lòng dùng trang quản trị EduWorld."; đăng xuất, xóa token |
| 409 `ACTIVE_SESSION_ON_OTHER_DEVICE` | Hộp thoại xác nhận (scene `ui/dialogs/TakeoverConfirm.tscn`): "Tài khoản của em đang được dùng trên {device_label} (hoạt động lúc {last_seen_at}). Nếu tiếp tục ở đây, thiết bị kia sẽ bị đăng xuất." — nút **Tiếp tục ở thiết bị này** / **Hủy**. Lớp 1–5: thêm biểu tượng và câu ngắn. Đồng ý → gọi lại với `takeover: true` (cùng `Idempotency-Key` mới) |
| `session.session.replaced` / close 4409 | Dừng gameplay, hộp thoại không tự đóng: "Em đã được đăng xuất vì tài khoản vừa đăng nhập trên thiết bị khác. Nếu không phải em, hãy báo thầy cô." → xóa token, không tự reconnect, về `Login` |
| 409 `ATTEMPT_ALREADY_SUBMITTED` | Coi như thành công, tải kết quả |
| 429 | Tôn trọng `Retry-After`; UI làm mờ nút |
| 5xx | Retry theo [GAME_NETWORKING](GAME_NETWORKING.md) §2.3; thông báo chung không lộ chi tiết |
| `maintenance` trong client-config | Màn hình bảo trì, kiểm tra lại mỗi 60s |

Thông điệp lỗi cho lớp 1–5 dùng ngôn ngữ đơn giản + biểu tượng; mã lỗi `code` chỉ hiện trong màn hình "Chi tiết" cho giáo viên hỗ trợ.

## 10. Settings, i18n & accessibility

- Game **song ngữ** `vi-VN` (mặc định) + `en-US` [Confirmed — ADR-0056]; chi tiết §10.1.
- Nội dung học tập và nội dung game (câu hỏi, hội thoại NPC, quest) đã được bản địa hóa phía backend; client chỉ chọn bản theo ngôn ngữ, không dịch.
- **Theme và token (ADR-0047, [DESIGN_SYSTEM §5](../05-ux/DESIGN_SYSTEM.md#5-game-design-system-godot-3d)):** game dùng **game design system** riêng, không dùng UI admin. `Theme` (`resources/ui/theme/eduworld_theme.tres`), lớp hằng `UiTokens` và resource 3D (`resources/art/generated/`) được **sinh** từ `design/game-tokens/` bằng `make design-tokens`; không sửa tay. Mọi `Control` lấy style từ Theme; không override màu/font/cỡ chữ trong scene (lint `tools/godot/lint_theme_overrides.gd` chặn trong CI, trừ allowlist). Kiểm tra trực quan bằng scene `UiGallery.tscn` (build debug).
- UI có hai lớp dùng chung Theme: **overlay 2D** (`CanvasLayer`: HUD, panel nhiệm vụ, câu hỏi, chat) và **UI trong không gian 3D** (`SubViewport` → `ViewportTexture` trên mesh/`Sprite3D`; `Label3D` cho nickname) cho bảng tên NPC, bảng câu hỏi gắn trong màn, biển chỉ dẫn.
- Font: font tiêu đề tròn thân thiện + font nội dung dễ đọc, hỗ trợ đầy đủ tiếng Việt có dấu, license OFL (vd. Baloo 2 + Be Vietnam Pro) [Proposed, Game Director chọn], fallback font trong `Theme`.
- Theme theo grade band (density token): `g1_3` chữ cơ sở 24px, nút ≥ 64px; `g4_5` 22px/56px; `g6_7` 18px/48px; `g8_9` 18px/44px; icon kèm chữ ở lớp nhỏ. Biến thể chọn từ `AppState.player.grade_band`.
- Tùy chọn: cỡ chữ (3 mức), chế độ tương phản cao, giảm chuyển động (tắt camera shake/particle), phụ đề cho mọi audio có lời, âm lượng riêng nhạc/SFX/giọng.
- Nhắc nghỉ giải lao theo policy trường (OQ-17) do server gửi qua notification; client chỉ hiển thị.

### 10.1. Quốc tế hóa i18n

Theo [ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md). Quy tắc ngôn ngữ của dữ liệu nội dung: [CONTENT_MODEL §8](../02-learning/CONTENT_MODEL.md#8-ngôn-ngữ-nội-dung-và-bản-dịch-adr-0056).

**Hai nguồn chuỗi**

| Nguồn | Ví dụ | Cơ chế |
|---|---|---|
| **Chuỗi UI tĩnh** (đóng gói trong build) | Nút, menu, HUD, thông điệp lỗi, tutorial cố định, tên màn hình | Gettext: `godot/locale/messages.pot` (sinh tự động) → `vi.po`, `en.po`; nạp bằng `TranslationServer`; trong code dùng `tr("ui.hud.quest_tracker.title")`, trong scene đặt `text` = khóa dịch (auto-translate của `Control` bật) |
| **Chuỗi dữ liệu** (từ backend, `localized_text`/`i18nText`) | Tên vùng, NPC, quest, item, đề bài | `ContentCache` giữ cả `vi` và `en` của snapshot game; helper `L10n.pick(text_dict)` chọn theo locale hiện tại, fallback theo CONTENT_MODEL §8 |

**Quy ước khóa dịch:** `ui.<screen>.<element>[.<state>]` (ví dụ `ui.login.school_code.placeholder`, `ui.quest_journal.empty`, `ui.error.ACCOUNT_LOCKED`). Mã lỗi API ánh xạ khóa `ui.error.<CODE>`; thiếu khóa thì dùng `ui.error.generic`. Khóa bất biến; sửa câu = sửa bản dịch, không đổi khóa.

**Số nhiều và tham số:** dùng `tr_n()` với `Plural-Forms` trong `.po` (tiếng Việt 1 dạng, tiếng Anh 2 dạng); tham số theo tên `{count}`, `{name}` thay bằng `String.format()`. Không ghép câu từ nhiều mảnh.

**Định dạng theo locale** (lớp `LocaleFormat`, không dùng định dạng của OS):

| Kiểu | `vi-VN` | `en-US` |
|---|---|---|
| Số | `1.234,5` | `1,234.5` |
| Ngày | `15/11/2026` | `Nov 15, 2026` |
| Giờ | `14:30` | `2:30 PM` |
| Thời lượng | `3 phút 20 giây` | `3 min 20 s` |
| Số trong đề toán | Theo `content_language` của câu hỏi; ô nhập chấp nhận `,` và `.` | Như cột trái |

**Đổi ngôn ngữ lúc chạy:** Cài đặt → Ngôn ngữ (mặc định theo tenant, lưu vào hồ sơ qua `PUT /game/profile` field `preferred_language`). Đổi ngay không khởi động lại: `TranslationServer.set_locale()` → `EventBus.locale_changed` → các màn đang mở tự cập nhật; NPC đang nói và câu hỏi đang hiển thị giữ ngôn ngữ cũ đến khi chuyển câu/hội thoại tiếp theo (tránh nhảy chữ giữa chừng). Tùy chọn "Hiện tên song ngữ" theo [WORLD_DESIGN §2.4](../01-game-design/WORLD_DESIGN.md#24-tên-song-ngữ-và-quy-tắc-hiển-thị-confirmed--adr-0056).

**Font và bố cục:** Theme có font chính hỗ trợ đầy đủ tiếng Việt có dấu và Latin mở rộng, fallback font cho ký hiệu; không dùng font chỉ có Latin. Mọi `Label`/`Button` thiết kế cho chuỗi dài hơn **35%** so với bản ngắn nhất (autowrap, `text_overrun_behavior = ellipsis` kèm tooltip, không cố định chiều rộng theo chữ). Chữ trong texture 3D (biển hiệu) dùng `Label3D`/`SubViewport` để dịch được, không vẽ chữ vào ảnh.

**Kiểm tra trong CI:**
- `tools/godot/extract_pot.gd` sinh `.pot`; thiếu khóa trong `vi.po` hoặc `en.po` → lỗi build.
- **Pseudo-localization**: build debug có locale `xx-PS` (chuỗi kéo dài 40% và có dấu `[!! Ẩnh Ẹxẗ !!]`) để phát hiện chuỗi hardcode và tràn chữ; chạy chụp ảnh các màn chính ở `vi`, `en`, `xx-PS` (visual regression).
- Lint scene: `Label.text` không phải khóa dịch (trừ allowlist số/ký hiệu) → cảnh báo.
- Telemetry `i18n.missing_key` ở runtime (không hiển thị khóa thô cho học sinh).

## 11. Build & export pipeline

| Nền tảng | Trạng thái | Template | Ghi chú |
|---|---|---|---|
| Windows 10+ x86_64 | Release 1.0, phát triển từ Phase 1 [Proposed] ADR-0013 | Mobile renderer mặc định, tự chuyển Compatibility; Forward+ chỉ preset Cao ([PERFORMANCE_BUDGET](PERFORMANCE_BUDGET.md) §4) | Ký code (Authenticode) ở Phase 6; installer + auto-update qua manifest |
| Android 9+ arm64-v8a | Release 1.0, phát triển từ Phase 1 [Proposed] | Mobile renderer (Vulkan), dự phòng Compatibility; ETC2/ASTC | Keystore inject từ CI secret; intent filter cho `vn.eduworld.game:/oauth2/callback`; AAB cho store, APK cho MDM trường |
| Web | Release 1.0; build CI từ Phase 2, hoàn thiện Phase 6 [Proposed] OQ-04 | Compatibility renderer, single-threaded export, preset tối đa Trung bình | Kích thước tải đầu và bộ nhớ theo [PERFORMANCE_BUDGET](PERFORMANCE_BUDGET.md) §11, §14; zone tải lười; redirect OIDC cùng tab; hosting qua CDN |
| iOS 15+ arm64 | Release 1.0; pipeline từ Phase 4, hoàn thiện Phase 6 [Proposed] OQ-04 | Mobile renderer, ASTC | Đăng nhập bằng form trong game (ADR-0068, không `ASWebAuthenticationSession`); không lưu token vào Keychain (ADR-0064); ký + TestFlight từ CI (runner macOS); phát hành App Store |

Pipeline CI (GitLab CI [Proposed], job path-based `godot/**`):

```mermaid
flowchart LR
  T[make design-tokens-check\nTheme/UiTokens/art generated khớp nguồn] --> A
  A["lint: gdlint + typed check + lint_theme_overrides"] --> B[GdUnit4 headless]
  B --> C[import assets --headless]
  C --> D[export Windows / Android]
  D --> E[build PCK zone bundles + manifest + sha256]
  E --> F[upload artifact / CDN staging]
  F --> G["smoke test: boot → login mock → hub"]
```

- Chạy Godot headless trong container có phiên bản Godot + export templates khóa cứng.
- `client_version` = SemVer, nhúng vào `Config` lúc build; build number = pipeline id.
- `export_presets.cfg` không chứa mật khẩu; CI ghi đè qua biến môi trường.

## 12. Testing

| Loại | Công cụ | Phạm vi |
|---|---|---|
| Unit | GdUnit4 | `login_form.gd` (xóa mật khẩu sau gửi, map lỗi), `backoff.gd`, `ws_envelope.gd`, answer models, `QuestService` với `FakeApiClient`, interpolation buffer |
| Scene test | GdUnit4 scene runner | Question renderer nhận render payload → tạo response đúng định dạng; mini-game lifecycle |
| Contract | Fixture JSON sinh từ `specs/` | Parse mọi response mẫu OpenAPI/AsyncAPI không lỗi; validate quest/minigame example theo JSON Schema trong CI |
| Integration | Headless client + backend docker compose | Login qua `/auth/game/login` (Keycloak test realm, user test) → bootstrap → mọi request có `X-Tenant-Id` → vào hub → làm 1 quest; kiểm tra log client không chứa mật khẩu |
| Performance | Kiểm tra import asset + cảnh benchmark headless trong CI (frame time CPU, draw call, tam giác qua `RenderingServer`); thiết bị lab thật mỗi milestone | Không vượt ngân sách [PERFORMANCE_BUDGET](PERFORMANCE_BUDGET.md) §7–§8 quá 5%; AC-PERF-001…011 |
| Theme / UI | `UiGallery.tscn` chụp ảnh từng density + lint override | Không màu/font hardcode trong `.tscn`; ảnh UI Gallery so với baseline (DESIGN_SYSTEM §9) |
| Renderer parity | Chụp ảnh zone/nhân vật mẫu trên Mobile, Forward+, Compatibility | Sai khác vượt ngưỡng SSIM → cảnh báo cho Art/Game Director [Proposed] |

Mục tiêu: service layer coverage ≥ 70% [Proposed]; mọi bug liên quan dữ liệu nghiệp vụ phải có test tái hiện.

## 13. Mã mẫu

### 13.1. `ApiClient` (rút gọn)

```gdscript
# scripts/networking/api_client.gd
class_name ApiClientImpl
extends Node

signal request_failed(problem: Dictionary)

const MAX_RETRIES := 3
var _base_url: String
var _auth: AuthServiceImpl

func setup(base_url: String, auth: AuthServiceImpl) -> void:
	_base_url = base_url
	_auth = auth

## Trả về Dictionary { ok: bool, status: int, data: Variant, problem: Dictionary }
func request_json(method: HTTPClient.Method, path: String, body: Variant = null,
		idempotency_key: String = "") -> Dictionary:
	var attempt := 0
	var refreshed := false
	while true:
		var token: String = await _auth.get_access_token()
		var headers := PackedStringArray([
			"Authorization: Bearer %s" % token,
			"Content-Type: application/json",
			"Accept: application/json, application/problem+json",
			"X-Client-Version: %s" % Config.client_version,
			"X-Device-Id: %s" % Device.id,
		])
		if _auth.tenant_id != "":                       # ADR-0065: bắt buộc sau bootstrap
			headers.append("X-Tenant-Id: %s" % _auth.tenant_id)
		if idempotency_key != "":
			headers.append("Idempotency-Key: %s" % idempotency_key)
		var http := HTTPRequest.new()
		http.timeout = 15.0
		add_child(http)
		var payload := "" if body == null else JSON.stringify(body)
		http.request(_base_url + path, headers, method, payload)
		var res: Array = await http.request_completed
		http.queue_free()
		var status: int = res[1]
		var text := (res[3] as PackedByteArray).get_string_from_utf8()
		var parsed: Variant = JSON.parse_string(text) if text != "" else null
		if status >= 200 and status < 300:
			return { "ok": true, "status": status, "data": parsed, "problem": {} }
		if status == 401 and not refreshed:
			refreshed = true
			if await _auth.refresh():
				continue
			EventBus.session_expired.emit()
		var retryable := res[0] != HTTPRequest.RESULT_SUCCESS or status in [502, 503, 504]
		# Chỉ retry request an toàn hoặc có Idempotency-Key
		if retryable and attempt < MAX_RETRIES and (method == HTTPClient.METHOD_GET or idempotency_key != ""):
			attempt += 1
			await get_tree().create_timer(Backoff.delay(attempt)).timeout
			continue
		var problem: Dictionary = parsed if parsed is Dictionary else { "status": status, "code": "NETWORK_ERROR" }
		request_failed.emit(problem)
		return { "ok": false, "status": status, "data": null, "problem": problem }
	return {}
```

### 13.2. Đăng nhập bằng form trong game qua backend (ADR-0068)

```gdscript
# scripts/authentication/auth_service.gd (trích)
var tenant_id := ""                                     # tenant của phiên, chỉ trong RAM (ADR-0065)
var _access_token := ""
var _refresh_token := ""                                # CHỈ trong RAM, không ghi đĩa/Keychain/Keystore (ADR-0064)
var _access_expires_at := 0.0

## Trả { ok, code, choices, login_ticket, retry_after }
func login(username: String, password: String) -> Dictionary:
	var body := {
		"username": username.strip_edges(),
		"password": password,                            # không log, không telemetry
		"device_id": Device.id,
		"client_version": Config.client_version,
		"locale": Settings.locale(),
	}
	var res: Dictionary = await Api.post_public("/api/v1/auth/game/login", body)  # không Authorization, không X-Tenant-Id
	body.clear()                                        # best effort xóa mật khẩu khỏi bộ nhớ
	return _handle_login_response(res)

func select_tenant(login_ticket: String, chosen_tenant_id: String) -> Dictionary:
	var res: Dictionary = await Api.post_public("/api/v1/auth/game/tenant:select",
		{ "login_ticket": login_ticket, "tenant_id": chosen_tenant_id })
	return _handle_login_response(res)

func _handle_login_response(res: Dictionary) -> Dictionary:
	if res.ok:
		var d: Dictionary = res.data
		_access_token = d["access_token"]
		_refresh_token = d["refresh_token"]
		_access_expires_at = Time.get_unix_time_from_system() + float(d["expires_in"]) - 30.0
		tenant_id = d["bootstrap"]["tenant_id"]          # ApiClient gắn X-Tenant-Id từ đây
		AppState.apply_bootstrap(d["bootstrap"])
		return { "ok": true }
	var p: Dictionary = res.problem
	return { "ok": false, "code": p.get("code", "NETWORK_ERROR"), "choices": p.get("choices", []),
		"login_ticket": p.get("login_ticket", ""), "retry_after": res.get("retry_after", 0) }

func refresh() -> bool:                                 # gọi qua backend vì client là confidential
	var res: Dictionary = await Api.post_public("/api/v1/auth/game/token:refresh",
		{ "refresh_token": _refresh_token }, { "X-Tenant-Id": tenant_id })
	if not res.ok:
		return false
	_access_token = res.data["access_token"]
	_refresh_token = res.data["refresh_token"]
	_access_expires_at = Time.get_unix_time_from_system() + float(res.data["expires_in"]) - 30.0
	return true

func logout() -> void:
	await Api.post_public("/api/v1/auth/game/logout", { "refresh_token": _refresh_token })  # best effort
	_access_token = ""; _refresh_token = ""; tenant_id = ""
```

Ghi chú: game không xác thực chữ ký token; **xác thực có thẩm quyền do backend thực hiện** khi nhận access token (ADR-0008). `Api.post_public` dùng cùng pool `HTTPRequest`, tắt ghi body vào log debug cho mọi path `/auth/game/*`.

## 14. Việc còn mở

- Certificate pinning trong Godot (desktop/mobile) cho tên miền API [Proposed, ADR-0068] — chọn cơ chế (`TLSOptions.client` với chứng chỉ CA riêng hoặc pin public key) và quy trình xoay chứng chỉ.
- Danh sách GPU/SoC cần ép Compatibility hoặc preset Thấp (`client-config.device_profiles`) — lập sau khảo sát thiết bị trường (OQ-04) và benchmark Phase 1.
- Device Authorization Grant cho máy dùng chung: ngoài Release 1.0 (ADR-0063), mở rộng sau.
