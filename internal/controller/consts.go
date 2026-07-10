package controller

import "fmt"

const (
	installationAccessTokenKey = "installation-access-token"
	dockerfileInlineKey        = "Dockerfile"

	appNameLabel = "app.kubernetes.io/name"
)

// appRegistrySecretName は Application が Registry から Image を Pull するための Secret 名を返す。
func appRegistrySecretName(appName string) string {
	return fmt.Sprintf("%s-registry-secret", appName)
}
