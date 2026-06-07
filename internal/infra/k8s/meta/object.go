package meta

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureMaps(o *metav1.ObjectMeta) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	if o.Annotations == nil {
		o.Annotations = map[string]string{}
	}
}

func SetOwner(o *metav1.ObjectMeta, ownerID string) {
	ensureMaps(o)
	o.Labels[LabelOwnerUserID] = clampLabelValue(ownerID)
	o.Annotations[AnnotationOwnerUserID] = ownerID
}

func ReadOwner(labels, annotations map[string]string) string {
	if v := annotations[AnnotationOwnerUserID]; v != "" {
		return v
	}
	return labels[LabelOwnerUserID]
}

func MarkPreview(o *metav1.ObjectMeta, originalApplicationID string, prNumber int) {
	ensureMaps(o)
	o.Labels[LabelPreview] = "true"
	o.Labels[LabelOriginalAppID] = originalApplicationID
	o.Labels[LabelPRNumber] = strconv.Itoa(prNumber)
}

func SelectPreview(originalApplicationID string, prNumber int) client.MatchingLabels {
	return client.MatchingLabels{
		LabelPreview:       "true",
		LabelOriginalAppID: originalApplicationID,
		LabelPRNumber:      strconv.Itoa(prNumber),
	}
}
