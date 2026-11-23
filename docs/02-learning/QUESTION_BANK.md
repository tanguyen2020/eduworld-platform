# Question Bank

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: EdTech Specialist + Senior Golang Developer · Cập nhật: 2026-11-16
> Liên quan: [CONTENT_MODEL](CONTENT_MODEL.md) · [ACTIVITY_ENGINE](ACTIVITY_ENGINE.md) · [EXAM_ENGINE](EXAM_ENGINE.md) · [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) · [ADR-0016](../adr/ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md) · Schema: [`question.schema.json`](../../specs/schemas/question.schema.json), [`question-set.schema.json`](../../specs/schemas/question-set.schema.json)

## 1. Nguyên tắc

1. **[Confirmed]** Ngân hàng câu hỏi tập trung; mini-game, quest, battle, exercise, exam **đều** lấy câu hỏi từ đây — không có ngân hàng riêng.
2. **[Confirmed]** Client không quyết định đáp án đúng hay điểm cuối (ADR-0016).
3. Mỗi `question_version` tách thành ba phần lưu riêng cột:
   - `render_payload` — gửi cho client (không chứa thông tin suy ra được đáp án).
   - `answer_key` — chỉ server đọc.
   - `scoring_rule` — chỉ server đọc.
   - `explanation` và phản hồi theo phương án (`options[].feedback`) — gửi cho client **sau khi** chấm, theo `FeedbackTiming` ([ACTIVITY_ENGINE §8](ACTIVITY_ENGINE.md#8-phản-hồi-trong-game)).
   - `hints[]` — gửi từng cấp khi học sinh xin gợi ý ([ACTIVITY_ENGINE §8.3](ACTIVITY_ENGINE.md#83-gợi-ý-trong-attempt-confirmed)).
4. Module `question_bank` cung cấp port cho module khác:

```go
// internal/question_bank/api.go
type Delivery interface {
    Select(ctx context.Context, scope TenantScope, rule SelectionRule, seed int64) ([]QuestionVersionRef, error)
    Render(ctx context.Context, refs []QuestionVersionRef, opts RenderOptions) ([]RenderedQuestion, error) // shuffle, không answer_key
    Grade(ctx context.Context, ref QuestionVersionRef, presentation Presentation, response json.RawMessage) (GradeResult, error)
    Explanation(ctx context.Context, ref QuestionVersionRef, locale string) (RichContent, error)
}
```

## 2. Cấu trúc một Question Version

| Field | Gửi client | Mô tả |
|---|---|---|
| `question_version_id`, `question_id`, `version` | ✔ (`question_version_id`) | |
| `type` | ✔ | `QuestionType`, 11 dạng §3 |
| `stem` | ✔ | rich content (CONTENT_MODEL §9); `stem.audio_ref` bắt buộc với nhóm `g1_3` ([CONTENT_MODEL §8.1](CONTENT_MODEL.md#81-audio-đọc-đề-và-hướng-dẫn-cho-nhóm-g1_3-confirmed)) |
| `render_payload` | ✔ | dữ liệu hiển thị theo dạng |
| `answer_key` | ✘ | đáp án |
| `scoring_rule` | ✘ | cách tính điểm |
| `options[].feedback` | sau chấm | Phản hồi riêng cho từng phương án (i18n, ≤ 500 ký tự) [Confirmed]: thuộc **phần đáp án**, bị loại khỏi DTO render như `options[].misconception_tag`; chỉ trả trong `ActivityResponseResult.feedback` ([ACTIVITY_ENGINE §8.2](ACTIVITY_ENGINE.md#82-phản-hồi-theo-phương-án-nhiễu-confirmed)). Áp dụng cho dạng có `options` (`single_choice`, `multiple_choice`); dạng khác dùng `explanation` |
| `explanation` | sau chấm | lời giải |
| `hints[]` | từng cấp, khi xin | Gợi ý nhiều cấp (≤ 5): cấp 1 định hướng ("Nhớ lại bảng nhân 7"), cấp 2 bước trung gian ("7 × 8 = 7 × 7 + 7"), cấp cuối gần lời giải nhưng không nêu đáp án. Phục vụ qua `hints:next` ([ACTIVITY_ENGINE §8.3](ACTIVITY_ENGINE.md#83-gợi-ý-trong-attempt-confirmed)); không có trong render payload ban đầu. Khuyến nghị ≥ 2 cấp cho câu dùng trong practice/quest của `g1_3`, `g4_5` [Confirmed] |
| `worked_example_ref?` | ✔ | Object `{content_item_id, block_id}`: `content_item_id` là lesson hoặc learning material, `block_id` là `id` của khối `worked_example` trong version hiện hành của nội dung đó, có ít nhất một LO chung với câu hỏi ([CONTENT_MODEL §9.2](CONTENT_MODEL.md#92-khối-worked_example-confirmed)); tham chiếu không tồn tại → `422`. Client hiện nút "Xem ví dụ mẫu" sau khi trả lời sai [Confirmed] |
| `min_response_ms?` | ✘ | Ngưỡng thời gian tối thiểu để không bị coi là đoán bừa; không khai báo thì dùng mặc định theo loại câu ([ACTIVITY_ENGINE §11.1](ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-confirmed)) [Confirmed] |
| `max_score` | ✔ | điểm tối đa (mặc định 1) |
| meta: `difficulty`, `grade_levels`, `subject_id`, `topic_id`, `objective_ids`, `status`, `author_id`, `reviewer_id`, `published_at`, `content_language` | một phần | client chỉ nhận `difficulty`, `grade_levels`, `subject_id` |

`presentation` — khi giao câu cho một attempt, server lưu cách đã hiển thị (thứ tự lựa chọn sau shuffle, giá trị tham số) trong `attempt_items.presentation` để chấm đúng (vì `option_id` ổn định, shuffle không ảnh hưởng chấm; nhưng câu tham số hóa cần biết giá trị đã sinh).

Option/item ID là chuỗi ngắn ngẫu nhiên ổn định trong version (vd. `o_7f3a`), **không** dùng `A/B/C/D` hay chỉ số để tránh lộ thứ tự gốc.

## 3. Mười một dạng câu hỏi

`QuestionType` chuẩn (dùng thống nhất trong mọi spec, event và tài liệu): `single_choice`, `multiple_choice`, `true_false`, `fill_in_blank`, `short_answer`, `numeric`, `matching`, `ordering`, `drag_drop`, `image_based`, `interactive`. Mọi dạng đều do server tự chấm.

Mặc định `scoring_rule.mode`: `all_or_nothing` (AON) hoặc `partial`. `max_score` mặc định 1.

### 3.1. `single_choice` — Một lựa chọn đúng

```json
{
  "type": "single_choice",
  "stem": { "blocks": [{ "type": "paragraph", "text": "7 × 8 = ?" }] },
  "render_payload": {
    "options": [
      { "id": "o_a91c", "content": { "blocks": [{ "type": "paragraph", "text": "54" }] }, "feedback": "54 là 6 × 9. Em thử đếm thêm 7 từ 7 × 7 = 49 nhé." },
      { "id": "o_3b2e", "content": { "blocks": [{ "type": "paragraph", "text": "56" }] }, "feedback": "Đúng rồi: 7 × 8 = 7 × 7 + 7 = 56." },
      { "id": "o_c7d0", "content": { "blocks": [{ "type": "paragraph", "text": "63" }] }, "feedback": "63 là 7 × 9, nhiều hơn một lần 7.", "misconception_tag": "off_by_one_factor" },
      { "id": "o_19fe", "content": { "blocks": [{ "type": "paragraph", "text": "48" }] }, "feedback": "48 là 6 × 8. Em xem lại thừa số thứ nhất nhé." }
    ],
    "shuffle_options": true
  },
  "answer_key": { "correct_option_id": "o_3b2e" },
  "scoring_rule": { "mode": "all_or_nothing" },
  "hints": [
    { "blocks": [{ "type": "paragraph", "text": "Em đã thuộc 7 × 7 chưa?" }] },
    { "blocks": [{ "type": "paragraph", "text": "7 × 8 = 7 × 7 + 7" }] }
  ]
}
```

`feedback` và `misconception_tag` của option được server tách khỏi DTO render; client chỉ nhận `id` và `content`. Response client: `{ "option_id": "o_3b2e" }`. Chấm: so khớp id; kết quả chấm trả `feedback` của phương án đã chọn khi `FeedbackTiming` cho phép.

### 3.2. `multiple_choice` — Nhiều lựa chọn đúng

```json
{
  "type": "multiple_choice",
  "render_payload": {
    "options": [
      { "id": "o_1", "content": { "blocks": [{ "type": "paragraph", "text": "12" }] } },
      { "id": "o_2", "content": { "blocks": [{ "type": "paragraph", "text": "15" }] } },
      { "id": "o_3", "content": { "blocks": [{ "type": "paragraph", "text": "18" }] } },
      { "id": "o_4", "content": { "blocks": [{ "type": "paragraph", "text": "20" }] } }
    ],
    "shuffle_options": true,
    "show_correct_count": false
  },
  "answer_key": { "correct_option_ids": ["o_1", "o_3"] },
  "scoring_rule": { "mode": "partial", "partial_method": "right_minus_wrong", "min_score": 0 }
}
```

Response: `{ "option_ids": ["o_1", "o_3"] }`.

| `partial_method` | Công thức (C = số đáp án đúng, c = số chọn đúng, w = số chọn sai) |
|---|---|
| `all_or_nothing` | `max_score` nếu tập chọn = tập đúng, ngược lại 0 |
| `right_minus_wrong` [mặc định] | `max(0, (c − w) / C) × max_score` |
| `proportional` | `c / C × max_score` nếu `w = 0`, ngược lại 0 |

`show_correct_count=true` cho lớp 1–3 ("Chọn 2 đáp án") [Confirmed].

### 3.3. `true_false`

```json
{
  "type": "true_false",
  "render_payload": { "labels": { "true": "Đúng", "false": "Sai" } },
  "answer_key": { "value": false },
  "scoring_rule": { "mode": "all_or_nothing" }
}
```

Biến thể "nhiều mệnh đề Đúng/Sai" (dạng đề THCS) dùng `statements[]`:

```json
{
  "render_payload": { "statements": [ { "id": "s_1", "content": {"blocks":[{"type":"paragraph","text":"Số 0 là số chẵn"}]} }, { "id": "s_2", "content": {"blocks":[{"type":"paragraph","text":"Số 1 là số nguyên tố"}]} } ] },
  "answer_key": { "statement_values": { "s_1": true, "s_2": false } },
  "scoring_rule": { "mode": "partial", "partial_method": "proportional" }
}
```

### 3.4. `fill_in_blank` — Điền vào chỗ trống

```json
{
  "type": "fill_in_blank",
  "stem": { "blocks": [{ "type": "paragraph", "text": "Thủ đô của Việt Nam là {{blank:b_1}}. Sông dài nhất chảy qua thủ đô là sông {{blank:b_2}}." }] },
  "render_payload": {
    "blanks": [
      { "id": "b_1", "input": "text", "max_length": 30 },
      { "id": "b_2", "input": "word_bank" }
    ],
    "word_bank": ["Hồng", "Mekong", "Đà"]
  },
  "answer_key": {
    "blanks": {
      "b_1": { "accepted": ["Hà Nội"], "match": "normalized" },
      "b_2": { "accepted": ["Hồng"], "match": "exact" }
    }
  },
  "scoring_rule": { "mode": "partial", "partial_method": "proportional" }
}
```

Response: `{ "blanks": { "b_1": "ha noi", "b_2": "Hồng" } }`.

### 3.5. `short_answer` — Trả lời ngắn

```json
{
  "type": "short_answer",
  "render_payload": { "max_length": 50, "keyboard": "text" },
  "answer_key": {
    "accepted": ["con mèo", "mèo"],
    "match": "normalized",
    "allow_diacritics_missing": false,
    "fuzzy": { "enabled": true, "max_edit_distance": 1, "min_length": 5 }
  },
  "scoring_rule": { "mode": "all_or_nothing" }
}
```

`short_answer` chỉ dùng cho câu có tập đáp án đóng, chấm tự động. Câu tự luận mở cần chấm tay **không thuộc 11 loại câu hỏi của Master Prompt §4.3** và nằm ngoài Release ([ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md)); thêm lại phải qua ADR mới.

### 3.6. `numeric` — Đáp số

```json
{
  "type": "numeric",
  "render_payload": { "keyboard": "numeric", "unit_label": "cm", "allow_fraction_input": false, "decimal_separator": "," },
  "answer_key": { "value": 12.5, "tolerance": { "type": "absolute", "value": 0.01 }, "accept_equivalent_forms": true },
  "scoring_rule": { "mode": "all_or_nothing" }
}
```

Response: `{ "value": "12,5" }` (chuỗi; server chấp nhận cả `,` và `.` làm dấu thập phân, khoảng trắng hoặc dấu ngăn nghìn theo `content_language` của câu hỏi — `vi`: `.` ngăn nghìn, `en`: `,` ngăn nghìn; giá trị mơ hồ như `1.234` hiểu theo `content_language`).

| `tolerance.type` | Đúng khi |
|---|---|
| `exact` | `|x − v| = 0` (sau khi parse về decimal, không float) |
| `absolute` | `|x − v| ≤ t` |
| `relative` | `|x − v| ≤ t × |v|` |

`accept_equivalent_forms`: chấp nhận `25/2`, `12,50`. Câu tham số hóa: `value` là biểu thức `"{{a}} * {{b}}"` tính trên `presentation.params`.

#### 3.6.1. Câu hỏi có công thức (ADR-0055)

Công thức Toán, Lý, Hóa **chỉ để hiển thị** trong game: trong đề, lựa chọn, giải thích, cặp ghép, thẻ kéo thả. **Học sinh không nhập công thức trong game** [Confirmed ADR-0055 mục 5–6].

- **Lưu trữ, soạn, render SVG, kiểm tra phía server, giới hạn:** quy định duy nhất ở [CONTENT_MODEL §9.1](CONTENT_MODEL.md#91-công-thức-toán-lý-hóa-adr-0055). Ví dụ mhchem trong câu hỏi: `\ce{2H2 + O2 -> 2H2O}`, `\pu{9.8 m/s^2}`.
- **Dạng trả lời dùng được với công thức** (không cần gõ biểu thức):

| Dạng | Ví dụ |
|---|---|
| `single_choice` / `multiple_choice` | Chọn biểu thức đúng trong 4 lựa chọn đã render: `\frac{11}{2}`, `\frac{2}{11}`… |
| `true_false` | "\(\ce{H2O2}\) là công thức của nước" — Đúng/Sai |
| `numeric` | Đề có công thức, học sinh nhập **số** (`5,5`), có dung sai và đơn vị |
| `fill_in_blank` | Điền **số hoặc từ** vào chỗ trống trong công thức đã render |
| `matching` | Ghép phương trình hóa học với tên phản ứng |
| `ordering` | Sắp xếp các bước biến đổi biểu thức |
| `drag_drop` | Kéo hệ số vào phương trình cân bằng (thẻ là số/công thức đã render) |

- [Confirmed] Câu hỏi bắt học sinh **gõ biểu thức toán** (đáp án là biểu thức) không có trong game Release 1.0. Loại đánh giá này dùng hình thức khác (chọn đáp án, nhập số, điền số/từ, sắp xếp bước biến đổi).
- **An toàn:** LaTeX là dữ liệu không tin cậy: allowlist lệnh/extension (`ams`, `mhchem`, `color` giới hạn), chặn `\href`, `\url`, `\html*`, `\class`, `\style`, `\require`, `\unicode`, macro tự định nghĩa; giới hạn độ dài 2.000 và độ sâu lồng 20 [Confirmed]. SVG luôn được sanitize phía server (chỉ phần tử/thuộc tính SVG an toàn, không `<script>`, không thuộc tính sự kiện, không tham chiếu ngoài), ≤ 256 KB [Confirmed]. Thiếu SVG, SVG không hợp lệ hoặc LaTeX ngoài allowlist → `422 MATH_RENDER_FAILED`.

### 3.7. `matching` — Nối cặp

```json
{
  "type": "matching",
  "render_payload": {
    "left":  [ { "id": "l_1", "content": {"blocks":[{"type":"paragraph","text":"cat"}]} }, { "id": "l_2", "content": {"blocks":[{"type":"paragraph","text":"dog"}]} } ],
    "right": [ { "id": "r_1", "content": {"blocks":[{"type":"paragraph","text":"con chó"}]} }, { "id": "r_2", "content": {"blocks":[{"type":"paragraph","text":"con mèo"}]} }, { "id": "r_3", "content": {"blocks":[{"type":"paragraph","text":"con gà"}]} } ],
    "shuffle": true
  },
  "answer_key": { "pairs": [["l_1", "r_2"], ["l_2", "r_1"]] },
  "scoring_rule": { "mode": "partial", "partial_method": "proportional" }
}
```

Response: `{ "pairs": [["l_1","r_2"],["l_2","r_3"]] }`. Điểm = số cặp đúng / số cặp trong key. Có thể có "mồi nhiễu" ở cột phải (`r_3`).

### 3.8. `ordering` — Sắp xếp thứ tự

```json
{
  "type": "ordering",
  "render_payload": {
    "items": [ { "id": "i_1", "content": {"blocks":[{"type":"paragraph","text":"Gieo hạt"}]} }, { "id": "i_2", "content": {"blocks":[{"type":"paragraph","text":"Nảy mầm"}]} }, { "id": "i_3", "content": {"blocks":[{"type":"paragraph","text":"Ra hoa"}]} }, { "id": "i_4", "content": {"blocks":[{"type":"paragraph","text":"Kết trái"}]} } ],
    "shuffle": true
  },
  "answer_key": { "order": ["i_1", "i_2", "i_3", "i_4"] },
  "scoring_rule": { "mode": "partial", "partial_method": "adjacent_pairs" }
}
```

| `partial_method` | Công thức (n = số item) |
|---|---|
| `all_or_nothing` | đúng toàn bộ |
| `position` | số item đúng vị trí / n |
| `adjacent_pairs` [mặc định] | số cặp kề đúng thứ tự / (n − 1) — công bằng hơn khi lệch một item |

Server đảm bảo thứ tự hiển thị sau shuffle **khác** thứ tự đúng.

### 3.9. `drag_drop` — Kéo thả vào vùng

```json
{
  "type": "drag_drop",
  "render_payload": {
    "background_media_ref": null,
    "draggables": [ { "id": "d_1", "content": {"blocks":[{"type":"paragraph","text":"2"}]} }, { "id": "d_2", "content": {"blocks":[{"type":"paragraph","text":"9"}]} }, { "id": "d_3", "content": {"blocks":[{"type":"paragraph","text":"14"}]} } ],
    "zones": [ { "id": "z_even", "label": "Số chẵn", "capacity": null }, { "id": "z_odd", "label": "Số lẻ", "capacity": null } ],
    "reusable_draggables": false
  },
  "answer_key": { "placements": { "d_1": "z_even", "d_2": "z_odd", "d_3": "z_even" } },
  "scoring_rule": { "mode": "partial", "partial_method": "proportional" }
}
```

Response: `{ "placements": { "d_1": "z_even", "d_2": "z_even", "d_3": "z_even" } }`. Điểm = số draggable đặt đúng / tổng draggable trong key; draggable "mồi" không nằm trong key phải ở ngoài mọi zone (nếu đặt vào → tính sai).

### 3.10. `image_based` — Câu hỏi dựa trên hình (hotspot / gắn nhãn)

```json
{
  "type": "image_based",
  "render_payload": {
    "interaction": "hotspot",
    "image_media_ref": "01JD8...",
    "image_size": { "w": 1024, "h": 768 },
    "max_selections": 1
  },
  "answer_key": {
    "regions": [ { "id": "h_heart", "shape": "circle", "cx": 512, "cy": 300, "r": 60 } ],
    "correct_region_ids": ["h_heart"]
  },
  "scoring_rule": { "mode": "all_or_nothing" }
}
```

Response: `{ "points": [ { "x": 505, "y": 310 } ] }` (tọa độ theo `image_size` gốc). Server kiểm tra điểm thuộc vùng (`circle`, `rect`, `polygon`). Tọa độ vùng **không** gửi xuống client. Biến thể `interaction: "label"` = drag_drop với zone đặt trên ảnh (zone gửi xuống client vì chúng là vị trí thả, đáp án là ánh xạ).

Các dạng khác (single_choice...) cũng có thể có hình trong `stem`/option — không cần type riêng.

### 3.11. `interactive` — Câu hỏi tương tác / mô phỏng

Godot widget hoặc mô phỏng (Science Simulation) chạy phía client; client gửi **trạng thái cuối/hành động**, server kiểm chứng bằng validator khai báo.

```json
{
  "type": "interactive",
  "render_payload": {
    "widget": "balance_scale",
    "widget_version": "1.0",
    "config": { "left_weights": [5, 3], "available_weights": [1, 2, 4, 8], "max_moves": 6 }
  },
  "answer_key": {
    "validator": "balance_scale.equal",
    "params": { "target": 8 }
  },
  "scoring_rule": { "mode": "all_or_nothing", "require_action_log": true }
}
```

Response: `{ "final_state": { "right_weights": [8] }, "action_log": [ {"t": 1200, "op": "place", "w": 8} ] }`.

Quy tắc:
- Mỗi `widget` có **validator Go thuần** đăng ký trong registry (`question_bank/validators`). Không chạy script do người soạn viết trên server.
- Server replay `action_log` nếu `require_action_log` (đảm bảo trạng thái cuối đạt được hợp lệ, chống gửi thẳng đáp án).
- Danh sách widget là cấu hình phiên bản hóa; widget mới cần cả Godot scene + validator → release kỹ thuật, không chỉ CMS.

## 4. Chuẩn hóa văn bản tiếng Việt (match = `normalized`)

> Câu hỏi `content_language = en` (môn Tiếng Anh) chuẩn hóa theo tiếng Anh: không bỏ dấu, chuẩn hóa dấu nháy/khoảng trắng, không phân biệt hoa thường trừ khi câu hỏi yêu cầu. Bản dịch tùy chọn của câu hỏi có đáp án chữ (`fill_in_blank`, `short_answer`) phải khai báo đáp án chấp nhận **riêng cho từng ngôn ngữ**; thiếu thì câu hỏi chỉ phát ở ngôn ngữ gốc ([CONTENT_MODEL §8](CONTENT_MODEL.md#8-ngôn-ngữ-nội-dung-và-bản-dịch-adr-0056)).

Áp dụng cho `fill_in_blank`, `short_answer`:

1. Unicode NFC.
2. Trim, gộp khoảng trắng liên tiếp.
3. Lowercase (theo locale vi).
4. Bỏ dấu câu ở đầu/cuối (`.,!?;:"'`).
5. Chuẩn hóa vị trí dấu thanh kiểu cũ/mới (`hoà` ≡ `hòa`, `thuỷ` ≡ `thủy`) bằng bảng ánh xạ.
6. Nếu `allow_diacritics_missing=true`: so sánh thêm sau khi bỏ dấu (`ha noi` ≡ `Hà Nội`) — **mặc định false** cho môn Tiếng Việt (chính tả là mục tiêu học), true cho môn khác [Confirmed].
7. Fuzzy (tùy chọn): Levenshtein ≤ `max_edit_distance` (1–2) khi chuỗi đã chuẩn hóa ≥ `min_length` (≥ 3); không áp dụng cho môn Tiếng Việt/Tiếng Anh chính tả — lưu câu thuộc môn `vietnamese`/`english` có `fuzzy.enabled = true` bị từ chối `422 VALIDATION_FAILED`, `errors[].code = FUZZY_NOT_ALLOWED_FOR_SUBJECT` (F023).
8. Số trong chuỗi: `"5"` ≡ `"năm"` chỉ khi khai báo trong `accepted`.

`match = exact`: chỉ bước 1–2. Các bước chuẩn hóa được unit test bằng bộ test-case tiếng Việt (≥ 200 cặp) [Confirmed].

## 5. Câu hỏi tham số hóa [Confirmed]

`render_payload.params` định nghĩa biến ngẫu nhiên; server sinh giá trị theo `seed` của attempt, lưu vào `attempt_items.presentation.params`:

```json
{
  "params": {
    "a": { "type": "int", "min": 6, "max": 9 },
    "b": { "type": "int", "min": 2, "max": 9 },
    "constraints": ["a != b"]
  }
}
```

Stem `"{{a}} × {{b}} = ?"`, `answer_key.value = "{{a}} * {{b}}"`. Server render stem đã thay giá trị trước khi gửi client. Biểu thức chỉ dùng evaluator an toàn (cộng/trừ/nhân/chia/mũ/so sánh), không eval tổng quát.

Evaluator (F023): số, biến đã khai báo, `+ − × ÷ ^` (cả `* / :`), so sánh `== != < <= > >=` (`=` ≡ `==`), `&& || !` (`and or not`), ngoặc; mọi định danh khác và lời gọi hàm bị từ chối khi lưu; số học chính xác (phân số), biểu thức ≤ 255 ký tự, độ sâu ≤ 32, số mũ nguyên |e| ≤ 10. Chỉ `single_choice` và `numeric` có `params`; tối đa 10 biến (`int`, `decimal` cần `step`, `choice` giá trị số ≤ 20), 10 ràng buộc. Giá trị đã sinh lưu ở `presentation.params` (chuỗi thập phân) và hiển thị theo `content_language` (`vi`: dấu phẩy thập phân).

Sinh giá trị bằng rejection sampling: rút ngẫu nhiên theo `seed` tới khi bộ giá trị thỏa mọi `constraints` (phép chia cho 0 coi là không thỏa), tối đa `param_sampling_max_tries` = 1.000 lần [Confirmed] (phạm vi `code`). Khi lưu câu hỏi, server chạy cùng giới hạn để kiểm khả thi; không tìm được bộ giá trị nào → `422 VALIDATION_FAILED`.

## 6. Chọn câu ngẫu nhiên theo điều kiện

`SelectionRule` (cũng là phần `selection_rules` trong QuestionSet):

```json
{
  "filters": {
    "subject_id": "…",
    "grade_levels": [3],
    "topic_ids": ["…"],
    "objective_ids": ["…"],
    "difficulty": { "min": 2, "max": 3 },
    "types": ["single_choice", "numeric"],
    "tags_any": ["bang_nhan"],
    "exclude_question_ids": []
  },
  "count": 10,
  "distribution": [
    { "difficulty": 2, "count": 6 },
    { "difficulty": 3, "count": 4 }
  ],
  "avoid_recent": { "enabled": true, "window_days": 7 },
  "prefer_weak_objectives": false,
  "prefer_siblings_of": [],
  "interleave": { "enabled": false, "max_consecutive_same_topic": 2 }
}
```

Thuật toán [Confirmed]:

1. Lọc ứng viên: version `published`, scope khả dụng cho học sinh, khớp filter.
2. Loại câu học sinh đã gặp trong `window_days` (từ `attempt_items`), nếu còn đủ ứng viên.
3. Chia theo `distribution` (hoặc theo objective nếu `per_objective`), chọn ngẫu nhiên có trọng số bằng PRNG seed = `attempt.seed` → tái lập được.
4. Không đủ câu: nới lỏng theo thứ tự `avoid_recent` → difficulty ±1; vẫn thiếu → lỗi `QUESTION_POOL_INSUFFICIENT` (CMS cảnh báo khi pool < 3 × count). Challenge theo mức khó (mode `challenge`, F076) chọn **nghiêm ngặt**: chỉ nới `avoid_recent`, không bao giờ nới dải độ khó hay loại câu (không phát câu ngoài dải của mức) [Confirmed].
5. `prefer_weak_objectives=true` (retry/practice thích ứng): trọng số ∝ `1 − mastery` của LO.
6. **Câu anh em** — `prefer_siblings_of: [question_id…]` (mode `retry_wrong`) [Confirmed]: với mỗi câu đã sai, chọn một câu **anh em** = cùng ít nhất một `objective_id`, cùng `type`, cùng `difficulty`, khác `question_id`, chưa gặp trong `window_days`. Ưu tiên câu anh em có `p_value` gần câu gốc nhất. Không có câu anh em → nới `difficulty` ±1 → cuối cùng mới dùng version hiện hành của chính `question_id` đó (mastery giảm trọng số bằng `w_rep`, [LEARNING_ANALYTICS §3.1](LEARNING_ANALYTICS.md#31-định-nghĩa)). Câu tham số hóa (§5) tự sinh giá trị mới nên được coi là câu anh em của chính nó khi tham số khác lần trước.
7. **Luyện trộn chủ đề (interleaving)** — `interleave.enabled=true` với `filters.topic_ids` gồm ≥ 2 topic [Confirmed]: chia `count` cho các topic theo `interleave.mix` (tỉ lệ mặc định ở [LEARNING_EXPERIENCE §5.2](LEARNING_EXPERIENCE.md#52-luyện-trộn-chủ-đề-interleaving-confirmed); không khai báo thì chia đều), rồi sắp thứ tự sao cho không quá `max_consecutive_same_topic` câu liên tiếp cùng topic. Dùng cho gợi ý "Luyện trộn" ([LEARNING_ANALYTICS §6](LEARNING_ANALYTICS.md#6-recommendation-rules)) và Practice Zone.

Truy vấn ứng viên dùng index `(subject_id, grade_level, topic_id, difficulty) WHERE status='published'` và cache danh sách ID theo filter trong Valkey 5 phút (`ew:learning:qb:pool:<tenant_id>:<filter_hash>`), xóa khi có `content.published`/`content.rolled_back`/`content.unpublished` của câu hỏi liên quan. Pool chỉ chứa version là **con trỏ hiện hành** của mỗi câu (không chọn version cũ đã publish nhưng không còn là con trỏ). Render payload từng version lấy từ snapshot bất biến `ew:learning:cv:question:<id>:<v>`; `answer_key` cache riêng phía server (`ew:learning:key:<question_version_id>`, không bao giờ trả client, không lên CDN) — ADR-0043.

## 7. Tạo đề từ Question Bank

- **Thủ công:** người biên tập nội dung hoặc `academic_manager` chọn câu cụ thể → QuestionSet `kind=static`.
- **Theo blueprint:** AssessmentBlueprint định nghĩa sections × objectives × difficulty → sinh `ExamPaper` (xem [EXAM_ENGINE §4](EXAM_ENGINE.md#4-sinh-đề)).
- **Mẫu đề (variant):** sinh N biến thể đề tương đương cho một exam session để giảm chép bài.

## 8. Chống lộ đáp án

| Nguy cơ | Biện pháp |
|---|---|
| Client đọc đáp án từ payload | `answer_key`, `scoring_rule`, vùng hotspot không bao giờ serialize ra DTO client; test contract kiểm tra DTO không chứa các field này |
| Đoán đáp án qua thứ tự/ID option | Option ID ngẫu nhiên; shuffle server-side; option đúng không luôn ở cùng vị trí trong dữ liệu gốc |
| Lộ explanation, phản hồi theo phương án hoặc gợi ý trước khi nộp | `explanation`, `options[].feedback` chỉ trả về theo `FeedbackTiming`; `hints[]` chỉ trả từng cấp qua `hints:next` khi mode cho phép ([ACTIVITY_ENGINE §8](ACTIVITY_ENGINE.md#8-phản-hồi-trong-game)) |
| Dò đáp án bằng cách thử liên tục (practice immediate feedback) | Mỗi item chỉ nhận số lần trả lời theo policy (mặc định 1 lần ghi điểm, cho phép thử lại không tính điểm tối đa 2 lần); rate limit `answers` 2 req/s/attempt |
| Ghi nhận câu hỏi từ exam rò rỉ sang practice | Câu gắn `exam_only=true` chỉ dùng trong exam; không xuất hiện ở practice/minigame/battle |
| Tải hàng loạt question bank qua API | Không có endpoint liệt kê render payload cho học sinh ngoài attempt; CMS API yêu cầu role biên tập |
| Battle: đối thủ nhìn đáp án qua mạng | Server chỉ phát kết quả câu sau khi khóa câu trả lời của mọi người chơi |

## 9. Thống kê chất lượng câu hỏi (item analysis) [Confirmed]

Worker tính hằng đêm cho question version có ≥ `item_analysis_min_responses` = 30 lần trả lời đầu (sau khi loại `rapid_guess`) [Confirmed], chỉ dùng **lần trả lời đầu tiên có tính điểm** và **loại** lần trả lời gắn `rapid_guess` ([ACTIVITY_ENGINE §11.1](ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-confirmed)), tách riêng lần trả lời có dùng gợi ý:

- `p_value` (tỷ lệ đúng) — so với `difficulty` khai báo; lệch lớn → cờ "cần hiệu chỉnh độ khó".
- `discrimination` (point-biserial với tổng điểm attempt).
- `distractor_rate` cho từng option sai; option không ai chọn → cờ; option sai được chọn nhiều → gợi ý viết `options[].feedback` cho phương án đó.
- `avg_time_ms`, `rapid_guess_rate`, `hint_usage_rate`.

Kết quả lưu `content.question_stats` và hiển thị trong CMS; **không** tự động sửa version đã publish.

### 9.1. Hiệu chỉnh độ khó bán tự động [Confirmed]

Độ khó do tác giả khai báo được hiệu chỉnh theo dữ liệu thật, **luôn có người duyệt**:

| `p_value` (≥ 100 lần trả lời đầu, đã loại `rapid_guess`, cùng khối lớp) | Độ khó đề xuất |
|---|---|
| ≥ 0,85 | 1 |
| 0,70 – < 0,85 | 2 |
| 0,50 – < 0,70 | 3 |
| 0,30 – < 0,50 | 4 |
| < 0,30 | 5 |

Ngưỡng ≥ 100 lượt và bảng ánh xạ trên (`difficulty_calibration`): [Confirmed — Technical Architect 2026-11-15].

1. Hằng tuần worker so `difficulty` khai báo với độ khó đề xuất; lệch ≥ 1 mức → tạo **đề xuất hiệu chỉnh** trong hàng đợi CMS (kèm `p_value`, số lượt, phân bố theo khối).
2. Người duyệt nội dung (`content_reviewer`/`tenant_content_reviewer` theo `owner_scope`) chấp nhận hoặc bỏ qua. Chấp nhận → hệ thống tạo **version mới** chỉ đổi `difficulty` (`change_note = "difficulty_calibration"`), đi qua publish như thường (ADR-0015, R11); không sửa version đã publish.
3. Ngược lại, khi câu chưa đủ dữ liệu, `p̂` dùng cho sinh đề ([EXAM_ENGINE §4.1](EXAM_ENGINE.md#41-cân-bằng-độ-khó-giữa-các-biến-thể-confirmed)) suy từ `difficulty` theo giá trị giữa khoảng của bảng trên (1 → 0,90; 2 → 0,77; 3 → 0,60; 4 → 0,40; 5 → 0,20).
4. Câu bị bỏ qua đề xuất 2 lần liên tiếp không được đề xuất lại trong 90 ngày.

## 10. Yêu cầu chức năng

Mã FR, nhãn và phase do [PRODUCT_REQUIREMENTS §2.6](../00-overview/PRODUCT_REQUIREMENTS.md#26-question-bank--qb-43) cấp (ADR-0082); bảng dưới chỉ chỉ ra phần chi tiết trong tài liệu này.

| Yêu cầu (mã PRD) | Chi tiết trong tài liệu này |
|---|---|
| FR-QB-001 | Mười một dạng câu hỏi (§3) |
| FR-QB-002 | Cấu trúc một Question Version (§2) |
| FR-QB-003 | Chống lộ đáp án ra client (§8) |
| FR-QB-004 | Chọn câu ngẫu nhiên theo điều kiện (§6) |
| FR-QB-005 | Tạo đề từ Question Bank (§7) |
| FR-QB-006 | So khớp chuẩn hóa tiếng Việt cho `short_answer`/`fill_in_blank` (§4), dung sai `numeric` (§3.6) |
| FR-QB-007 | Nội dung tiếng Việt có dấu, công thức, hình ảnh, audio (§2, §3) |
| FR-QB-008 | Câu hỏi tham số hóa (§5) |
| FR-QB-009 | Interactive validator registry (§3.11) |
| FR-QB-010 | Item analysis (§9) |
| FR-QB-011 | Import câu hỏi từ Excel/CSV/QTI-lite có validate; import Word/MathType: [CONTENT_PRODUCTION_PLAN](CONTENT_PRODUCTION_PLAN.md). Cột CSV/Excel: [questions-row.schema.json](../../specs/schemas/import/questions-row.schema.json), mẫu [questions.template.csv](../../specs/examples/import/questions.template.csv). **QTI-lite** (F024): gói zip QTI 2.1 gồm các `assessmentItem` + `metadata.csv` (cột `identifier` và cột mã curriculum như CSV); `choiceInteraction` → `single_choice` (`maxChoices = 1`) / `multiple_choice`, `orderInteraction` → `ordering`, `textEntryInteraction` → `fill_in_blank`; tương tác khác bị từ chối theo item (`QTI_NOT_SUPPORTED`); `responseProcessing` không bao giờ được chạy |
| FR-QB-013 | Phản hồi theo phương án `options[].feedback` (phía đáp án, chỉ trả sau khi chấm); gợi ý nhiều cấp `hints[]` (FR-EXR-012); tham chiếu `worked_example` (FR-CNT-020) (§2, §3.1) |
| FR-QB-014 | Hiệu chỉnh độ khó bán tự động từ `p_value`, có người duyệt (§9.1) |
| FR-QB-015 | Partial credit theo `scoring_rule.mode` và `partial_method` (§2, §3) |
| FR-EXR-013, FR-EXR-014 | Chọn câu anh em cho `retry_wrong`; luyện trộn chủ đề (§6) |

## 11. Kiểm thử

- Grader của từng dạng: bảng test (response → điểm) gồm edge case (rỗng, id lạ, trùng lặp, response sai kiểu → `INVALID_RESPONSE`, không 500).
- Property test: shuffle không ảnh hưởng điểm.
- Contract test: DTO render không chứa `answer_key`/`scoring_rule`/`regions`/`options[].feedback`/`options[].misconception_tag`/`hints`/`min_response_ms`.
- Selection câu anh em: câu đã sai có câu anh em published → không chọn lại `question_id` cũ; không có → fallback đúng thứ tự §6 bước 6.
- Interleaving: 10 câu từ 2 topic với `max_consecutive_same_topic = 2` → không có 3 câu liên tiếp cùng topic.
- Hiệu chỉnh độ khó: chấp nhận đề xuất tạo version mới chỉ khác `difficulty`; version cũ không đổi.
- Công thức (ADR-0055): test chặn LaTeX ngoài allowlist (`\href`, `\require`, macro) và SVG chứa `<script>`/thuộc tính sự kiện → `422 MATH_RENDER_FAILED`; contract test render payload dùng `math_ref` SVG thay cho LaTeX; visual regression bộ công thức mẫu Toán/Lý/Hóa trên portal và Godot.
- Quiz và bài tập tương tác là `activity_type` `quiz` / `interactive_exercise` trong [Activity Registry](ACTIVITY_ENGINE.md); câu hỏi trong tài liệu này là thành phần nội dung của các activity đó.
- Selection: cùng seed → cùng tập câu; pool thiếu → lỗi đúng mã.
