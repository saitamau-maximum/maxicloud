package k8s

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"
)

var nonLabelChar = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
var edgeNonAlnum = regexp.MustCompile(`^[^A-Za-z0-9]+|[^A-Za-z0-9]+$`)

func truncateLabelValue(raw string) string {
	const maxLabelLen = 63
	if len(raw) <= maxLabelLen {
		return raw
	}
	return raw[:maxLabelLen]
}

// Labelには/を使用することができないため正規化する
func normalizeBranchForLabel(branch string) string {
	const (
		maxLabelLen = 63
		hashBytes   = 4
	)

	raw := strings.TrimSpace(branch)
	normalized := nonLabelChar.ReplaceAllString(raw, "-")
	normalized = edgeNonAlnum.ReplaceAllString(normalized, "")
	if normalized == "" {
		normalized = "branch"
	}

	sum := sha1.Sum([]byte(raw))
	suffix := fmt.Sprintf("-%x", sum[:hashBytes])
	maxBaseLen := maxLabelLen - len(suffix)
	maxBaseLen = max(maxBaseLen, 1)

	if len(normalized) > maxBaseLen {
		normalized = normalized[:maxBaseLen]
		normalized = edgeNonAlnum.ReplaceAllString(normalized, "")
	}

	return normalized + suffix
}
