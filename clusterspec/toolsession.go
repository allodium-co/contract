package clusterspec

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"regexp"
	"strings"
)

// ToolSessionHostPrefix is the leftmost part of a session's public hostname:
// the data plane's gateway serves a session at ToolSessionHostPrefix+ID+"."+domain.
const ToolSessionHostPrefix = "t-"

const (
	// MinToolSessionIDLen is the length of a NewToolSessionID: 128 random bits
	// in unpadded base32. Anything shorter is too easy to guess.
	MinToolSessionIDLen = 26
	// MaxToolSessionIDLen keeps ToolSessionHostPrefix+ID within the 63-octet
	// limit of one DNS label.
	MaxToolSessionIDLen = 63 - len(ToolSessionHostPrefix)
	// maxOwnerSubLen is the OpenID Connect limit on a subject identifier.
	maxOwnerSubLen = 255
)

var (
	dnsLabelPattern     = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	toolSessionIDCoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)
)

// ToolSessionSpec is the desired state of one user's tool session, such as a
// marimo notebook server. It is compute shape plus the owner's identity and
// nothing else: a session never carries a token, key or other credential, a
// notebook path, a repository URL, an image reference or any notebook content.
// The data plane resolves Tool and Size against its own catalog, which pins
// the image for each tool, so the control plane cannot choose what runs.
type ToolSessionSpec struct {
	// ID names the session and its public hostname. ValidToolSessionID
	// defines the format; NewToolSessionID makes one.
	ID string `json:"id"`
	// OwnerSub is the owning user's subject (sub) at the tenant's identity
	// provider. Only this user may reach the session.
	OwnerSub string `json:"ownerSub"`
	// Tool is a key into the data plane's tool catalog, such as "marimo".
	Tool string `json:"tool"`
	// Size is a key into the sizes the catalog defines for Tool.
	Size string `json:"size"`
	// DesiredPhase is PhaseRunning or PhaseStopped. A stopped session keeps
	// the user's storage and releases its compute.
	DesiredPhase Phase `json:"desiredPhase"`
}

// Validate rejects a session the agent must not build Kubernetes objects from.
// It checks the form of Tool and Size; whether the catalog has them is the
// agent's check.
func (s ToolSessionSpec) Validate() error {
	if !ValidToolSessionID(s.ID) {
		return fmt.Errorf("id %q is not a valid tool session ID", s.ID)
	}
	if err := validateOwnerSub(s.OwnerSub); err != nil {
		return err
	}
	if !validCatalogKey(s.Tool) {
		return fmt.Errorf("tool %q is not a valid catalog key", s.Tool)
	}
	if !validCatalogKey(s.Size) {
		return fmt.Errorf("size %q is not a valid catalog key", s.Size)
	}
	if s.DesiredPhase != PhaseRunning && s.DesiredPhase != PhaseStopped {
		return fmt.Errorf("desiredPhase must be %q or %q, got %q", PhaseRunning, PhaseStopped, s.DesiredPhase)
	}
	return nil
}

// ValidToolSessionID reports whether id can name a tool session: a lowercase
// RFC 1123 label of MinToolSessionIDLen to MaxToolSessionIDLen characters, so
// ToolSessionHostPrefix+id is still one DNS label.
func ValidToolSessionID(id string) bool {
	return len(id) >= MinToolSessionIDLen && len(id) <= MaxToolSessionIDLen && dnsLabelPattern.MatchString(id)
}

// NewToolSessionID returns a fresh, unguessable session ID that
// ValidToolSessionID accepts.
func NewToolSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return toolSessionIDCoding.EncodeToString(b), nil
}

func validateOwnerSub(sub string) error {
	if sub == "" {
		return fmt.Errorf("ownerSub is required")
	}
	if len(sub) > maxOwnerSubLen {
		return fmt.Errorf("ownerSub is longer than %d characters", maxOwnerSubLen)
	}
	if strings.IndexFunc(sub, func(r rune) bool { return r < '!' || r > '~' }) >= 0 {
		return fmt.Errorf("ownerSub must be printable ASCII without spaces")
	}
	return nil
}

func validCatalogKey(key string) bool {
	return len(key) <= 63 && dnsLabelPattern.MatchString(key)
}
