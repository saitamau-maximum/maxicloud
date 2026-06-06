import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ApplicationService } from "~/gen/maxicloud/v1/application_pb";
import { DeploymentService } from "~/gen/maxicloud/v1/deployment_pb";
import { DomainService } from "~/gen/maxicloud/v1/domain_pb";
import { GitHubService } from "~/gen/maxicloud/v1/github_pb";
import { ProjectService } from "~/gen/maxicloud/v1/project_pb";
import { UserService } from "~/gen/maxicloud/v1/user_pb";
import { env } from "~/utils/env";

const transport = createConnectTransport({
	baseUrl: env("BASE_URL"),
	fetch: (input, init) =>
		fetch(input, {
			...init,
			credentials: "include",
		}),
	useBinaryFormat: false,
});

export const connectClient = {
	project: createClient(ProjectService, transport),
	application: createClient(ApplicationService, transport),
	deployment: createClient(DeploymentService, transport),
	domain: createClient(DomainService, transport),
	github: createClient(GitHubService, transport),
	user: createClient(UserService, transport),
};
