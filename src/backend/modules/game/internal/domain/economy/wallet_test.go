package economy

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// F075-AC-01, AC-03: credits add, a debit larger than the balance is refused and leaves it unchanged, no zero posting.
func TestWalletNeverNegative(t *testing.T) {
	w := Wallet{Currency: Xu}
	if b, err := w.Credit(20); err != nil || b != 20 {
		t.Fatalf("credit: %d %v", b, err)
	}
	if b, err := w.Debit(25); !errors.Is(err, ErrInsufficientBalance) || b != 20 || w.Balance != 20 {
		t.Fatalf("overdraft: %d %v", b, err)
	}
	if b, err := w.Debit(20); err != nil || b != 0 {
		t.Fatalf("debit all: %d %v", b, err)
	}
	for _, n := range []int64{0, -5} {
		if _, err := w.Credit(n); !errors.Is(err, ErrInvalidAmount) {
			t.Fatalf("credit %d: %v", n, err)
		}
		if _, err := w.Debit(n); !errors.Is(err, ErrInvalidAmount) {
			t.Fatalf("debit %d: %v", n, err)
		}
	}
	if _, err := w.Apply(-1); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("negative adjustment on 0: %v", err)
	}
	if b, err := w.Apply(7); err != nil || b != 7 {
		t.Fatalf("positive adjustment: %d %v", b, err)
	}
}

// F075-AC-02: only the remaining allowance of the day is credited; nothing once the cap is reached.
func TestEarnableUnderDailyCap(t *testing.T) {
	cases := []struct {
		amount, earned, cap, want int
		capped                    bool
	}{
		{200, 0, 300, 200, false},
		{200, 200, 300, 100, true},
		{50, 300, 300, 0, true},
		{50, 320, 300, 0, true},
		{0, 0, 300, 0, false},
	}
	for _, c := range cases {
		got, capped := Earnable(c.amount, c.earned, c.cap)
		if got != c.want || capped != c.capped {
			t.Fatalf("Earnable(%d, %d, %d) = %d %v", c.amount, c.earned, c.cap, got, capped)
		}
	}
	if Remaining(320, 300) != 0 || Remaining(120, 300) != 180 {
		t.Fatal("Remaining")
	}
}

// F075-AC-14: duplicate_item_xu covers every rarity.
func TestDuplicateTable(t *testing.T) {
	d := DuplicateTable{"common": 5, "uncommon": 10, "rare": 25, "epic": 50, "legendary": 100}
	if err := d.Validate(); err != nil || d.Conversion("rare") != 25 || d.Conversion("mythic") != 0 {
		t.Fatalf("table: %v", err)
	}
	delete(d, "epic")
	if d.Validate() == nil {
		t.Fatal("missing rarity accepted")
	}
}

func listing() Listing {
	return Listing{ShopItemID: uuid.New(), ItemID: "hat_star_01", ItemKind: "accessory", Price: 120, Currency: Xu, PurchaseLimit: 1,
		Assigned: true}
}

// F075-AC-06, AC-10: sale window, tenant assignment, grade, event/season listing only while it runs.
func TestListingAvailability(t *testing.T) {
	now := time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)
	l := listing()
	if !l.Available(3, now, false) {
		t.Fatal("plain listing unavailable")
	}
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	for name, mut := range map[string]func(*Listing){
		"not assigned": func(l *Listing) { l.Assigned = false },
		"not yet":      func(l *Listing) { l.From = &future },
		"ended":        func(l *Listing) { l.To = &past },
		"ends now":     func(l *Listing) { l.To = &now },
		"other grade":  func(l *Listing) { l.Grades = []int{6, 7} },
		"event":        func(l *Listing) { id := uuid.New(); l.EventID = &id },
		"season":       func(l *Listing) { id := uuid.New(); l.SeasonID = &id },
		"real money":   func(l *Listing) { l.Currency = "vnd" },
	} {
		c := l
		mut(&c)
		if c.Available(3, now, false) {
			t.Fatalf("%s: available", name)
		}
	}
	c := l
	id := uuid.New()
	c.EventID = &id
	if !c.Available(3, now, true) {
		t.Fatal("event listing while the event runs")
	}
	c = l
	c.Grades = []int{3}
	if !c.Available(3, now, false) {
		t.Fatal("grade listing")
	}
}

// F075-AC-10: price changed, owned non-stackable, limit over every version, stack limit, quantity of a non-stackable.
func TestPurchaseCheck(t *testing.T) {
	now := time.Now()
	l := listing()
	if total, err := l.Check(Buyer{Grade: 3}, 1, 120, 1, now); err != nil || total != 120 {
		t.Fatalf("buy: %d %v", total, err)
	}
	cases := map[string]struct {
		b        Buyer
		qty, exp int
		want     error
	}{
		"price changed": {Buyer{Grade: 3}, 1, 100, ErrPriceChanged},
		"owned":         {Buyer{Grade: 3, Owned: 1}, 1, 120, ErrAlreadyOwned},
		"limit":         {Buyer{Grade: 3, Purchased: 1}, 1, 120, ErrLimitReached},
		"quantity 2":    {Buyer{Grade: 3}, 2, 120, ErrQuantityInvalid},
		"quantity 0":    {Buyer{Grade: 3}, 0, 120, ErrQuantityInvalid},
	}
	for name, c := range cases {
		if _, err := l.Check(c.b, c.qty, c.exp, 1, now); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	s := l
	s.ItemKind, s.Stackable, s.MaxStack, s.PurchaseLimit, s.Price = "consumable_cosmetic", true, 10, 5, 10
	if total, err := s.Check(Buyer{Grade: 3, Owned: 2, Purchased: 2}, 3, 10, 1, now); err != nil || total != 30 {
		t.Fatalf("stack: %d %v", total, err)
	}
	if _, err := s.Check(Buyer{Grade: 3, Owned: 2, Purchased: 3}, 3, 10, 1, now); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("stack limit: %v", err)
	}
	if _, err := s.Check(Buyer{Grade: 3, Owned: 9}, 2, 10, 1, now); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("max_stack: %v", err)
	}
	u := l
	u.Assigned = false
	if _, err := u.Check(Buyer{Grade: 3}, 1, 120, 1, now); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unassigned: %v", err)
	}
	if (Listing{}).Limit(1) != 1 || (Listing{PurchaseLimit: 3}).Limit(1) != 3 {
		t.Fatal("limit default")
	}
}

// F075-AC-11: real money, a non-cosmetic kind, a kind other than the item's, a tradable item are refused.
func TestContentValidation(t *testing.T) {
	item := Item{ItemID: "hat_star_01", Kind: "accessory", Rarity: "rare", EquipSlot: "head", MaxStack: 1, Name: Name{"Mũ", "Hat"}}
	if err := item.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(*Item){
		"kind":     func(i *Item) { i.Kind = "xp_booster" },
		"slot":     func(i *Item) { i.EquipSlot = "pet" },
		"tradable": func(i *Item) { i.Tradable = true },
		"stack":    func(i *Item) { i.MaxStack = 5 },
		"name":     func(i *Item) { i.Name.EN = "" },
		"rarity":   func(i *Item) { i.Rarity = "mythic" },
	}
	for name, mut := range bad {
		c := item
		mut(&c)
		var fe *FieldError
		if err := c.Validate(); !errors.As(err, &fe) || !errors.Is(err, ErrContentInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	s := ShopItem{Listing: listing()}
	if err := s.Validate(&item); err != nil {
		t.Fatal(err)
	}
	shop := map[string]func(*ShopItem){
		"vnd":       func(s *ShopItem) { s.Currency = "vnd" },
		"free":      func(s *ShopItem) { s.Price = 0 },
		"badge":     func(s *ShopItem) { s.ItemKind = "badge" },
		"kind diff": func(s *ShopItem) { s.ItemKind = "hair" },
		"limit 2":   func(s *ShopItem) { s.PurchaseLimit = 2 },
		"window": func(s *ShopItem) {
			a, b := time.Now(), time.Now().Add(-time.Hour)
			s.From, s.To = &a, &b
		},
	}
	for name, mut := range shop {
		c := s
		mut(&c)
		if err := c.Validate(&item); !errors.Is(err, ErrContentInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := s.Validate(nil); !errors.Is(err, ErrContentInvalid) {
		t.Fatalf("unknown item: %v", err)
	}
}

// F075-AC-25, AC-26: the request rules and four-eyes.
func TestAdjustment(t *testing.T) {
	xu, item := Xu, "hat_star_01"
	creator := uuid.New()
	a := Adjustment{ID: uuid.New(), StudentID: uuid.New(), Kind: AdjustCurrency, Currency: &xu, Delta: -50, Reason: "Sửa lỗi cộng trùng",
		Status: StatusPending, CreatedBy: creator}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Adjustment{
		{Kind: "money", Delta: 1, Reason: "lý do đủ dài"},
		{Kind: AdjustCurrency, Currency: &xu, Delta: 0, Reason: "lý do đủ dài"},
		{Kind: AdjustCurrency, Currency: &xu, Delta: 5, Reason: "ngắn"},
		{Kind: AdjustCurrency, Delta: 5, Reason: "lý do đủ dài"},
		{Kind: AdjustItem, Delta: 1, Reason: "lý do đủ dài"},
		{Kind: AdjustXP, ItemID: &item, Delta: 1, Reason: "lý do đủ dài"},
	}
	for i, b := range bad {
		if err := b.Validate(); !errors.Is(err, ErrAdjustInvalid) {
			t.Fatalf("bad %d: %v", i, err)
		}
	}
	if err := a.Approve(creator); !errors.Is(err, ErrSelfApproval) || a.Status != StatusPending {
		t.Fatalf("self approval: %v", err)
	}
	approver := uuid.New()
	if err := a.Approve(approver); err != nil || a.Status != StatusApplied || *a.ApprovedBy != approver {
		t.Fatalf("approve: %v", err)
	}
	if err := a.Approve(uuid.New()); !errors.Is(err, ErrNotPending) {
		t.Fatalf("approve twice: %v", err)
	}
	if err := a.Reject(uuid.New()); !errors.Is(err, ErrNotPending) {
		t.Fatalf("reject applied: %v", err)
	}
	r := Adjustment{Status: StatusPending, CreatedBy: creator}
	if err := r.Reject(creator); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("self reject: %v", err)
	}
	if err := r.Reject(approver); err != nil || r.Status != StatusRejected {
		t.Fatalf("reject: %v", err)
	}
}
