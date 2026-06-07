package meta

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func MarkPreview(o *metav1.ObjectMeta, originalApplicationID string) {
	ensureMaps(o)
	o.Labels[LabelPreview] = "true"
	o.Annotations[AnnotationOriginalAppID] = originalApplicationID
}
