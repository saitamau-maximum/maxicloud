package meta

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployRunMeta struct {
	DeployRunID string
	AppID       string
	OwnerUserID string
	IsPreview   bool
}

func (m DeployRunMeta) Apply(o *metav1.ObjectMeta) {
	ensureMaps(o)
	o.Labels[LabelDeployRunID] = m.DeployRunID
	o.Labels[LabelAppID] = m.AppID
	o.Labels[LabelPreview] = strconv.FormatBool(m.IsPreview)
	SetOwner(o, m.OwnerUserID)
}

func DeployRunMetaFrom(o metav1.Object) DeployRunMeta {
	l, a := o.GetLabels(), o.GetAnnotations()
	isPreview, _ := strconv.ParseBool(l[LabelPreview])
	return DeployRunMeta{
		DeployRunID: l[LabelDeployRunID],
		AppID:       l[LabelAppID],
		OwnerUserID: readOwner(l, a),
		IsPreview:   isPreview,
	}
}

func SelectByDeployRunID(id string) client.MatchingLabels {
	return client.MatchingLabels{LabelDeployRunID: id}
}

func SelectDeployRunsByApp(appID string, isPreview bool) client.MatchingLabels {
	return client.MatchingLabels{
		LabelAppID:   appID,
		LabelPreview: strconv.FormatBool(isPreview),
	}
}
