package unit

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/installer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/release"
)

func kit(id string, bands []string, released time.Time) installer.Kit {
	return installer.Kit{KitID: id, Platform: "android", ClientVersion: "0.4.0", GradeBands: bands,
		URL: "https://play.example.test/releases/school-kit/0.4.0/" + id + ".apk", SHA256: strings.Repeat("a", 64), SizeBytes: 1000,
		MinOS: "Android 8.0", ReleasedAt: released}
}

// F134-AC-13: grade level → band (GLOSSARY), kits whose bands meet the school's, the latest per platform × band set,
// every kit for a school without an active student.
func TestInstallerForSchool(t *testing.T) {
	for l, b := range map[int]string{1: "g1_3", 3: "g1_3", 4: "g4_5", 5: "g4_5", 6: "g6_7", 7: "g6_7", 8: "g8_9", 9: "g8_9", 0: "", 10: ""} {
		if got := installer.GradeBandOf(l); got != b {
			t.Errorf("grade %d → %q, want %q", l, got, b)
		}
	}
	if got := installer.BandsOf([]int{5, 1, 3, 4, 12}); strings.Join(got, ",") != "g1_3,g4_5" {
		t.Fatalf("bands %v", got)
	}
	d := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	kits := []installer.Kit{
		kit("g13-old", []string{"g1_3"}, d), kit("g13-new", []string{"g1_3"}, d.AddDate(0, 0, 7)),
		kit("g45", []string{"g4_5"}, d), kit("g69", []string{"g6_7", "g8_9"}, d), kit("g69-new", []string{"g8_9", "g6_7"}, d.AddDate(0, 0, 1)),
	}
	got := installer.ForSchool(kits, []string{"g1_3", "g4_5"})
	if len(got) != 2 || got[0].KitID != "g13-new" || got[1].KitID != "g45" {
		t.Fatalf("primary school kits %+v", got)
	}
	if got := installer.ForSchool(kits, []string{"g8_9"}); len(got) != 1 || got[0].KitID != "g69-new" {
		t.Fatalf("lower secondary kits %+v", got)
	}
	if got := installer.ForSchool(kits, nil); len(got) != 3 {
		t.Fatalf("school without students: %d kits, want 3 (latest of each set)", len(got))
	}
}

func signingKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048) // TEST KEY generated per run, never a release key
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return k, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

func sign(t *testing.T, k *rsa.PrivateKey, body []byte) []byte {
	t.Helper()
	sum := sha256.Sum256(body)
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return []byte(base64.StdEncoding.EncodeToString(sig))
}

func indexJSON(t *testing.T, mutate func(m map[string]any)) []byte {
	t.Helper()
	ix := installer.Index{SchemaVersion: 1, Channel: "production", GeneratedAt: time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC),
		Kits: []installer.Kit{kit("android-g1-3", []string{"g1_3"}, time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC))}}
	b, _ := json.Marshal(ix)
	if mutate == nil {
		return b
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	mutate(m)
	b, _ = json.Marshal(m)
	return b
}

// F134-AC-14, AC-16: only an index with a valid RSA-SHA256 signature and schema is accepted — a changed byte, another
// key, an unknown field, a Windows kit, a non-https URL or a field over its bound is refused.
func TestInstallerIndexVerification(t *testing.T) {
	k, pub := signingKey(t)
	key, err := release.ParsePublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	body := indexJSON(t, nil)
	ix, err := release.Verify(key, body, sign(t, k, body), "production")
	if err != nil || len(ix.Kits) != 1 {
		t.Fatalf("valid index: %v", err)
	}
	tampered := append([]byte(nil), body...)
	tampered[len(tampered)-3] ^= 1
	if _, err := release.Verify(key, tampered, sign(t, k, body), "production"); !errors.Is(err, release.ErrSignature) {
		t.Fatalf("one byte changed: %v", err)
	}
	other, _ := signingKey(t)
	if _, err := release.Verify(key, body, sign(t, other, body), "production"); !errors.Is(err, release.ErrSignature) {
		t.Fatalf("other key: %v", err)
	}
	if _, err := release.Verify(key, body, []byte("%%%"), "production"); !errors.Is(err, release.ErrSignature) {
		t.Fatalf("garbage signature: %v", err)
	}
	if _, err := release.Verify(key, body, sign(t, k, body), "staging"); !errors.Is(err, installer.ErrInvalidIndex) {
		t.Fatalf("wrong channel: %v", err)
	}
	kitField := func(f string, v any) func(m map[string]any) {
		return func(m map[string]any) { m["kits"].([]any)[0].(map[string]any)[f] = v }
	}
	for name, mut := range map[string]func(m map[string]any){
		"unknown field":   func(m map[string]any) { m["tenant_id"] = "x" },
		"windows kit":     kitField("platform", "windows"),
		"http url":        kitField("url", "http://play.example.test/a.apk"),
		"url too long":    kitField("url", "https://play.example.test/"+strings.Repeat("a", 2048)),
		"bad sha256":      kitField("sha256", "ABC"),
		"bad band":        kitField("grade_bands", []any{"g10_12"}),
		"version too big": kitField("client_version", strings.Repeat("1", 33)),
		"schema v2":       func(m map[string]any) { m["schema_version"] = 2 },
	} {
		b := indexJSON(t, mut)
		if _, err := release.Verify(key, b, sign(t, k, b), "production"); !errors.Is(err, installer.ErrInvalidIndex) {
			t.Errorf("%s: %v, want ErrInvalidIndex", name, err)
		}
	}
	small, _ := rsa.GenerateKey(rand.Reader, 1024)
	der, _ := x509.MarshalPKIXPublicKey(&small.PublicKey)
	if _, err := release.ParsePublicKey(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})); err == nil {
		t.Fatal("RSA-1024 release key accepted")
	}
}
