package domain

type DockerfileSource interface {
	dockerfileSource()
}

type DockerfileSourcePath struct {
	Path string
}

type DockerfileSourceInline struct {
	Content string
}

func (d DockerfileSourcePath) dockerfileSource()   {}
func (d DockerfileSourceInline) dockerfileSource() {}

type BuildConfig interface{ buildConfig() }

type BuildConfigDockerfile struct {
	Source DockerfileSource
}

type BuildConfigBuildpacks struct {
	Builder string
}

func (b BuildConfigDockerfile) buildConfig() {}
func (b BuildConfigBuildpacks) buildConfig() {}
