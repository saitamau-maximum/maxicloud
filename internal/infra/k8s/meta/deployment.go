package meta

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkflowMeta struct {
	WorkflowID  string
	AppID       string
	OwnerUserID string
	IsPreview   bool
}

func (m WorkflowMeta) Apply(o *metav1.ObjectMeta) {
	ensureMaps(o)
	o.Labels[LabelWorkflowID] = m.WorkflowID
	o.Labels[LabelAppID] = m.AppID
	o.Labels[LabelPreview] = strconv.FormatBool(m.IsPreview)
	SetOwner(o, m.OwnerUserID)
}

func WorkflowMetaFrom(o metav1.Object) WorkflowMeta {
	l, a := o.GetLabels(), o.GetAnnotations()
	isPreview, _ := strconv.ParseBool(l[LabelPreview])
	return WorkflowMeta{
		WorkflowID:  l[LabelWorkflowID],
		AppID:       l[LabelAppID],
		OwnerUserID: readOwner(l, a),
		IsPreview:   isPreview,
	}
}

func SelectByWorkflowID(id string) client.MatchingLabels {
	return client.MatchingLabels{LabelWorkflowID: id}
}

func SelectWorkflowsByApp(appID string, isPreview bool) client.MatchingLabels {
	return client.MatchingLabels{
		LabelAppID:   appID,
		LabelPreview: strconv.FormatBool(isPreview),
	}
}
