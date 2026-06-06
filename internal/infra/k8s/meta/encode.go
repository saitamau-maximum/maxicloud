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
	nonLabelChar = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	edgeNonAlnum = regexp.MustCompile(`^[^A-Za-z0-9]+|[^A-Za-z0-9]+$`)
)

func ProjectNamespace(projectID string) string {
	return namespacePrefix + projectID
}

func ProjectIDFromNamespace(namespace string) string {
	return strings.TrimPrefix(namespace, namespacePrefix)
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

func SetOwner(o *metav1.ObjectMeta, ownerID string) {
	ensureMaps(o)
	o.Labels[LabelOwnerUserID] = TruncateLabelValue(ownerID)
	o.Annotations[AnnotationOwnerUserID] = ownerID
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
