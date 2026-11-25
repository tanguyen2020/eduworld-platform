# ADR-0102 — Nguồn asset đồ họa theo license MIT (hoặc dễ dãi hơn), asset tự làm phát hành MIT

> **Trạng thái:** Accepted · Superseded một phần bởi [ADR-0103](ADR-0103-art-direction-chibi-anime-cartoon-3d-thay-stylized-low-poly.md) (câu "phong cách stylized low-poly của OQ-16 giữ nguyên" ở mục 7) · thay phần "nguồn asset" của OQ-16 / [WORLD_DESIGN §11](../01-game-design/WORLD_DESIGN.md#11-art-direction) ("mua pack có license thương mại + tự làm nhân vật chính") · **Ngày:** 2026-11-24 · **OQ:** OQ-16 · **Tài liệu:** [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md) · [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Đã được thay một phần bởi ADR-0103 (2026-11-25).** Không còn hiệu lực: câu "phong cách stylized low-poly của OQ-16 giữ nguyên" ở mục 7 — phong cách hiện hành là "Chibi anime cartoon 3D". **Còn hiệu lực:** toàn bộ quy tắc license (mục 1–6) và phần còn lại của mục 7.

## Bối cảnh

Đến 2026-11-24 game vẫn là greybox (F062 "Main Hub greybox", F071 zone sinh từ seed): khối hộp, viên nang không vật liệu, không có model 3D nào trong repo. OQ-16 (owner 2026-11-20) chốt phong cách stylized low-poly và nguồn asset "mua pack có license thương mại + tự làm nhân vật chính"; chưa pack nào được mua, CODE_PLAN không có feature đồ họa.

Ngày 2026-11-24 owner xem game và yêu cầu: "đảm bảo game phải đẹp và đồ họa phải đúng với mô tả". Khi được hỏi chọn nguồn asset (asset miễn phí CC0 dựng bằng code / chờ pack mua / cả hai), owner trả lời: "làm theo hướng MIT License".

Cách hiểu dưới đây do orchestrator ghi lại để owner sửa nếu chưa đúng (xem Câu hỏi mở ở F067).

## Quyết định

1. **Danh sách license được phép cho asset đồ họa** (model 3D, texture, ảnh, icon, shader, vật liệu, VFX của game client): `MIT`, hoặc dễ dãi hơn — `CC0-1.0`, public domain (`Public-Domain`). [Confirmed — owner 2026-11-24]
2. **Danh sách cấm:** license share-alike (`CC-BY-SA-*`, mọi biến thể `-SA`), non-commercial (`-NC`), no-derivatives (`-ND`), họ GPL (`GPL`, `LGPL`, `AGPL`), license độc quyền / EULA của store và **pack trả phí**. License có điều kiện ghi công nhưng không share-alike (ví dụ `CC-BY-4.0`) **không** nằm trong danh sách được phép cho tới khi owner mở rộng bằng ADR mới. [Confirmed — owner 2026-11-24: "hướng MIT"; phần CC-BY là cách hiểu của orchestrator, Câu hỏi mở F067-1]
3. **Asset tự làm** (mesh low-poly sinh bằng code, scene, shader, bảng màu, vật liệu) phát hành theo **MIT**, ghi `MIT` trong `LICENSES.md` và (với file mã như shader/script dựng asset) dòng `SPDX-License-Identifier: MIT`. Repo hiện chưa có file LICENSE chung; giấy phép của asset tự làm áp dụng theo dòng trong `LICENSES.md`. [Confirmed]
4. **Ghi nhận license:** mọi file asset đồ họa của project Godot có một dòng trong `src/game/assets/LICENSES.md` (= `res://assets/LICENSES.md`, vị trí [WORLD_RENDERING §8](../08-game-client/WORLD_RENDERING.md#8-asset-pipeline) quy định): đường dẫn (glob), URL nguồn chính xác, tác giả, license, ngày tải. Asset ngoài giữ kèm văn bản license gốc khi license yêu cầu (MIT). Script `src/game/tools/lint_art.gd` (F067) chặn file thiếu dòng hoặc license ngoài danh sách được phép. [Confirmed]
5. **Font** không thuộc phạm vi ADR này: giữ license `OFL-1.1` theo [DESIGN_SYSTEM §5.5](../05-ux/DESIGN_SYSTEM.md#55-font) (quyết định riêng, [Confirmed]); lint chỉ chấp nhận `OFL-1.1` cho file font. **Âm thanh**: áp dụng cùng danh sách được phép cho tới khi owner quyết định khác [Proposed — Câu hỏi mở F067-2].
6. **Không tải asset trong phiên làm việc của agent:** khi cần pack ngoài (CC0/MIT), agent liệt kê URL + license để owner tự tải và kiểm; asset chỉ vào repo kèm dòng `LICENSES.md`. [Confirmed — quy tắc phân quyền phiên làm việc]
7. **Phạm vi thay thế:** dòng "Nguồn asset" của WORLD_DESIGN §11 và câu trả lời OQ-16 phần nguồn asset ("mua pack có license thương mại + tự làm nhân vật chính") không còn hiệu lực; phong cách stylized low-poly của OQ-16 giữ nguyên. Lượt dựng hình đầu tiên theo quyết định này là [F067](../../specs/067-art-pass-hub-chibi/spec.md) (Main Hub + nhân vật chibi, toàn bộ tự làm, MIT).

## Hệ quả

- (+) Không phát sinh chi phí mua pack, không vướng ràng buộc share-alike/non-commercial khi phát hành thương mại hoặc khi tenant/trường phân phối bản cài.
- (+) Asset tự làm sinh bằng code (bake trong `tools/art/`) nhẹ, đúng ngân sách hiệu năng, đổi màu qua design token mà không phải dựng lại (R32).
- (+) License kiểm được tự động trong CI (`lint_art.gd`), không phụ thuộc trí nhớ người review.
- (−) Nguồn CC0/MIT chất lượng cao ít hơn store trả phí; nhân vật có rig/animation thật (skeleton `kid_base`, CHARACTER_SYSTEM §5.2) và các vùng khác vẫn phải tự làm hoặc tìm pack CC0 phù hợp phong cách.
- (−) Việc phải làm: cập nhật WORLD_DESIGN §11, WORLD_RENDERING §8, OPEN_QUESTIONS OQ-16, RISKS RISK-16 (cùng MR với ADR này).

## Phương án đã loại

1. **Mua pack có license thương mại (OQ-16 cũ):** owner chọn hướng MIT; pack trả phí cũng thường kèm EULA cấm phân phối lại asset nguồn.
2. **Cho phép CC-BY / CC-BY-SA:** CC-BY-SA buộc chia sẻ lại tác phẩm phái sinh theo cùng license — trái hướng MIT; CC-BY để mở cho owner quyết định sau.
3. **Cả hai (dùng CC0 trước, chờ pack mua):** owner không chọn.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-24 | Tạo ADR | Owner "làm theo hướng MIT License"; F067 |
| 2026-11-25 | Superseded một phần bởi ADR-0103: câu về phong cách ở mục 7 (stylized low-poly → "Chibi anime cartoon 3D"); quy tắc license giữ nguyên | Owner 2026-11-25 "Chibi anime cartoon 3D"; F068 |
