package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

var errInvalidProfileMedia = errors.New("referência de mídia inválida")

// Tons da capa — mesmos tokens do design system (não cores soltas).
var allowedBannerTones = map[string]struct{}{
	"primary": {},
	"chart-2": {},
	"chart-3": {},
	"chart-4": {},
	"chart-5": {},
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
			if _, allowed := allowedBannerTones[tone]; !allowed {
				return "", errInvalidProfileMedia
			}
			return "tone:" + tone, nil
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
