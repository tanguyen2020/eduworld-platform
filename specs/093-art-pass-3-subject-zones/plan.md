# Implementation Plan: F093 — Art pass 3: năm vùng môn học theo "Chibi anime cartoon 3D"

**Branch**: `093-art-pass-3-subject-zones` · **Date**: 2026-11-25 · **Spec**: [spec.md](spec.md)

## Summary

Dùng nguyên pipeline F067/F068 (công thức trong `src/game/tools/art/` → bake `.res`, màu là slot token `art.palette.*`, `lint_art`, `bake_art --check`, benchmark có renderer) cho năm vùng môn học:

- **Prefab vùng chung** `zone_kit.gd` (WORLD_DESIGN §4.2): bố cục điểm neo của seed F071 (spawn (0, 8), NPC (0, 2), khám phá (±12, −6), vật sưu tầm (0, −14), cổng gió (0, −20)); `terrain()` gần (cỏ hai tông, quảng trường tròn, lối chính, lối ngang, bệ lát), `far()` cảnh xa, `boundary()` vòng rào ±24 m (collision `Boundary*` trong scene), `house()`, `lamp()`, `flower_bed()`, `scatter()` (luật tránh lối đi, quảng trường, bệ và vùng cấm của từng môn).
- **Công thức từng môn** `zone_{math,vietnamese,english,science,history}_art.gd`: landmark sau cổng gió (trong `Buildings`), set piece cạnh điểm neo (mesh riêng, đặt bằng transform của node trong scene), `props()` (đèn theo màu môn, ghế, bồn hoa, đồ trang trí), cây riêng khi cần (`veg_tree_autumn_a`, `veg_tree_blossom_a`, `veg_palm_coconut_a`), `SCATTER` + `placements()`; đăng ký ở `ArtCatalog.ZONES` → `bake_art` bake mesh và `ScatterSet`, `drift()` kiểm cả hai.
- **Cổng gió về Thị trấn**: biểu tượng ngôi nhà (`prop_wind_gate_main_hub.res`), `AreaPortal` tự nạp theo `target_area_id`.
- **Token** `tokens@0.11.0`: 8 slot mới (52–59).
- **Scene**: phần tay (Environment, Terrain/Ground + Terrain/Far, Static: collision + mesh, Ambient: ScatterInstance) trên marker; `gen_greybox_zones.mjs` giữ phần tay của **mọi** scene đã có marker và chỉ sinh lại khối neo (trước đây chỉ Hub).
- **Ngân sách**: cảnh xa bake riêng, vẽ bằng `mat_toon_no_outline`, không đổ bóng (đồi bậc chia 2 cho dải cel mượt mà không nhân đôi primitive); cây lặp bậc chia gốc; số cây ngoài rào và hoa/cỏ giới hạn theo đo đạc.
- **Công cụ**: `tools/capture_zone.gd` (5 góc), `tests/perf/ZoneBenchmark.tscn` (`--area=`), `test_zone_benchmark.gd`, `test_subject_zone_art.gd`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2), static typing; Node 20 (generator, pipeline token) |
| Primary Dependencies | GdUnit4; pipeline token F005 (`design/`, Style Dictionary 4) |
| Storage | Không có dữ liệu nghiệp vụ; mesh/`ScatterSet` `.res` nén trong repo |
| Testing | GdUnit4 headless; benchmark có renderer `ZoneBenchmark.tscn` (Mobile + Compatibility); `capture_zone.gd`; export Web (preset F133) |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | `scene_budget_zone`, PERFORMANCE_BUDGET §8 |
| Constraints | AGENTS R1, R2, R17, R32, R33, R34 · ADR-0047, ADR-0057, ADR-0101, ADR-0102, ADR-0103 |
| Scale/Scope | 5 vùng môn học; 6 vùng còn lại → F094 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec từ ADR-0103, WORLD_DESIGN §2.2/§4.2/§11; không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client + design token; không backend, không contract |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu tenant/học sinh |
| IV Server-authoritative | ✅ | Điểm neo giữ nguyên seed; server quyết định khám phá/quest như cũ |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Không vũ khí/bạo lực (cổng thành không có lính/súng; núi lửa không dung nham); `zone_lint` kiểm từ khóa |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc ở WORLD_DESIGN/WORLD_RENDERING/PERFORMANCE_BUDGET; tham số `scene_budget_zone` ở CONFIG_DEFAULTS; màu ở token |
| VIII UI tách logic, token | ✅ | Scene chỉ hiển thị; file sinh không sửa tay |
| IX Test có bằng chứng | ✅ | Mỗi F093-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
design/game-tokens/{primitive,art3d}.json, design/CHANGELOG.md, design/package.json            # tokens@0.11.0
src/game/tools/art/{zone_kit,zone_math_art,zone_vietnamese_art,zone_english_art,zone_science_art,zone_history_art}.gd   # mới
src/game/tools/art/{art_catalog,bake_art,hub_art}.gd                                           # ZONES, scatter vùng, icon ngôi nhà
src/game/tools/{capture_zone.gd,gen_greybox_zones.mjs}
src/game/assets/models/{env,bld,prop,veg}/*_{math,vietnamese,english,science,history}_*.res, prop_wind_gate_main_hub.res
src/game/scenes/world/zones/learning_zone_{math,vietnamese,english,science,history}/Zone.tscn
src/game/tests/perf/{ZoneBenchmark.tscn,zone_benchmark.gd,test_zone_benchmark.gd}, tests/scene/world/test_subject_zone_art.gd
src/game/assets/LICENSES.md
```

## Phase 0: Research

- Bố cục điểm neo của năm vùng môn học giống nhau (seed F071) → một prefab chung + cấu hình (WORLD_DESIGN §4.2), set piece đặt **cạnh** điểm neo (marker `WorldAnchor` vẫn thấy được), riêng vật sưu tầm nằm giữa một cổng/vòm trên lối chính.
- Lần đo đầu (Mobile, preset Trung bình): Vương quốc Số 131k primitive, Đảo Khoa Học 142k (cây cau 492 tam giác × 30 cây × 3 pass), Rừng Ngôn Ngữ 125k → tách cảnh xa sang preset không viền/không bóng (−5k/pass), cây dừa nhẹ (304), bớt cây ngoài rào, tre, hoa → 103–115k.
- Viền inverted hull trên mặt phẳng hai mặt dày 0 (mái chợ, lá cau) tô đen mặt sau → mọi tấm mỏng mới là khối kín (mái sọc, lá dừa).
- Compatibility đếm ít primitive hơn (không pass bóng nắng): 60–67k.
- Ma trận `Transform3D` trong `.tscn` theo hàng; yaw −90° = `(0, 0, -1, 0, 1, 0, 1, 0, 0)` (mặt trước +Z quay về −X).

## Phase 1: Design & Contracts

- Không có contract API/event mới (contracts/README.md).
- Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.
