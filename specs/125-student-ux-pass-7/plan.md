# Implementation Plan: F125 — Student UX pass 7: phóng to giao diện trên điện thoại và nút đóng đỏ cho mọi cửa sổ

**Branch**: `125-student-ux-pass-7` · **Date**: 2026-11-27 · **Spec**: [spec.md](spec.md)

## Summary

Hai phần, chỉ phía client, trên token và component chung của F113/F114:

- **Chế độ `compact` (US1)**: `CompactPolicy` (thuần, `scripts/core/compact_policy.gd`) quyết định `compact` khi cạnh ngắn của cửa sổ / mật độ màn hình (`DisplayServer.screen_get_scale()`: CSS px trên Web, dp trên Android, pt trên iOS) < `eduworld/ui/compact_short_side` (500); bàn phím ảo được cộng lại vào chiều cao trước khi quyết định. Hệ số `content_scale_factor` của chế độ = chiều cao canvas logic (stretch `canvas_items` + `expand` trên 1280×720) / `component.compact.logical_height` (540), chặn để canvas không hẹp hơn 960 (16:9), không bao giờ < 1; ngoài `compact` = 1. `OrientationGuard` (đã sở hữu `content_scale_factor` từ F113) áp dụng trên mọi lần đổi cỡ cửa sổ: ghi hệ số chỉ khi khác (không nháy), gọi `AppState.set_compact()`; khi màn "Xoay ngang" hiện, hệ số của nó thắng; khi ẩn, trả về hệ số của chế độ hiện hành (thay cho "hệ số trước đó" của F113). `AppState.compact` + tín hiệu `compact_changed`; `ThemeService.set_compact()` đổi sang Theme `compact` của khối (`UiTokens.COMPACT_THEME_PATHS`) và phát `density_changed` (dock chat, HUD xếp lại). Token `tokens@0.21.0`: primitive `size.control.60`, `size.layout.540`; component `component.compact.logical_height` = 540; density mới `density/compact.json`: `density.compact.button.min_height` = 60, `density.compact.font.small` = 18. Bộ sinh Theme (`compactMap`) dựng 4 Theme `eduworld_theme_<band>_compact.tres` với `button.min_height` và `font.small` = max(khối, compact) — đệm nút/ô nhập/hộp kiểm theo chiều cao mới, cỡ chữ nhỏ nhất nâng lên 18; không cỡ nào bị hạ. `UiBuild.target()`/`target_for(band)` = `CompactPolicy.button_target(band, AppState.compact)`; mọi nơi đọc thẳng `DENSITY_BUTTON_MIN_HEIGHT` trong scene (HUD, chuông, vòng biểu cảm, thanh media, tạo nhân vật, phối đồ) chuyển sang helper; HUD xếp lại khi `compact_changed`. Màn không vừa canvas 540 được xếp lại (cuộn, cột, gom), không thu chữ.
- **Nút đóng đỏ chung (US2)**: `UiBuild.title_bar(title_key, icon)` = khung tiêu đề tầng 1 `WindowTitleBar` (icon, `WindowTitleLabel` in hoa, xuống dòng) + `UiBuild.close_button()` (`DangerButton`, icon `cross`, "Đóng", `name = Close`) ở cuối. `UiBuild.window` (Thành tựu/Bộ sưu tập, Thử thách, kết quả thử thách, Tủ đồ/Cửa hàng, Hành trình, menu "Thêm") và `ArenaUi.window` dùng nó; 4 cửa sổ dựng bằng `.tscn` (Nhật ký quest, Bản đồ, Phối đồ, sảnh đề thi) đổi `Header` thành cùng cấu trúc (`TitleBar/Row/{TitleIcon,Title,Close}`, `UiBuild.fit_icon`). Nhóm: bỏ nút phụ "Thu nhỏ", nút đỏ luôn hiện ở cuối (trong nhóm có tooltip "Nhóm vẫn còn…", "Rời nhóm" giữ nguyên). Cài đặt, Hộp thông báo, chọn bạn mời, chọn hoạt động đã đúng (F114/F119/F123). Esc/Back: thêm `_unhandled_input` cho kết quả thử thách (các cửa sổ khác đã có). Không đổi hộp xác nhận ("Hủy"), màn làm bài/thi ("Thoát"), màn toàn khung.
- **Bằng chứng (US3)**: `BandLayoutProbe.SIZES` thay 844×390 (1558×720) bằng hai điện thoại `compact` 844×390 → 1168×540 và 740×360 → 1110×540 (Theme `compact`, ngưỡng `CompactPolicy`), probe vật lý (cỡ × CSS px/px logic ≥ 12 / ≥ 40) và kiểm ngược `LEGACY_PHONE` 1558×720 hệ số 1; `tools/lib/screen_capture.gd` thêm cửa sổ 740×360 (cửa sổ thật → `OrientationGuard` bật `compact` như trên máy).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, static typing); Node 20 (pipeline token) |
| Primary Dependencies | GdUnit4; Style Dictionary 4 (`design/`) |
| Storage | Không (RAM: `AppState.compact`; cài đặt dự án `eduworld/ui/compact_short_side`) |
| Testing | GdUnit4 headless (unit, scene, ma trận khối); `pnpm test` + `checks/run-all.mjs`; capture có renderer Compatibility |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | Không thêm render pass; hệ số chỉ ghi khi cửa sổ đổi cỡ và giá trị khác; Theme `compact` sinh sẵn, cache theo khóa |
| Constraints | AGENTS R1, R17, R32, R33, R34 · ADR-0047, ADR-0057 |
| Scale/Scope | 1 policy, 4 Theme sinh mới, 3 token + 2 primitive, ~16 cửa sổ, ma trận 4 cửa sổ × 4 khối |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready + Clarifications [Confirmed] (owner 2026-11-27, "chốt hết"); mâu thuẫn 56 → 60 và sàn chữ 18 ghi ở Clarifications plan; không contract mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; quy tắc ở `CompactPolicy` (thuần), áp dụng ở `OrientationGuard`/`ThemeService`, scene chỉ hiển thị (R1) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu mới, không gửi server, không lưu đĩa |
| IV Server-authoritative | ✅ | Không đổi luật; đóng cửa sổ không đổi trạng thái server (nhóm vẫn còn khi đóng) |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Chữ ≥ 12 px, vùng chạm ≥ 40 px trên điện thoại; đỏ chỉ cho rời đi/đóng (F114); hộp xác nhận giữ "Hủy" trung tính |
| VII Một nguồn sự thật, cấu hình | ✅ | Ngưỡng 500 ở `STUDENT_GAME_UX` §2 (cài đặt dự án); 540 / 60 / 18 ở `DESIGN_SYSTEM` §5.2 (token); sàn vật lý 12 / 40 ở `DESIGN_SYSTEM` §9 (chỉ test); đăng ký CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Theme `compact` sinh bằng `make design-tokens`; không override, không literal màu/cỡ |
| IX Test có bằng chứng | ✅ | Mỗi F125-AC có test (bảng dưới) + ảnh quickstart §5 |

| AC | Test |
|---|---|
| F125-AC-01 | `tests/unit/core/test_compact_policy.gd`, `tests/scene/test_compact_scale.gd` (guard, Window thật 540) |
| F125-AC-02 | `test_compact_scale.gd::test_the_rotate_prompt_factor_wins_and_the_mode_factor_returns`, `test_orientation_guard.gd` |
| F125-AC-03 | `test_compact_policy.gd` (sàn vật lý mọi khối), `test_compact_scale.gd` (Theme compact), ma trận khối (probe vật lý) |
| F125-AC-04 | `tests/scene/test_grade_band_matrix.gd` (4 cửa sổ × 4 khối, `test_physical_probe_flags_the_unscaled_phone_layout`) |
| F125-AC-05 | `test_compact_scale.gd::test_compact_themes_floor_and_never_lower`, `test_compact_policy.gd::test_compact_floors_never_lower_a_band` |
| F125-AC-06 | `tests/scene/ui/test_window_close_button.gd` (16 cửa sổ × thường/compact, Esc + nút) |
| F125-AC-07 | `test_window_close_button.gd::test_dialogs_activity_and_full_frame_screens_have_no_red_close` |
| F125-AC-08 | quickstart §5 (ảnh, GdUnit4, lint, token, export Web) |

## Project Structure

```text
design/game-tokens/{primitive,component}.json, design/game-tokens/density/compact.json (mới)
design/style-dictionary.config.mjs, design/lib/pipeline.mjs, design/sd/formats/{godot-tres,gdscript-tokens}.mjs, design/checks/structure.mjs
design/package.json (0.21.0), design/CHANGELOG.md
src/game/resources/ui/theme/eduworld_theme_<band>_compact.tres (sinh), src/game/scripts/core/ui_tokens.gd (sinh)
src/game/project.godot (eduworld/ui/compact_short_side)
src/game/scripts/core/{compact_policy (mới),orientation_guard,app_state,theme_service}.gd
src/game/scenes/ui/components/ui_build.gd, scenes/competition/components/arena_ui.gd, scenes/social/party_lobby.gd
src/game/scenes/{quest/QuestJournal,world/WorldMap,character/CustomizationStudio,exam/ExamLobby}.tscn + script
src/game/scenes/challenge/challenge_result.gd, scenes/ui/hud/{main_hud,notification_bell}.gd, scenes/ui/components/media_track.gd
src/game/scenes/character/{character_creation_screen,customization_studio}.gd, scripts/player/emote_wheel.gd
src/game/scenes/** (xếp lại màn không vừa canvas 540)
src/game/tools/lib/screen_capture.gd
src/game/tests/{unit/core/test_compact_policy.gd,scene/test_compact_scale.gd,scene/ui/test_window_close_button.gd}
src/game/tests/{fakes/band_layout_probe.gd,scene/test_grade_band_matrix.gd,scene/test_orientation_guard.gd}
```

## Phase 0: Research

- Godot 4.7.2, `stretch canvas_items` + `expand`: hệ số tỉ lệ cuối = min(cửa sổ / 1280×720) × `content_scale_factor`; với cửa sổ ngang rộng hơn 16:9 chiều cao logic = 720 / hệ số → hệ số 4/3 cho mọi điện thoại ngang (độc lập mật độ: Web canvas = CSS px × `devicePixelRatio`, `screen_get_scale()` = `devicePixelRatio`). Kiểm bằng `Window` thật (844×390, hệ số 4/3 → rect hiển thị 1168,6 × 540).
- CSS px mỗi px logic: 844×390 → 0,722; 740×360 → 0,667. Hệ quả: chữ 16 (`font.small` g6_7/g8_9) = 10,7 px và nút 56 = 37,3 px ở 740×360 → cần sàn 18 và 60 (Clarifications plan). Chữ 18 = 12,0 px, nút 60 = 40,0 px.
- `screen_get_scale()` trên Windows trả 1 (ảnh chụp và máy bàn không đổi); Android trả mật độ dp; iOS `nativeScale`.
- Chiều cao nút đến từ đệm StyleBox sinh theo `density.<band>.button.min_height` (F114 `raised()`), nên Theme `compact` phải sinh lại (không thể chỉ đổi `custom_minimum_size`); cỡ chữ nhỏ (`SectionTag`, `CountBadge`, `ChatMetaLabel`, `TileTag*`…) dùng `font.small`.
- Bàn phím ảo: `DisplayServer.virtual_keyboard_get_height()`; trên Web canvas không co theo bàn phím (viewport bố cục giữ nguyên), nhưng cộng lại vẫn an toàn.

## Phase 1: Design & Contracts

- Không contract API/event mới ([contracts/README.md](contracts/README.md)). Model hiển thị: [data-model.md](data-model.md). Quy trình duyệt: [quickstart.md](quickstart.md).
- Re-check Constitution sau thiết kế: không đổi (✅ cả 9 điều).

## Complexity Tracking

Không có vi phạm cần biện minh.
