package config

import (
	"fmt"
	"net/url"

	pkgconfig "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Installer is the configuration of the school installer page (F134 US3, R3): the release CDN base URL
// (EW_CLIENT_RELEASES_BASE_URL, e.g. https://play.<domain>/releases), the release public key (EW_CLIENT_RELEASE_PUBKEY or
// EW_CLIENT_RELEASE_PUBKEY_FILE, PEM) and the Web client link for school PCs (EW_GAME_WEB_URL, ADR-0101 §8). Without a base
// URL the page answers 503 UNAVAILABLE (never a hardcoded URL).
type Installer struct {
	ReleasesBaseURL string
	PublicKeyPEM    []byte
	WebClientURL    string
}

// InstallerFrom reads the installer configuration; a base URL without a public key is a start-up error.
func InstallerFrom(c *pkgconfig.Config) (Installer, error) {
	out := Installer{ReleasesBaseURL: c.String("EW_CLIENT_RELEASES_BASE_URL"), WebClientURL: c.String("EW_GAME_WEB_URL")}
	if k := c.String("EW_CLIENT_RELEASE_PUBKEY"); k != "" {
		out.PublicKeyPEM = []byte(k)
	}
	if out.ReleasesBaseURL != "" && len(out.PublicKeyPEM) == 0 {
		return out, fmt.Errorf("game: EW_CLIENT_RELEASES_BASE_URL needs EW_CLIENT_RELEASE_PUBKEY_FILE (release public key)")
	}
	if out.WebClientURL != "" {
		u, err := url.Parse(out.WebClientURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || len(out.WebClientURL) > 2048 {
			return out, fmt.Errorf("game: EW_GAME_WEB_URL must be an absolute URL")
		}
	}
	return out, nil
}
