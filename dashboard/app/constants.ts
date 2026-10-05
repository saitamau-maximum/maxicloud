import type { ApplicationStatus } from "~/repository/application";
import type { DeploymentStatus } from "~/repository/deployment";

export const APP_NAME = "MaxiCloud";

export type ValueOf<T> = T[keyof T];

export const APPLICATION_STATUS = {
	RUNNING: "running",
	UNAVAILABLE: "unavailable",
	STOPPED: "stopped",
} as const;

export const DEPLOYMENT_STATUS = {
	SUCCESS: "success",
	IN_PROGRESS: "in_progress",
	FAILED: "failed",
} as const;

export const USER_STATUS = {
	ACTIVE: "active",
	INVITED: "invited",
	SUSPENDED: "suspended",
} as const;

export const CREATE_APPLICATION_ACCESS_MODE = {
	PUBLIC: "public",
	PRIVATE: "private",
	IDP: "idp",
} as const;

export const CREATE_APPLICATION_DOCKERFILE_SOURCE = {
	PATH: "path",
	INLINE: "inline",
} as const;

export const CREATE_APPLICATION_BUILD_STRATEGY = {
	BUILDPACKS: "buildpacks",
	DOCKERFILE: "dockerfile",
} as const;

export const GIT_PROVIDER = {
	GITHUB: "github",
} as const;

export const APPLICATION_STATUS_LABEL: Record<ApplicationStatus, string> = {
	[APPLICATION_STATUS.RUNNING]: "Running",
	[APPLICATION_STATUS.UNAVAILABLE]: "Unavailable",
	[APPLICATION_STATUS.STOPPED]: "Stopped",
};

export const DEPLOYMENT_STATUS_LABEL: Record<DeploymentStatus, string> = {
	[DEPLOYMENT_STATUS.SUCCESS]: "Success",
	[DEPLOYMENT_STATUS.IN_PROGRESS]: "In Progress",
	[DEPLOYMENT_STATUS.FAILED]: "Failed",
};
