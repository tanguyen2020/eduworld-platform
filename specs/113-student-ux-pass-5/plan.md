# Implementation Plan: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

**Branch**: `113-student-ux-pass-5` · **Date**: 2026-11-26 · **Spec**: [spec.md](spec.md)

## Summary

Ba phần, chỉ phía client, theo quy tắc ngang của `STUDENT_GAME_UX` §2:

- **Native (US1)**: `project.godot` `[display] window/handheld/orientation=4` (sensor landscape). Test cấu hình `test_orientation_config.gd` đọc `project.godot` (ConfigFile) và `export_presets.cfg`: giá trị 4, preset Android không có khóa `screen/orientation`/`orientation`, iOS `additional_plist_content` không chứa `UISupportedInterfaceOrientations`/`Portrait`.
- **Web/cửa sổ dọc (US2)**: `OrientationPolicy` (tĩnh) + autoload `OrientationGuard` (#14, `PROCESS_MODE_ALWAYS`): nối `get_tree().root.size_changed`, `apply(view)` → hiện/ẩn `RotatePrompt` trên `CanvasLayer` lớp cao (phóng theo cạnh ngắn), dừng/bỏ dừng SceneTree qua hai Callable tiêm được (test không dừng runner), trên Web gọi `JavaScriptBridge.eval` thử `screen.orientation.lock`. `ApiClient`, `RealtimeClient` đặt `PROCESS_MODE_ALWAYS`. `RotatePrompt.tscn`: `SkyBackdrop`, cột `FormBox`: `SpeechBubble` (tiêu đề `HeadingLabel`… lời phụ), hàng Mây + icon `rotate_phone`. Shell: khối `#ew-rotate` trong `#ew-shell` (HTML/SVG, biến token), `isPortrait()` + `data-portrait` cập nhật theo `resize`/`orientationchange`, `tryLockLandscape()` trong handler "Chơi".
- **Dọn & bằng chứng (US3)**: `BandLayoutProbe.SIZES` → `1280x720`, `1024x600`, `844x390`; `PORTRAIT_SIZES` (`720x1280` → 1280×2276, `390x844` → 1280×2770) chỉ cho `test_rotate_prompt_fits_every_band`. `screen_capture.gd`: cỡ `844x390`, `390x844`, màn `rotate_prompt`; bỏ `720x1280`. Ảnh Chromium bằng Playwright của `src/cms/frontend` (script tạm ngoài repo).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, static typing); JavaScript (shell, Node 20 test) |
| Primary Dependencies | GdUnit4; jsdom (`scripts/`); Playwright (`src/cms/frontend`) |
| Storage | Không (shell không dùng storage trình duyệt — ADR-0086) |
| Testing | GdUnit4 headless (unit, scene); `node --test src/game/web/tests/`; capture có renderer |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | Guard chỉ chạy khi cửa sổ đổi cỡ (signal), không mỗi khung; màn nhắc vẽ tĩnh (không animation) |
| Constraints | AGENTS R1, R32, R33, R34 · ADR-0047, ADR-0086, ADR-0101 |
| Scale/Scope | 1 autoload, 1 policy, 1 scene, 1 icon, 3 khóa dịch, shell + 2 test web |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec + Clarifications [Confirmed] trước code; không contract mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; logic ở `OrientationGuard`/`OrientationPolicy`, scene chỉ hiển thị (R1) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu mới; shell không dùng storage |
| IV Server-authoritative | ✅ | Không đổi; mạng chạy tiếp sau màn nhắc nên server giữ phiên/trạng thái |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Lời tử tế, không đếm ngược, không ép thao tác |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc hướng ở `STUDENT_GAME_UX` §2; cài đặt hiển thị ở `GODOT_ARCHITECTURE` §10.2; không tham số mới |
| VIII UI tách logic, token | ✅ | Màu/cỡ từ token/Theme; shell chỉ biến `shell_tokens.css` |
| IX Test có bằng chứng | ✅ | Mỗi F113-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
src/game/project.godot (orientation, autoload OrientationGuard)
src/game/scripts/core/{orientation_policy,orientation_guard}.gd
src/game/scripts/networking/{api_client,realtime_client}.gd (process_mode)
src/game/scenes/ui/rotate_prompt/{RotatePrompt.tscn,rotate_prompt.gd}
design/icons/game/rotate_phone.svg → src/game/resources/ui/icons/rotate_phone.svg (sinh), design/CHANGELOG.md, src/game/assets/LICENSES.md
src/game/web/shell.html, src/game/web/tests/{shell.test.mjs,shell.browser.test.mjs}
src/game/locale/{vi,en}.po, messages.pot, xx_PS.po
src/game/tools/lib/screen_capture.gd
src/game/tests/{unit/core/test_orientation_config.gd,unit/core/test_orientation_policy.gd,scene/test_orientation_guard.gd,scene/test_grade_band_matrix.gd,scene/test_boot.gd,fakes/band_layout_probe.gd}
```

## Phase 0: Research

- `project.godot` không có `display/window/handheld/orientation` → mặc định `0` (một chiều ngang). Enum Godot 4.7: `SCREEN_LANDSCAPE=0`, `SCREEN_SENSOR_LANDSCAPE=4` (kiểm bằng Godot 4.7.2 console).
- Preset Android (Godot 4.7) không còn tùy chọn hướng (lấy từ cài đặt dự án); preset iOS chỉ có thể ghi đè qua `additional_plist_content` (đang rỗng).
- Canvas: `stretch canvas_items`/`expand` trên 1280×720 → cửa sổ 390×844 thành logic 1280×2770 (hệ số 0,305): màn nhắc phải phóng riêng (CanvasLayer `scale` = cạnh ngắn / 720).
- `ClientClock.wait` dùng `create_timer(process_always = true)` → heartbeat game session chạy khi SceneTree dừng; `ApiClient` (pool `HTTPRequest` con) và `RealtimeClient._process` (poll WebSocket) cần `PROCESS_MODE_ALWAYS`.
- Shell: `#ew-shell` ẩn khi engine chạy → lời nhắc HTML đặt trong `#ew-shell` tự hết khi game chạy (game tự có màn nhắc).

## Phase 1: Design & Contracts

- Không contract API/event mới (contracts/README.md). Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.
