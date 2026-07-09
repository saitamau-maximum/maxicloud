import { Code, ConnectError } from "@connectrpc/connect";
import {
	APPLICATION_STATUS,
	CREATE_APPLICATION_ACCESS_MODE,
	CREATE_APPLICATION_BUILD_STRATEGY,
	CREATE_APPLICATION_DOCKERFILE_SOURCE,
	type ValueOf,
} from "~/constants";
import {
	AccessMode,
	BuildStrategy,
	DockerfileSource,
	type Application as ProtoApplication,
	ApplicationStatus as ProtoApplicationStatus,
} from "~/gen/maxicloud/v1/application_pb";
import { connectClient } from "~/utils/connect";
import { formatTimestamp } from "~/utils/date";

export type ApplicationStatus = ValueOf<typeof APPLICATION_STATUS>;

export type Application = {
	id: string;
	projectId: string;
	name: string;
	repository: string;
	branch: string;
	status: ApplicationStatus;
	url?: string;
	updatedAt: string;
	ownerId: string;
};

export type CreateApplicationInput = {
	projectId: string;
	ownerId: string;
	name: string;
	repositoryOwner: string;
	repositoryName: string;
	branch: string;
	buildStrategy: ValueOf<typeof CREATE_APPLICATION_BUILD_STRATEGY>;
	dockerfileSource: ValueOf<typeof CREATE_APPLICATION_DOCKERFILE_SOURCE>;
	dockerfilePath: string;
	dockerfileInline: string;
	accessMode: ValueOf<typeof CREATE_APPLICATION_ACCESS_MODE>;
	domainSubdomain?: string;
	domainRootDomain?: string;
	port: number;
	environmentVariables: Record<string, string>;
	secrets: Record<string, string>;
};

export type CreateApplicationResult = {
	application: Application;
	initialDeploymentID?: string;
	initialDeploymentStarted: boolean;
	initialDeploymentError?: string;
};

export interface IApplicationRepository {
	listApplications$$key(): readonly ["applications"];
	getApplication$$key(id: string): readonly ["applications", string];
	listApplications(): Promise<Application[]>;
	getApplication(id: string): Promise<Application | undefined>;
	createApplication(
		input: CreateApplicationInput,
	): Promise<CreateApplicationResult>;
	deleteApplication(id: string): Promise<void>;
}

const mapApplicationStatus = (
	status: ProtoApplicationStatus,
): ApplicationStatus => {
	switch (status) {
		case ProtoApplicationStatus.RUNNING:
			return APPLICATION_STATUS.RUNNING;
		case ProtoApplicationStatus.UNAVAILABLE:
			return APPLICATION_STATUS.UNAVAILABLE;
		case ProtoApplicationStatus.STOPPED:
			return APPLICATION_STATUS.STOPPED;
		default:
			return APPLICATION_STATUS.UNAVAILABLE;
	}
};

const toApplication = (application: ProtoApplication): Application => {
	if (
		!application.source?.repository?.owner ||
		!application.source.repository.name ||
		!application.updatedAt ||
		!application.condition
	) {
		throw new Error(
			`Invalid application data received for ID: ${application.id}`,
		);
	}
	return {
		id: application.id,
		projectId: application.projectId,
		name: application.name,
		repository: `${application.source.repository.owner}/${application.source.repository.name}`,
		branch: application.source.branch,
		status: mapApplicationStatus(application.condition.status),
		// TODO: 後でここなおす
		url: application.condition?.domain
			? `http://${application.condition.domain.subdomain}.${application.condition.domain.rootDomain}:8080`
			: undefined,
		updatedAt: formatTimestamp(application.updatedAt),
		ownerId: application.ownerUserId,
	};
};

const toAccessMode = (
	accessMode: CreateApplicationInput["accessMode"],
): AccessMode => {
	switch (accessMode) {
		case CREATE_APPLICATION_ACCESS_MODE.PUBLIC:
			return AccessMode.PUBLIC;
		case CREATE_APPLICATION_ACCESS_MODE.PRIVATE:
			return AccessMode.PRIVATE;
		case CREATE_APPLICATION_ACCESS_MODE.IDP:
			return AccessMode.MEMBERS_ONLY;
		default:
			return AccessMode.UNSPECIFIED;
	}
};

export class ApplicationRepository implements IApplicationRepository {
	listApplications$$key() {
		return ["applications"] as const;
	}

	getApplication$$key(id: string) {
		return ["applications", id] as const;
	}

	async listApplications(): Promise<Application[]> {
		const { projects } = await connectClient.project.listProjects({});
		const all: Application[] = [];

		await Promise.all(
			projects.map(async (project) => {
				const res = await connectClient.application.listApplications({
					projectId: project.id,
				});
				all.push(...res.applications.map(toApplication));
			}),
		);

		return all.sort((a, b) => (a.updatedAt > b.updatedAt ? -1 : 1));
	}

	async getApplication(id: string): Promise<Application | undefined> {
		try {
			const res = await connectClient.application.getApplication({
				applicationId: id,
			});
			if (!res.application) {
				return undefined;
			}
			return toApplication(res.application);
		} catch (error) {
			if (error instanceof ConnectError && error.code === Code.NotFound) {
				return undefined;
			}
			throw error;
		}
	}

	async createApplication(
		input: CreateApplicationInput,
	): Promise<CreateApplicationResult> {
		const res = await connectClient.application.createApplication({
			name: input.name.trim(),
			ownerId: input.ownerId,
			spec: {
				projectId: input.projectId,
				source: {
					repository: {
						owner: input.repositoryOwner,
						name: input.repositoryName,
					},
					branch: input.branch,
				},
				build: {
					strategy:
						input.buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.BUILDPACKS
							? BuildStrategy.BUILDPACKS
							: BuildStrategy.DOCKERFILE,
					dockerfile:
						input.buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.DOCKERFILE
							? {
									source:
										input.dockerfileSource ===
										CREATE_APPLICATION_DOCKERFILE_SOURCE.INLINE
											? DockerfileSource.INLINE
											: DockerfileSource.PATH,
									dockerfilePath: input.dockerfilePath,
									dockerfileInline: input.dockerfileInline,
								}
							: undefined,
					buildpacks:
						input.buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.BUILDPACKS
							? { builder: "" }
							: undefined,
				},
				access: {
					mode: toAccessMode(input.accessMode),
					domain:
						input.accessMode === CREATE_APPLICATION_ACCESS_MODE.PRIVATE
							? undefined
							: {
									subdomain: input.domainSubdomain ?? "",
									rootDomain: input.domainRootDomain ?? "",
								},
					port: input.port,
				},
				environmentVariables: input.environmentVariables,
				secrets: input.secrets,
			},
		});

		if (!res.application) {
			throw new Error("CreateApplication returned empty application");
		}

		return {
			application: toApplication(res.application),
			initialDeploymentID: res.initialDeploymentId,
			initialDeploymentStarted: res.initialDeploymentStarted,
			initialDeploymentError: res.initialDeploymentError,
		};
	}

	async deleteApplication(id: string): Promise<void> {
		await connectClient.application.deleteApplication({ applicationId: id });
	}
}
