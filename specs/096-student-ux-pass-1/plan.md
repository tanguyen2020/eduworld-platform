# Implementation Plan: F096 — Student UX pass 1: nhận diện EduWorld và làm mượt hành trình học sinh

**Branch**: `096-student-ux-pass-1` · **Date**: 2026-11-26 · **Spec**: [spec.md](spec.md)

## Summary

Hai phần, cùng pipeline token/icon của F005/F060 và thành phần S01 đã có:

- **Nhận diện (US1)**: component `BrandLockup` (Tia Sáng `SparkMark` + wordmark `LogoLabel` + `CloudyMascot`) và `SkyBackdrop` dùng cho splash trong game, "Đang vào lớp…", giờ nghỉ, hàng chờ. Công cụ `tools/brand_assets.gd` (cần renderer) dựng `BrandLockup` trong `SubViewport` trong suốt → `resources/branding/boot_splash.png`, icon 1024/192/432 (nền trời bo góc + Tia Sáng + Mây); ghi `application/boot_splash/*`, `application/config/icon` vào `project.godot` và các key icon vào `export_presets.cfg` bằng thay dòng (giữ chú thích); `--check` (headless) kiểm file, kích thước, cài đặt, màu nền = token, dòng `LICENSES.md`. Shell Web: thêm logo + Mây SVG nội tuyến tô bằng biến CSS, thanh tiến độ bo tròn (div), câu thân thiện vi/en; `game-shell-css.mjs` sinh thêm tiền tố `component.logo.`, `component.backdrop.`, `component.mascot.`, `component.progress.` (`tokens@0.13.0`). Trang **Giấy phép** trong `SettingsPanel` (Engine API).
- **Hành trình (US2–US5)**: sửa transform `SpringArm3D` (pitch −20°, trước đây +20° chĩa xuống đất); `ToastStack` đặt `body_key` trước khi thêm toast (UiComponent ẩn Body rỗng ở `_ready`); icon cho nút HUD (Button.icon, Theme `Button/constants/icon_max_width`), nhãn không "(J)", dịch "Emote"; `OnboardingHint` (Mây + bong bóng + 3 bước, làm nổi nút đích) + model `OnboardingGuide`; `NextStepCard` + model `NextStepHint`; World Map: ô hình có icon + tên (`g1_3`), hàng danh sách icon + cột chữ co giãn + nút có chữ; Cài đặt: `ChoiceChip` cho ngôn ngữ/preset (nền nhấn + ✓), nhóm Thông tin (Giấy phép, Xem lại hướng dẫn); Kết quả: `CelebrationBanner` (Mây vui + sao icon, tween ≤ 1,5 s, tắt khi giảm chuyển động), bỏ "1/11" ở header khi xem kết quả; thay emoji bằng icon; S01 nút "Hủy" sau 10 s; `L10n` ánh xạ `HTTP_502/503/504` → `UNAVAILABLE` với chuỗi mới.
- **Bằng chứng**: `tools/capture_screens.gd` (+ `tools/lib/screen_capture.gd`) chụp hành trình trên fixture/fake ở Compatibility; Playwright chụp shell Web.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, static typing); HTML/CSS/JS thuần (shell); Node 20 (pipeline token, test shell) |
| Primary Dependencies | GdUnit4; Style Dictionary 4 (`design/`); Playwright (`src/cms/frontend/apps/portal`) chỉ để chụp/kiểm shell |
| Storage | Cờ thiết bị `onboarding_hub_seen` trong `user://settings.cfg` (Settings autoload); không dữ liệu nghiệp vụ |
| Testing | GdUnit4 headless (unit, scene); `node --test src/game/web/tests/`, `scripts/ci/`; `brand_assets.gd --check`; capture có renderer |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | `STUDENT_GAME_UX` §6: ≤ 6 tween đồng thời, panel mở ≤ 200 ms; shell không tải thêm file ngoài `shell_tokens.css` |
| Constraints | AGENTS R1, R17, R19, R32, R34 · ADR-0047, ADR-0101, ADR-0102, ADR-0103 |
| Scale/Scope | 13 màn của bảng kiểm, 20 AC |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready trước code; không contract mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; model thuần (`OnboardingGuide`, `NextStepHint`) ngoài scene |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu tenant mới; cờ thiết bị không PII |
| IV Server-authoritative | ✅ | Phần thưởng chỉ hiển thị thứ server trao; không tính điểm ở client |
| V Xác thực, phân quyền | ✅ | Không endpoint mới; "Hủy" chỉ bỏ phản hồi phía client |
| VI An toàn trẻ em | ✅ | Không FOMO/streak/so sánh (R19, NFR-SAFE-003); không lộ thông tin cá nhân |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc UX ở STUDENT_GAME_UX*, tham số ở tài liệu sở hữu; màu ở token |
| VIII UI tách logic, token | ✅ | Scene chỉ hiển thị; file sinh không sửa tay; icon qua pipeline |
| IX Test có bằng chứng | ✅ | Mỗi F096-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
design/game-tokens/component.json, design/sd/formats/game-shell-css.mjs, design/icons/game/*.svg, design/CHANGELOG.md, design/package.json
src/game/project.godot, src/game/export_presets.cfg, src/game/resources/branding/*.png, src/game/assets/LICENSES.md
src/game/tools/{brand_assets.gd,capture_screens.gd}, src/game/tools/lib/screen_capture.gd
src/game/web/shell.html, src/game/web/tests/*.mjs
src/game/scenes/ui/components/{brand_lockup.gd,BrandLockup.tscn,celebration_banner.gd}
src/game/scenes/bootstrap/Splash.tscn, src/game/scenes/auth/{ResumeSplash,CurfewNotice,LoginWaiting,Login}.tscn + .gd
src/game/scenes/character/Player.tscn
src/game/scenes/ui/hud/{MainHud.tscn,main_hud.gd,toast_stack.gd,OnboardingHint.tscn,onboarding_hint.gd,NextStepCard.tscn,next_step_card.gd}
src/game/scenes/ui/settings/{SettingsPanel.tscn,settings_panel.gd}
src/game/scenes/world/{WorldMap.tscn,world_map.gd,WorldRoot.tscn,world_root.gd}
src/game/scenes/learning/{ActivityResult.tscn,activity_result.gd,activity_host.gd,hint_panel.gd,voice_over_player.gd}
src/game/scripts/core/{onboarding_guide.gd,next_step_hint.gd,l10n.gd,settings.gd}
src/game/locale/{vi.po,en.po,messages.pot,xx_PS.po}
src/game/tests/{unit,scene}/…
```

## Phase 0: Research

- Godot 4.7 `application/boot_splash/{image,bg_color,use_filter,stretch_mode,show_image}`; Web export `html/export_icon` tạo `index.icon.png`/`index.apple-touch-icon.png` từ `application/config/icon`; Android `launcher_icons/*`, iOS `icons/icon_1024x1024` rơi về icon project khi để trống.
- Godot Web không có SystemFont → ký tự ngoài font đóng gói (emoji) thành ô vuông; kiểm bằng `Font.has_char` trên font của Theme.
- `SpringArm3D` transform trong `Player.tscn` có basis xoay +20° quanh X (cột Z = (0, −0,34, 0,94)) → cần −20° (cột Z = (0, 0,34, 0,94)).
- `UiComponent._ready` gán `Body.text = body_key` và ẩn khi rỗng → `ToastStack.push` gán chữ sau khi add nên Body ẩn.

## Phase 1: Design & Contracts

- Không contract API/event mới (contracts/README.md).
- Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.
