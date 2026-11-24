# ADR-0073 — Phát hành theo pilot ưu tiên giáo dục thay cho "Release 1.0 đủ mọi thứ"

> **Trạng thái:** Accepted · Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (bỏ Windows) · **Ngày:** 2026-11-15 · **OQ:** OQ-08 · **Tài liệu:** [ROADMAP](../12-project/ROADMAP.md) · [PRODUCT_VISION](../00-overview/PRODUCT_VISION.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (bỏ Windows, 2026-11-24).** Không còn hiệu lực: Windows trong nền tảng của pilot (mục quyết định). **Còn hiệu lực:** toàn bộ phần còn lại; pilot chạy trên Web, Android, iOS (PC phòng máy dùng bản Web).

## Bối cảnh

[ROADMAP §1](../12-project/ROADMAP.md) chốt Release 1.0 là game hoàn chỉnh, "không cắt thành MVP": khoảng 40 sprint lịch (~80 tuần) với đội ~12 người, gồm 8 vùng đất, 12 loại quest, 11 mini-game, 8 chế độ thi đấu, 13 leaderboard, 6 định dạng giải, 8 loại channel chat, 4 nền tảng và mục tiêu 10.000 CCU. Vòng lặp nhà trường (M3) chỉ xong khoảng tuần 42, UAT với trường ở Phase 6, đánh giá mastery thật ở M7. Review ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) GD-01, K3) đánh giá phạm vi này không khả thi và bằng chứng với trường đến quá muộn cho một sản phẩm giáo dục bán B2B.

## Quyết định

1. Thay mốc "Release 1.0 hoàn chỉnh" bằng ba giai đoạn:
   - **Pilot ưu tiên giáo dục** (mục tiêu khoảng tuần 30–36): Learning Engine 6–8 loại câu hỏi; LMS nhà trường (giao bài, School Learning Dashboard, heatmap LO); Toán + Tiếng Việt lớp 1–5; quest `main/learning/daily/weekly` + boss solo; 4–5 mini-game, trong đó ít nhất 2 cơ chế "kỹ năng là cách chơi" ([LEARNING_MECHANICS](../01-game-design/LEARNING_MECHANICS.md)); XP, level, shop cosmetic; một kiểu co-op phụ thuộc lẫn nhau; chat chỉ quick message + emote; nền tảng theo ADR-0013 (Windows, Android, iOS, Web); nghiên cứu hiệu quả theo ADR-0080.
   - **Mở rộng sau pilot:** 1v1 + bot, Team Battle, Glicko-2, leaderboard cấp trường, chat party/match có kiểm duyệt, thêm môn/khối.
   - **Hoàn thiện:** leaderboard Global, các định dạng giải còn lại, đủ 8 channel + DM, club, season/event, chứng minh 10.000 CCU. Cả ba giai đoạn đều chạy trên đủ 4 nền tảng của ADR-0013.
2. Mỗi giai đoạn có exit criteria riêng trong ROADMAP; pilot bắt buộc có kết quả nghiên cứu hiệu quả (ADR-0080) trước khi mở rộng.
3. Không bỏ yêu cầu nào của Master Prompt; chỉ đổi thứ tự và điểm phát hành.

## Hệ quả

(+) Có sản phẩm chạy tại trường sớm khoảng 1 năm; điều chỉnh thiết kế theo dữ liệu thật; giảm rủi ro giao hàng. (−) Phải viết lại ROADMAP §1–§3, PRODUCT_VISION §8; stakeholder phải chấp nhận thay quyết định "không MVP" đã [Confirmed].

## Phương án đã loại

Giữ "Release 1.0 đủ mọi thứ": rủi ro trễ hạn và thiếu bằng chứng học tập cao (RISK-05).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Accepted; nền tảng theo ADR-0013 (Windows, Android, iOS, Web) vì ADR-0079 bị từ chối | Technical Architect |
| 2026-11-24 | Superseded một phần bởi ADR-0101: pilot chỉ Web, Android, iOS | Owner dự án |
