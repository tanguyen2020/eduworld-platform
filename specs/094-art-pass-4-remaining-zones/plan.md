# Implementation Plan: F094 — Art pass 4: sáu vùng còn lại theo "Chibi anime cartoon 3D" và các điểm mở của F093

**Branch**: `094-art-pass-4-remaining-zones` · **Date**: 2026-11-25 · **Spec**: [spec.md](spec.md)

## Summary

Dùng nguyên pipeline F067/F068/F093 (công thức trong `src/game/tools/art/` → bake `.res`, màu là slot token `art.palette.*`, `lint_art`, `bake_art --check`, benchmark có renderer, `capture_zone.gd`) cho sáu vùng còn lại và đóng F093-1..3:

- **Bố cục tùy biến trên prefab chung** (`zone_kit.gd`): tách `terrain()` của F093 thành các khối dùng lại (`lawn()`, `plaza()`, `pad()`, `path_strip()`; đầu ra của năm vùng F093 không đổi), thêm `clear_of()`/`scatter_in()` nhận bố cục của vùng (tâm quảng trường, các lối là hình chữ nhật, các bệ tròn, vùng cấm), đồ dùng chung mới (`pennant()`, `bunting()`, `balloons()`, `cairn()`, `rock()`), cây thông `veg_pine_a`.
- **Công thức từng vùng** `zone_{practice,social,ocean,mountain,exam,arena}_art.gd`: landmark sau cổng gió (`Buildings`), set piece cạnh điểm neo, `props()`, `SCATTER` + `placements()`; đăng ký ở `ArtCatalog.ZONES`.
- **Scene**: phần tay (Environment, Terrain/Ground + Terrain/Far, Static: collision + mesh, Ambient) viết trên marker `BEGIN F071 generated anchors`; khối neo do `gen_greybox_zones.mjs` sinh (không đổi điểm neo).
- **Token** `tokens@0.12.0`: `color.subject.history` (shared), slot `art.palette.subject_history` (60), `env.{history,ocean,mountain,exam,arena}`; portal: nhãn `domain.subject.history`.
- **F093-1**: chạy `gen_greybox_zones.mjs` cho Hub (thêm 3 `WorldAnchor` quest F072 theo seed); `test_interactable.gd` rời bảng tin tới (0, 0.2, 18) (lối nam, > 2 m + bán kính tầm với tới mọi điểm neo).
- **F093-3**: lá cau thành khối dẹt kín, cánh sen thành khối kín; test mới `test_outline_meshes_have_no_two_sided_planes` quét mọi mesh bake (trừ decor phẳng) tìm cặp tam giác trùng ngược chiều.
- **Đo**: benchmark có renderer từng vùng × Mobile + Compatibility; giảm rải cây/chia mịn tới khi đạt `scene_budget_zone`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2), static typing; Node 20 (generator, pipeline token) |
| Primary Dependencies | GdUnit4; pipeline token F005 (`design/`, Style Dictionary 4); portal (Vite, Vitest) chỉ nhận token sinh |
| Storage | Không có dữ liệu nghiệp vụ; mesh/`ScatterSet` `.res` nén trong repo |
| Testing | GdUnit4 headless; benchmark có renderer `ZoneBenchmark.tscn` (Mobile + Compatibility); `capture_zone.gd`; export Web (preset F133); portal `pnpm lint`, typecheck, Vitest các file liên quan màu nghiệp vụ |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client (+ token dùng chung) |
| Performance Goals | `scene_budget_zone`, PERFORMANCE_BUDGET §8 |
| Constraints | AGENTS R1, R2, R17, R32, R33, R34 · ADR-0047, ADR-0057, ADR-0101, ADR-0102, ADR-0103 |
| Scale/Scope | 6 vùng + 3 điểm mở F093 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec từ WORLD_DESIGN §2–§4, §11, ADR-0103, F093; không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client + design token; không backend, không contract |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu tenant/học sinh; bảng xếp hạng 3D chỉ khung ô trống |
| IV Server-authoritative | ✅ | Điểm neo = seed; server quyết định quest/khám phá như cũ |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Không vũ khí/bạo lực (xác tàu không đại bác, đấu trường là lễ hội); `zone_lint` kiểm từ khóa |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc ở WORLD_DESIGN/WORLD_RENDERING/PERFORMANCE_BUDGET; màu nghiệp vụ ở shared-tokens; tham số `scene_budget_zone` |
| VIII UI tách logic, token | ✅ | Scene chỉ hiển thị; file sinh không sửa tay |
| IX Test có bằng chứng | ✅ | Mỗi F094-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
design/shared-tokens/domain.json, design/game-tokens/art3d.json, design/CHANGELOG.md, design/package.json   # tokens@0.12.0
src/cms/frontend/packages/ops-ui/src/i18n/{vi,en}.json                                                     # domain.subject.history
src/game/tools/art/{zone_kit,art_catalog,hub_art,zone_history_art}.gd                                      # bố cục tùy biến, ZONES, cau/sen kín
src/game/tools/art/zone_{practice,social,ocean,mountain,exam,arena}_art.gd                                  # mới
src/game/tools/{capture_zone.gd,gen_greybox_zones.mjs}
src/game/assets/models/{env,bld,prop,veg}/*_{practice,social,ocean,mountain,exam,arena}_*.res, veg_pine_a.res
src/game/scenes/world/zones/{practice_zone,social_zone,quest_zone_discovery_ocean,challenge_mountain,exam_zone,arena,main_hub,learning_zone_history}/Zone.tscn
src/game/tests/perf/test_zone_benchmark.gd, tests/scene/world/{test_zone_art_pass_4.gd,test_interactable.gd}
src/game/assets/LICENSES.md
```

## Phase 0: Research

- Bố cục điểm neo của sáu vùng khác bố cục vùng môn học (seed F071): Thư viện Gió (NPC (0, 2), trạm (5, 0)), Vườn Bạn Bè (NPC (0, 2), vật sưu tầm (8, −4)), Đại dương (NPC (0, 3), khám phá (±14, −8), vật sưu tầm (0, −16)), Núi (khám phá (0, −18) sát cổng (0, −20), vật sưu tầm (10, −10)), Tháp (NPC (0, 2), bảng (4, 0)), Đấu trường (hai NPC (±3, 2)) → mỗi vùng khai báo bố cục riêng; luật rải cây và test lối đi đọc bố cục đó.
- Palette shader 64 slot, đã dùng 60 → chỉ thêm `subject_history`; tuyết = `eye_light`, biển sâu = `tile_blue_dark`, tinh thể = `blossom_violet` + `wind` (Câu hỏi mở F094-1).
- F093-3: viền inverted hull của mặt phẳng hai mặt dày 0 tô đen mặt sau (pháp tuyến hàn của hai mặt triệt tiêu) → khối dẹt kín; test tìm cặp tam giác trùng đỉnh ngược chiều.
- F093-1: `welcome_sign` (0, 0, 12) trùng điểm rời bảng tin của test → đổi điểm rời, không dời seed.

## Phase 1: Design & Contracts

- Không có contract API/event mới (contracts/README.md).
- Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.
