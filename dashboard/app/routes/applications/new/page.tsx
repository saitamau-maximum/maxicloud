import { valibotResolver } from "@hookform/resolvers/valibot";
import { FormProvider, useForm } from "react-hook-form";
import { css } from "styled-system/css";
import { Panel } from "~/components/ui/panel";
import {
	CREATE_APPLICATION_ACCESS_MODE,
	CREATE_APPLICATION_DOCKERFILE_SOURCE,
} from "~/constants";
import { useSession } from "~/hooks/use-session";
import {
	type CreateApplicationInputValues,
	type CreateApplicationOutput,
	CreateApplicationSchema,
} from "~/routes/applications/new/internal/schema";
import {
	ActionRow,
	BuildSection,
	EnvironmentSection,
	ExposeSection,
	GeneralSection,
	ProjectSection,
	SourceSection,
} from "./internal/components";
import { useCreateApplication } from "./internal/hooks/use-create-application";
import { useGitHubRepositories } from "./internal/hooks/use-source";

const DEFAULT_BRANCH = "main";

// TODO: こいつもutilsとかにおきたいな
const parseKeyValueLines = (value: string): Record<string, string> => {
	return value
		.split("\n")
		.map((line) => line.trim())
		.filter((line) => line.length > 0 && line.includes("="))
		.reduce<Record<string, string>>((acc, line) => {
			const delimiter = line.indexOf("=");
			const key = line.slice(0, delimiter).trim();
			const val = line.slice(delimiter + 1).trim();
			if (key.length > 0) {
				acc[key] = val;
			}
			return acc;
		}, {});
};

// TODO: そもそもRepositoryでくっつけちゃってるのでは？？
const parseRepositoryFullName = (fullName: string) => {
	const [owner, name] = fullName.split("/", 2);
	return {
		owner: owner ?? "",
		name: name ?? "",
	};
};

export default function NewApplicationPage() {
	const { me } = useSession();
	const { mutate: createApplication, isPending } = useCreateApplication();
	const { data: githubRepositories = [] } = useGitHubRepositories();

	const methods = useForm<
		CreateApplicationInputValues,
		unknown,
		CreateApplicationOutput
	>({
		resolver: valibotResolver(CreateApplicationSchema),
		defaultValues: {
			projectId: "",
			applicationName: "",
			repositoryId: "",
			branch: DEFAULT_BRANCH,
			dockerfileSource: CREATE_APPLICATION_DOCKERFILE_SOURCE.PATH,
			dockerfilePath: "Dockerfile",
			dockerfileInline: "",
			exposureMode: CREATE_APPLICATION_ACCESS_MODE.PUBLIC,
			domainPrefix: "",
			domainSuffix: "",
			domainEdited: false,
			port: "3000",
			envText: "",
			secrets: [],
		},
	});

	const {
		handleSubmit,
		formState: { isValid },
	} = methods;

	const onSubmit = (data: CreateApplicationOutput) => {
		const ownerId = me?.id;
		if (!ownerId) return;

		const repository = githubRepositories.find(
			(target) => target.id === data.repositoryId,
		);
		const repoFullName = repository?.fullName ?? "";
		if (!repoFullName) return;

		const { owner, name } = parseRepositoryFullName(repoFullName);
		const environmentVariables = parseKeyValueLines(data.envText);
		const secrets = data.secrets.reduce<Record<string, string>>((acc, item) => {
			if (item.key.trim().length > 0) {
				acc[item.key] = item.value;
			}
			return acc;
		}, {});
		const enableDomain =
			data.exposureMode !== CREATE_APPLICATION_ACCESS_MODE.PRIVATE;

		createApplication({
			projectId: data.projectId,
			ownerId,
			name: data.applicationName,
			repositoryOwner: owner,
			repositoryName: name,
			branch: data.branch,
			dockerfileSource: data.dockerfileSource,
			dockerfilePath: data.dockerfilePath,
			dockerfileInline: data.dockerfileInline,
			accessMode: data.exposureMode,
			domainSubdomain: enableDomain ? data.domainPrefix : undefined,
			domainRootDomain: enableDomain ? data.domainSuffix : undefined,
			port: data.port,
			environmentVariables,
			secrets,
		});
	};

	return (
		<Panel>
			<FormProvider {...methods}>
				<form
					className={css({ display: "grid", gap: 5 })}
					onSubmit={handleSubmit(onSubmit)}
				>
					<ProjectSection />
					<GeneralSection />
					<SourceSection />
					<BuildSection />
					<ExposeSection />
					<EnvironmentSection />
					<ActionRow isPending={isPending} canSubmit={isValid} />
				</form>
			</FormProvider>
		</Panel>
	);
}
