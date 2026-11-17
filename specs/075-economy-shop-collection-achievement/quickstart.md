# Quickstart: F075 — Kinh tế Xu, Shop cosmetic, inventory, collection, achievement, badge

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F070, F074 (và F031 phát `learning_objective.progressed`).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events`, `make seed-check` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Seed config: `go run ./cmd/worker seed-config --module=game --dir deploy/seed/game-config` (currency `xu`, 10 item, 3 shop item trong đó 1 gán riêng `th-demo-a`, 1 collection knowledge card, 2 achievement có một ẩn).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`), `ad.truong.a`, `sa.admin` (điều chỉnh, MFA tắt ở local) và một tài khoản platform thứ hai có `game.economy.adjust` (`cm.noidung` được gán tạm role này trong realm local — xem task Polish).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Phát `quest.completed` có rule thưởng 20 Xu cho `hs.an.3a`; phát lại | Một bút toán `reward`; `getWallet.balance = 20`; `currency.credited` | F075-AC-01, F075-AC-04 |
| 2 | Phát event đến khi Xu hôm nay chạm cap | Bút toán cuối chỉ phần còn lại; `daily_earn_remaining = 0` | F075-AC-02 |
| 3 | `listShopItems` bằng `hs.an.3a` và `hs.dung.7a` | `hs.dung.7a` không thấy mặt hàng chỉ gán `th-demo-a` | F075-AC-06, F075-AC-12 |
| 4 | `purchaseShopItem` hai lần cùng `Idempotency-Key` | Cùng response, trừ Xu một lần; audit 1 dòng | F075-AC-07, F075-AC-08 |
| 5 | Hai request khác key đồng thời khi chỉ đủ Xu cho một | Một `200`, một `409 INSUFFICIENT_BALANCE`; số dư không âm | F075-AC-09 |
| 6 | Mua lại vật phẩm không xếp chồng đã có; mặt hàng hết thời gian bán | `409 ITEM_ALREADY_OWNED`; `409 SHOP_ITEM_UNAVAILABLE` | F075-AC-10 |
| 7 | Grant cùng item không xếp chồng lần 2 | Một dòng inventory; bút toán Xu `duplicate_conversion` theo độ hiếm | F075-AC-13, F075-AC-14 |
| 8 | `updateAvatar` với item chưa sở hữu / sai slot | `422 ITEM_NOT_OWNED` / `422 EQUIP_SLOT_INVALID` | F075-AC-16 |
| 9 | Phát `learning_objective.progressed` lên `proficient`, rồi xuống `developing` | Knowledge card cấp và còn nguyên | F075-AC-19 |
| 10 | `listAchievements` trước/sau khi mở khóa achievement ẩn; phát trigger lặp | Trước: không có điều kiện; mở khóa một lần | F075-AC-21, F075-AC-22 |
| 11 | Token `ad.truong.a` gọi `getWallet` | `403 GAME_ACCESS_STUDENT_ONLY` | F075-AC-12 |
| 12 | `sa.admin` tạo điều chỉnh −10 Xu cho `hs.an.3a` (`X-Tenant-Id` = `th-demo-a`) rồi tự duyệt | `202 pending_approval`; tự duyệt `403 PERMISSION_DENIED` | F075-AC-25, F075-AC-26 |
| 13 | Tài khoản thứ hai duyệt | Bút toán `adjustment`, `economy.adjusted`, audit | F075-AC-26 |
| 14 | Token `ad.truong.a` gọi `createEconomyAdjustment` | `403 PERMISSION_DENIED`, `authz.denied` | F075-AC-27 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã AC, log job `wallet-reconcile` 0 lệch, kết quả test race 1.000 cặp, trace_id kịch bản 4 và 13.
