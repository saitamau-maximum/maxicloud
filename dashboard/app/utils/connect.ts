import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ApplicationService } from "~/gen/maxicloud/v1/application_pb";
import { DeploymentService } from "~/gen/maxicloud/v1/deployment_pb";
import { DomainService } from "~/gen/maxicloud/v1/domain_pb";
import { GitHubService } from "~/gen/maxicloud/v1/github_pb";
import { ProjectService } from "~/gen/maxicloud/v1/project_pb";
import { UserService } from "~/gen/maxicloud/v1/user_pb";
import { getToken } from "~/utils/auth";
import { env } from "~/utils/env";

const authInterceptor: Interceptor = (next) => async (req) => {
	const token = getToken();
	if (token) {
		req.header.set("Authorization", `Bearer ${token}`);
	}
	return next(req);
};

const transport = createConnectTransport({
	baseUrl: env("BASE_URL"),
	useBinaryFormat: false,
	interceptors: [authInterceptor],
});

export const connectClient = {
	project: createClient(ProjectService, transport),
	application: createClient(ApplicationService, transport),
	deployment: createClient(DeploymentService, transport),
	domain: createClient(DomainService, transport),
	github: createClient(GitHubService, transport),
	user: createClient(UserService, transport),
};
