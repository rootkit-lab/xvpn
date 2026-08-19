package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

var errInvalidProfileMedia = errors.New("referência de mídia inválida")

// Paleta do perfil — só tokens do design system (PLAN.md §6.12).
var allowedProfileThemes = map[string]struct{}{
	"primary":     {},
	"safe":        {},
	"xgroup":      {},
	"xdriver":     {},
	"marketplace": {},
	"glow-amber":  {},
	"glow-red":    {},
}

// Tons antigos da capa (`tone:chart-2`) mapeiam para a paleta atual.
var legacyBannerTones = map[string]string{
	"primary":     "primary",
	"chart-2":     "safe",
	"chart-3":     "xgroup",
	"chart-4":     "xdriver",
	"chart-5":     "marketplace",
	"safe":        "safe",
	"xgroup":      "xgroup",
	"xdriver":     "xdriver",
	"marketplace": "marketplace",
	"glow-amber":  "glow-amber",
	"glow-red":    "glow-red",
}

func normalizeTheme(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if mapped, ok := legacyBannerTones[s]; ok {
		return mapped, nil
	}
	if _, ok := allowedProfileThemes[s]; !ok {
		return "", errInvalidProfileMedia
	}
	return s, nil
}

func resolveProfileTheme(p store.SocialProfile) string {
	if t, err := normalizeTheme(p.Theme); err == nil && t != "" {
		return t
	}
	if tone, ok := strings.CutPrefix(p.BannerURL, "tone:"); ok {
		if mapped, err := normalizeTheme(tone); err == nil {
			return mapped
		}
	}
	return ""
}

func normalizeAvatarRef(raw string) (string, error) {
	return normalizeMediaRef(raw, false)
}

func normalizeBannerRef(raw string) (string, error) {
	return normalizeMediaRef(raw, true)
}

func normalizeMediaRef(raw string, allowTone bool) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if allowTone {
		if tone, ok := strings.CutPrefix(s, "tone:"); ok {
			mapped, err := normalizeTheme(tone)
			if err != nil || mapped == "" {
				return "", errInvalidProfileMedia
			}
			return "tone:" + mapped, nil
		}
	}
	if rest, ok := strings.CutPrefix(s, "attachment:"); ok {
		id, err := strconv.ParseUint(rest, 10, 64)
		if err != nil || id == 0 {
			return "", errInvalidProfileMedia
		}
		return fmt.Sprintf("attachment:%d", id), nil
	}
	return "", errInvalidProfileMedia
}

func attachmentIDFromRef(ref string) (uint, bool) {
	rest, ok := strings.CutPrefix(ref, "attachment:")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(rest, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func (a *App) validateOwnedImageRef(userID uint, ref string) error {
	id, ok := attachmentIDFromRef(ref)
	if !ok {
		return nil
	}
	var att store.SocialAttachment
	if err := a.Store.DB.First(&att, id).Error; err != nil {
		return errInvalidProfileMedia
	}
	if att.UploaderID != userID {
		return errInvalidProfileMedia
	}
	if !strings.HasPrefix(att.Mime, "image/") {
		return errInvalidProfileMedia
	}
	return nil
}

func (a *App) isProfileMedia(attachmentID uint) bool {
	ref := fmt.Sprintf("attachment:%d", attachmentID)
	var n int64
	_ = a.Store.DB.Model(&store.SocialProfile{}).
		Where("avatar_url = ? OR banner_url = ?", ref, ref).
		Count(&n).Error
	return n > 0
}
