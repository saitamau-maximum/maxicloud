package meta

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	maxLabelLen     = 63
	namespacePrefix = "maxicloud-"
)

var (
	nonLabelChar     = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	edgeNonAlnum     = regexp.MustCompile(`^[^A-Za-z0-9]+|[^A-Za-z0-9]+$`)
	nonNamespaceChar = regexp.MustCompile(`[^a-z0-9-]+`)
	multiHyphen      = regexp.MustCompile(`-+`)
	edgeHyphen       = regexp.MustCompile(`^-+|-+$`)
)

func ProjectNamespace(projectName string, stableKey string) string {
	const suffixHexBytes = 4

	normalized := normalizeNamespaceBase(projectName, "project")
	suffix := hashSuffix(stableKey, suffixHexBytes)
	maxBaseLen := maxLabelLen - len(namespacePrefix) - len(suffix)
	normalized = truncateNormalizedBase(normalized, maxBaseLen, "project", func(s string) string {
		return edgeHyphen.ReplaceAllString(s, "")
	})

	return namespacePrefix + normalized + suffix
}

func TruncateLabelValue(raw string) string {
	if len(raw) <= maxLabelLen {
		return raw
	}
	return raw[:maxLabelLen]
}

// Kubernetes Label では 英数字、ドット、アンダースコア、ハイフン以外は使用できないため、ブランチ名を正規化する。
func NormalizeBranchForLabel(branch string) string {
	const hashBytes = 4

	normalized := normalizeLabelBase(branch, "branch")
	suffix := hashSuffix(strings.TrimSpace(branch), hashBytes)
	maxBaseLen := maxLabelLen - len(suffix)
	normalized = truncateNormalizedBase(normalized, maxBaseLen, "branch", func(s string) string {
		return edgeNonAlnum.ReplaceAllString(s, "")
	})

	return normalized + suffix
}

func normalizeNamespaceBase(raw string, fallback string) string {
	base := strings.ToLower(strings.TrimSpace(raw))
	base = nonNamespaceChar.ReplaceAllString(base, "-")
	base = multiHyphen.ReplaceAllString(base, "-")
	base = edgeHyphen.ReplaceAllString(base, "")
	if base == "" {
		return fallback
	}
	return base
}

func normalizeLabelBase(raw string, fallback string) string {
	base := strings.TrimSpace(raw)
	base = nonLabelChar.ReplaceAllString(base, "-")
	base = edgeNonAlnum.ReplaceAllString(base, "")
	if base == "" {
		return fallback
	}
	return base
}

func hashSuffix(raw string, bytes int) string {
	sum := sha1.Sum([]byte(raw))
	return fmt.Sprintf("-%x", sum[:bytes])
}

func truncateNormalizedBase(base string, maxBaseLen int, fallback string, trim func(string) string) string {
	maxBaseLen = max(maxBaseLen, 1)
	if len(base) <= maxBaseLen {
		return base
	}

	base = base[:maxBaseLen]
	base = trim(base)
	if base == "" {
		return fallback
	}
	return base
}

func SetOwner(o *metav1.ObjectMeta, ownerID string) {
	ensureMaps(o)
	o.Labels[LabelOwnerUserID] = TruncateLabelValue(ownerID)
	o.Annotations[AnnotationOwnerUserID] = ownerID
}

func MarkPreview(o *metav1.ObjectMeta, originalApplicationID string) {
	ensureMaps(o)
	o.Labels[LabelPreview] = "true"
	o.Annotations[AnnotationOriginalAppID] = originalApplicationID
}

func readOwner(labels, annotations map[string]string) string {
	if v := annotations[AnnotationOwnerUserID]; v != "" {
		return v
	}
	return labels[LabelOwnerUserID]
}

func ensureMaps(o *metav1.ObjectMeta) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	if o.Annotations == nil {
		o.Annotations = map[string]string{}
	}
}
