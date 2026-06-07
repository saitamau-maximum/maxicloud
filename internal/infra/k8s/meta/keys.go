package meta

const (
	labelPrefix      = "maxicloud.maximum.vc/"
	annotationPrefix = "maxicloud.maximum.vc/"
)

const (
	LabelAppID           = labelPrefix + "app-id"
	LabelAppName         = labelPrefix + "app-name"
	LabelOwnerUserID     = labelPrefix + "owner-user-id"
	LabelSourceRepoOwner = labelPrefix + "source-repo-owner"
	LabelSourceRepoName  = labelPrefix + "source-repo-name"
	LabelSourceBranch    = labelPrefix + "source-branch"
	LabelDeployRunID     = labelPrefix + "deploy-run-id"
	LabelPreview         = labelPrefix + "preview"
	LabelProject         = labelPrefix + "project"
	LabelProjectID       = labelPrefix + "project-id"
	LabelProjectName     = labelPrefix + "project-name"
)

const (
	AnnotationSourceBranch       = annotationPrefix + "source-branch"
	AnnotationRootDomain         = annotationPrefix + "root-domain"
	AnnotationProjectID          = annotationPrefix + "project-id"
	AnnotationOriginalAppID      = annotationPrefix + "original-application-id"
	AnnotationOwnerUserID        = annotationPrefix + "owner-user-id"
	AnnotationProjectDescription = annotationPrefix + "project-description"
	AnnotationCreatedAt          = annotationPrefix + "created-at"
	AnnotationUpdatedAt          = annotationPrefix + "updated-at"
)
