# Learning Mechanics — Cơ chế học tập nội tại

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: Game Director + EdTech Specialist · Cập nhật: 2026-11-15
> Liên quan: [GAME_DESIGN §6](GAME_DESIGN.md#6-học-tập-lồng-trong-gameplay) · [QUEST_SYSTEM](QUEST_SYSTEM.md) · [CHALLENGE_BOSS_SYSTEM](CHALLENGE_BOSS_SYSTEM.md) · [WORLD_DESIGN](WORLD_DESIGN.md) · [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) · [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) · [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-03 · [ADR-0016](../adr/ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md) · [ADR-0052](../adr/README.md) · [ADR-0073 (Proposed)](../adr/ADR-0073-phat-hanh-theo-pilot-uu-tien-giao-duc.md)

Toàn bộ nội dung tài liệu là **[Confirmed]** cho tới khi Game Director và EdTech Specialist duyệt.

## 1. Mục đích và phạm vi

Tài liệu là nguồn duy nhất cho **cơ chế học tập nội tại** (intrinsic integration): những cơ chế mà **thao tác chơi chính là khái niệm đang học**, thay vì câu hỏi bật lên rồi mới được chơi tiếp. Tài liệu gồm nguyên tắc, catalog cơ chế theo môn, cách gắn cơ chế vào khối xây dựng hiện có (bước quest, mini-game, mô phỏng, boss), tiêu chí review và danh sách ưu tiên cho pilot.

Bối cảnh: review độ phù hợp giáo dục ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-03) nhận xét học tập hiện chủ yếu là "cổng" hoặc "lớp vỏ": mini-game được định nghĩa là lớp trình bày trên câu hỏi ([MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) M-3), bước `collect`/`go_to` không mang giá trị học tập, boss bị trừ năng lượng bằng câu trả lời. Nguyên tắc L-7 của [GAME_DESIGN](GAME_DESIGN.md#6-học-tập-lồng-trong-gameplay) trỏ về tài liệu này.

| Ngoài phạm vi | Tài liệu |
|---|---|
| Định nghĩa loại câu hỏi, chấm điểm, answer key | [QUESTION_BANK](../02-learning/QUESTION_BANK.md), [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| Kiến trúc mini-game client, thêm `minigame_type` | [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) |
| Template mô phỏng và checker Go | [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) |
| Bước quest, party, `interdependent` | [QUEST_SYSTEM](QUEST_SYSTEM.md) |
| Mastery và evidence | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) |

## 2. Nguyên tắc tích hợp nội tại

| # | Nguyên tắc | Câu hỏi kiểm tra |
|---|---|---|
| LM-1 | **Hành động chính = khái niệm học.** Thao tác lặp lại nhiều nhất của người chơi (di chuyển, đặt, ghép, chia, điều chỉnh) biểu diễn trực tiếp khái niệm của LO | Bỏ khái niệm đi thì còn chơi được không? Nếu còn → cơ chế đang là lớp vỏ |
| LM-2 | **Phản hồi nằm trong thế giới.** Kết quả sai hiện ra bằng hệ quả của chính hành động (cầu nghiêng, robot đi sai hướng, cửa không khớp chữ), kèm giải thích ngắn | Học sinh có tự thấy vì sao sai mà không cần đọc chữ "Sai" không? |
| LM-3 | **Server vẫn là trọng tài** (ADR-0016). Client chỉ gửi chuỗi hành động hoặc trạng thái cuối; module learning chấm bằng validator/checker; không gửi đáp án xuống client | Có cách nào client tự kết luận đúng/sai mà không hỏi server không? Nếu có → sai thiết kế |
| LM-4 | **Mỗi lần thao tác hợp lệ là một evidence** gắn LO, như item thông thường; điểm vui của game tách khỏi learning score ([MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) M-5) | Mastery có ghi được từ cơ chế này không? |
| LM-5 | **Tham số hóa được.** Một cơ chế sinh nhiều item từ tham số (số, từ, mạch) để giảm chi phí sản xuất nội dung ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-02) | Một template cho ra ≥ 20 biến thể có ý nghĩa không? |
| LM-6 | **Phù hợp lứa tuổi.** Nhóm `g1_3`: thao tác chạm/kéo lớn, đề bài và hướng dẫn có lồng tiếng, không phụ thuộc đọc nhiều; không thưởng tốc độ cho lớp 1–5 | Học sinh lớp 1 chưa đọc thạo có chơi được không? |
| LM-7 | **Không thay thế đa dạng định dạng.** Cơ chế nội tại bổ sung cho quiz/bài tập, không ép mọi LO thành mini-game | LO này có thật sự hợp với thao tác không, hay một câu hỏi là đủ? |

## 3. Catalog cơ chế theo môn

Mã cơ chế: `LM-<MÔN>-NN`. Khối theo chương trình GDPT 2018; nội dung cụ thể do EdTech chọn theo LO và `yccd_ref` ([CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)).

### 3.1. Toán

| Mã | Cơ chế | Khái niệm | Khối | Phản hồi trong thế giới |
|---|---|---|---|---|
| `LM-MATH-01` | **Trục số để di chuyển**: nhân vật đi trên cây cầu đánh số; mỗi bước chọn "tiến/lùi bao nhiêu" để tới đúng ô đích (ô có rương, cửa) | Cộng, trừ, so sánh số; số âm (lớp 6); phân số trên trục số (lớp 4–6) | 1–7 | Đi quá hoặc thiếu → dừng sai ô, cầu sáng ô đúng để so sánh |
| `LM-MATH-02` | **Cân phương trình để sửa cầu**: cầu treo nghiêng như đĩa cân; đặt/bớt khối ở hai bên để cầu thăng bằng mới đi qua được | Phép cộng cân bằng (lớp 1–3); tìm thành phần chưa biết; phương trình bậc nhất một ẩn (lớp 6–8) | 2–8 | Cầu nghiêng về bên nặng hơn, hiển thị độ lệch |
| `LM-MATH-03` | **Nhân = nhóm vật phẩm khi thu thập**: thu hoạch táo vào giỏ, mỗi giỏ đựng đúng n quả; giao đơn "4 giỏ, mỗi giỏ 6 quả" bằng cách gom nhóm; chia = chia đều vào giỏ | Ý nghĩa phép nhân là cộng các nhóm bằng nhau; phép chia hết/có dư | 2–4 | Giỏ thiếu/thừa quả hiện rõ; NPC đếm lại theo nhóm |
| `LM-MATH-04` | **Chia bánh, đổ nước**: cắt bánh thành phần bằng nhau, đổ nước vào bình chia vạch để đạt đúng phân số | Phân số, phân số bằng nhau, so sánh phân số | 3–6 | Phần bánh không đều; mực nước lệch vạch |
| `LM-MATH-05` | **Dựng hình mở cổng**: xoay, ghép, đo góc để khớp ổ khóa hình học | Hình phẳng, góc, đối xứng, diện tích | 2–9 | Ổ khóa không khớp, chỉ ra cạnh/góc lệch |

### 3.2. Tiếng Việt

| Mã | Cơ chế | Khái niệm | Khối | Phản hồi trong thế giới |
|---|---|---|---|---|
| `LM-VIE-01` | **Ghép âm tiết để mở cửa**: cửa có ổ khóa ba phần; kéo âm đầu + vần + dấu thanh vào ổ, cửa đọc to tiếng vừa ghép (lồng tiếng) | Đánh vần, ghép âm – vần – thanh | 1–2 | Cửa đọc tiếng sai vừa ghép, gợi ý phần cần đổi |
| `LM-VIE-02` | **Xếp toa tàu chữ thành câu**: mỗi toa mang một từ, nối toa theo thứ tự để tàu chạy | Trật tự từ trong câu, dấu câu | 1–5 | Tàu dừng ở toa đặt sai |
| `LM-VIE-03` | **Sửa biển hiệu bị Bóng Ma Chính Tả làm hỏng**: chọn chữ/dấu đúng để biển sáng lại | Chính tả (tr/ch, s/x, dấu hỏi/ngã) | 2–5 | Biển vẫn mờ ở chỗ còn sai |
| `LM-VIE-04` | **Tìm đường bằng câu chỉ dẫn**: đọc (hoặc nghe) chỉ dẫn của NPC rồi đi đúng đường | Đọc hiểu câu, từ chỉ phương hướng | 1–4 | Đi sai ngã rẽ, NPC nhắc lại chỉ dẫn |

### 3.3. Tiếng Anh

| Mã | Cơ chế | Khái niệm | Khối | Phản hồi trong thế giới |
|---|---|---|---|---|
| `LM-ENG-01` | **Nghe và chỉ**: NPC nói tên đồ vật/hành động, học sinh chạm đúng vật trong cảnh | Từ vựng qua nghe | 3–5 | NPC lặp lại từ và chỉ vào vật đúng |
| `LM-ENG-02` | **Ra lệnh cho robot bằng câu**: ghép từ thành câu lệnh ("Go to the red door") để robot làm theo | Trật tự từ, giới từ, mệnh lệnh | 4–9 | Robot làm đúng theo câu đã ghép, kể cả khi câu sai nghĩa |
| `LM-ENG-03` | **Mua bán ở chợ**: chọn câu hỏi/đáp theo tình huống (hỏi giá, đếm tiền) để hoàn thành đơn | Ngôn ngữ chức năng, số đếm | 3–7 | Người bán phản ứng theo câu đã chọn |

### 3.4. Khoa học (Tự nhiên và Xã hội 1–3, Khoa học 4–5, Khoa học tự nhiên 6–9)

| Mã | Cơ chế | Khái niệm | Khối | Phản hồi trong thế giới |
|---|---|---|---|---|
| `LM-SCI-01` | **Nối mạch điện để thắp đèn mở đường hầm** | Mạch kín/hở, vật dẫn điện, nối tiếp/song song | 4–9 | Đèn không sáng, dòng điện dừng ở chỗ hở |
| `LM-SCI-02` | **Gieo hạt và chăm cây**: chọn ánh sáng, nước, đất để cây lớn qua các giai đoạn | Điều kiện sống của thực vật, vòng đời | 1–5 | Cây héo/vàng theo điều kiện thiếu |
| `LM-SCI-03` | **Đưa sinh vật về đúng môi trường để hồi sinh vùng đất** | Phân loại, môi trường sống, chuỗi thức ăn | 1–7 | Vùng đất đổi màu khi đúng; sinh vật tỏ ra khó chịu khi sai |
| `LM-SCI-04` | **Điều chỉnh nhiệt độ để băng tan thành cầu nước rồi đông lại** | Ba thể của nước, sự chuyển thể | 3–6 | Nước chưa đông/đã bay hơi, không đi qua được |
| `LM-SCI-05` | **Đòn bẩy nâng tảng đá** | Đòn bẩy, lực, cánh tay đòn | 6–8 | Tảng đá không nhấc lên, hiển thị chiều mômen |

## 4. Ánh xạ vào khối xây dựng hiện có

Không tạo hệ thống mới; mỗi cơ chế dùng một trong các khối sau.

| Khối | Dùng cho cơ chế | Cách chấm | Cần bổ sung |
|---|---|---|---|
| Bước quest `minigame` ([QUEST_SYSTEM §3](QUEST_SYSTEM.md#3-step-types)) + `minigame_type` hiện có (`drag_and_drop`, `ordering`, `word_game`, `matching`) | `LM-MATH-03`, `LM-VIE-01`, `LM-VIE-02`, `LM-VIE-03`, `LM-ENG-02`, `LM-SCI-03` | Câu hỏi `drag_drop`/`ordering`/`fill_in_blank` ở Question Bank, chấm server | Scene trình bày mới trong cùng `minigame_type` (không đổi schema) |
| Câu hỏi `interactive` (validator kiểm chuỗi hành động, [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) trong bước `minigame` | `LM-MATH-01`, `LM-MATH-02` (lớp 1–5), `LM-MATH-04`, `LM-MATH-05`, `LM-VIE-04`, `LM-ENG-01` | Validator server tính lại từ chuỗi hành động | `minigame_type` mới khi cần trình bày riêng (ví dụ `number_line`) theo checklist [MINIGAME_ENGINE §7](../08-game-client/MINIGAME_ENGINE.md#7-thêm-một-mini-game-mới--checklist) |
| Activity `simulation` + checker Go ([SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md), ADR-0052) | `LM-MATH-02` (lớp 6–8), `LM-SCI-01`, `LM-SCI-02`, `LM-SCI-04`, `LM-SCI-05` | Checker tính lại từ tham số + action log | Template mô phỏng mới (`template_code`) và checker tương ứng |
| Bước `interdependent` (co-op, [QUEST_SYSTEM §3.2](QUEST_SYSTEM.md#32-gán-bước-trong-party-và-bước-interdependent-confirmed)) | Biến thể nhóm của `LM-MATH-02` (mỗi bạn giữ khối lượng một bên cân), `LM-VIE-04` (mỗi bạn giữ một phần chỉ dẫn), `LM-SCI-01` | Mỗi thành viên trả lời câu hỏi cuối trong attempt riêng | Thẻ dữ kiện theo role |
| Phase boss ([CHALLENGE_BOSS_SYSTEM](CHALLENGE_BOSS_SYSTEM.md)) | `LM-MATH-04` cho Rồng Phân Số, `LM-VIE-03` cho Bóng Ma Chính Tả | Như khối gốc của phase | Phase dùng activity cơ chế thay vì chỉ câu hỏi trắc nghiệm |
| Bước khung `go_to`/`collect` | Chỉ dẫn dắt tới nơi diễn ra cơ chế | Không chấm | Không gắn điều kiện học tập vào `collect`; khi việc thu thập mang khái niệm (như `LM-MATH-03`), đổi thành bước `minigame` |

Ví dụ áp dụng: quest mẫu "Phiên chợ của bác Gấu Nâu" ([QUEST_SYSTEM §9](QUEST_SYSTEM.md#9-ví-dụ-hoàn-chỉnh--toán-lớp-3)) có bước `collect` "nhặt 3 giỏ táo" chỉ để trang trí. Theo `LM-MATH-03`, bước này nên thành bước `minigame` "xếp táo vào giỏ theo đơn hàng 4 × 6", nối trực tiếp với bước ghép phép nhân.

## 5. Chấm điểm và evidence

- Mỗi lần nộp của cơ chế là một item đã chấm (attempt `mode = minigame` hoặc `quest`), sinh evidence mastery với trọng số ngữ cảnh do [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) sở hữu. Cờ `rapid_guess` và gợi ý (`hints_used`) áp như mọi item ([ACTIVITY_ENGINE §8.3, §11.1](../02-learning/ACTIVITY_ENGINE.md)).
- Thao tác thử–sai trong lúc chơi (ví dụ đặt khối lên cân rồi bỏ ra) **không** là evidence; chỉ trạng thái nộp cuối (hoặc từng bước "khóa" do template khai báo) được chấm.
- Phản hồi trong thế giới (LM-2) được dựng từ kết quả server trả về sau khi chấm, không suy ra ở client.
- Game score (điểm vui, combo) tách hoàn toàn khỏi learning score; với lớp 1–5 không có thưởng tốc độ.

## 6. Tiêu chí review cơ chế

Một cơ chế (hoặc một quest dùng cơ chế) chỉ được publish khi reviewer học thuật và reviewer game cùng xác nhận:

| # | Tiêu chí | Người kiểm |
|---|---|---|
| RV-1 | Gắn ≥ 1 LO (và `yccd_ref` khi có); thao tác chính biểu diễn đúng khái niệm, không gây hiểu sai (ví dụ trục số chia đều, cân đúng tỉ lệ) | EdTech |
| RV-2 | Đạt LM-1: bỏ khái niệm thì không còn chơi được | Game + EdTech |
| RV-3 | Phản hồi sai nằm trong thế giới và có giải thích ngắn song ngữ | Game |
| RV-4 | Chấm hoàn toàn ở server; payload gửi client không chứa đáp án hay trạng thái đích | Kỹ thuật |
| RV-5 | Nhóm `g1_3`: có lồng tiếng đề bài và hướng dẫn; vùng chạm đủ lớn; không yêu cầu đọc đoạn dài | EdTech + UX |
| RV-6 | Micro loop 10–60 giây, có điểm dừng tự nhiên; không đếm ngược số giây với lớp 1–3 | Game |
| RV-7 | Tham số hóa sinh đủ biến thể; không lặp cùng item trong cùng phiên | EdTech |
| RV-8 | Trợ năng: màu không là tín hiệu duy nhất, có âm thanh + biểu tượng | UX |
| RV-9 | Chạy trong ngân sách hiệu năng máy tầm trung ([PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md)) | Kỹ thuật |

## 7. Ưu tiên cho pilot

Chọn 2–4 cơ chế cho pilot (phạm vi pilot theo [ADR-0073](../adr/ADR-0073-phat-hanh-theo-pilot-uu-tien-giao-duc.md), Proposed). Tiêu chí chọn: dùng khối có sẵn (không cần engine mới), phủ nhóm lớp 1–3 khó nhất, khớp cốt truyện chương 1–2 ([WORLD_DESIGN §2.2](WORLD_DESIGN.md#22-mạch-truyện-chính-confirmed--người-duyệt-2026-11-15-adr-0056)), tham số hóa tốt.

| Thứ tự | Cơ chế | Lý do | Khối kỹ thuật |
|---|---|---|---|
| 1 | `LM-MATH-03` Nhân = nhóm vật phẩm | Khớp quest mẫu Toán lớp 3; chỉ cần scene trình bày mới trên `drag_and_drop` | `minigame` + câu hỏi `drag_drop` |
| 2 | `LM-MATH-02` Cân phương trình sửa cầu (bản lớp 2–5) | Khớp chương 1 "giúp cư dân sửa cầu"; dùng được cho co-op `interdependent` | `minigame` + câu hỏi `interactive` |
| 3 | `LM-VIE-01` Ghép âm tiết mở cửa | Lớp 1–2 chưa đọc thạo là nhóm thiếu thiết kế nhất (PED-07); khớp chương 2 Rừng Ngôn Ngữ | `minigame` (`word_game`/`drag_and_drop`) + lồng tiếng |
| 4 (tùy chọn) | `LM-MATH-01` Trục số để di chuyển | Dùng cho nhiều khối và nhiều chủ đề số | `interactive` + `minigame_type` mới `number_line` |

Đo hiệu quả của các cơ chế pilot theo kế hoạch đo hiệu quả học tập ([ADR-0080](../adr/ADR-0080-do-hieu-qua-hoc-tap-la-exit-criterion-pilot.md), Proposed).

## 8. Câu hỏi mở

- Có thêm `minigame_type` mới (`number_line`) cho pilot hay dùng `interactive` với scene chung? Phụ thuộc năng lực Godot ở Phase pilot [TBD].
- Ngân sách lồng tiếng tiếng Việt cho `LM-VIE-01` (ghi âm sẵn hay TTS phía server) [TBD — xem EDU_FIT_REVIEW A11Y-01].

## 9. Acceptance criteria

| ID | Tiêu chí |
|---|---|
| AC-MG-101 | Mỗi cơ chế pilot có ít nhất một quest đã publish trong đó cơ chế là bước học tập chính, đạt đủ RV-1…RV-9 |
| AC-MG-102 | Payload render của cơ chế (mini-game/mô phỏng) không chứa đáp án hoặc trạng thái đích; client sửa đổi gửi trạng thái "đúng" giả vẫn bị server chấm sai |
| AC-MG-103 | Thao tác thử–sai trước khi nộp không sinh learning event; lần nộp sinh đúng một evidence cho mỗi LO gắn với item |
| AC-MG-104 | Học sinh lớp 1 hoàn thành `LM-VIE-01` chỉ với hướng dẫn lồng tiếng (kiểm thử khả dụng trong pilot) |
