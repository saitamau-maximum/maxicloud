import { Layers } from "react-feather";
import { useFormContext, useWatch } from "react-hook-form";
import { css } from "styled-system/css";
import { Form } from "~/components/ui/form";
import {
	CREATE_APPLICATION_BUILD_STRATEGY,
	CREATE_APPLICATION_DOCKERFILE_SOURCE,
} from "~/constants";
import type { CreateApplicationInputValues } from "../schema";
import { ModeButton } from "./mode-button";
import { SectionHeading } from "./section-heading";

export const BuildSection = () => {
	const {
		register,
		setValue,
		control,
		formState: { errors },
	} = useFormContext<CreateApplicationInputValues>();
	const buildStrategy = useWatch({ control, name: "buildStrategy" });
	const dockerfileSource = useWatch({ control, name: "dockerfileSource" });

	return (
		<section className={css({ display: "grid", gap: 3 })}>
			<SectionHeading
				icon={<Layers size={15} />}
				title="3. Build Strategy"
				description="Buildpacksによる自動検出、またはDockerfileを選択"
			/>

			<div
				className={css({
					display: "grid",
					gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
					gap: 2,
					mdDown: { gridTemplateColumns: "1fr" },
				})}
			>
				<ModeButton
					active={
						buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.BUILDPACKS
					}
					title="Buildpacks"
					description="言語と起動方法を自動検出"
					onClick={() => {
						setValue(
							"buildStrategy",
							CREATE_APPLICATION_BUILD_STRATEGY.BUILDPACKS,
							{
								shouldDirty: true,
							},
						);
						setValue("port", "8080", { shouldDirty: true });
					}}
				/>
				<ModeButton
					active={
						buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.DOCKERFILE
					}
					title="Dockerfile"
					description="リポジトリ内またはinlineのDockerfileを使用"
					onClick={() =>
						setValue(
							"buildStrategy",
							CREATE_APPLICATION_BUILD_STRATEGY.DOCKERFILE,
							{
								shouldDirty: true,
							},
						)
					}
				/>
			</div>

			{buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.DOCKERFILE && (
				<div
					className={css({
						display: "grid",
						gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
						gap: 2,
						mdDown: { gridTemplateColumns: "1fr" },
					})}
				>
					<ModeButton
						active={
							dockerfileSource === CREATE_APPLICATION_DOCKERFILE_SOURCE.PATH
						}
						title="Path"
						description="リポジトリ内のパスを指定"
						onClick={() =>
							setValue(
								"dockerfileSource",
								CREATE_APPLICATION_DOCKERFILE_SOURCE.PATH,
								{
									shouldDirty: true,
								},
							)
						}
					/>
					<ModeButton
						active={
							dockerfileSource === CREATE_APPLICATION_DOCKERFILE_SOURCE.INLINE
						}
						title="Inline"
						description="Dockerfile本文を直接入力"
						onClick={() =>
							setValue(
								"dockerfileSource",
								CREATE_APPLICATION_DOCKERFILE_SOURCE.INLINE,
								{
									shouldDirty: true,
								},
							)
						}
					/>
				</div>
			)}

			<div className={css({ display: "grid", gap: 2 })}>
				{buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.DOCKERFILE &&
					dockerfileSource === CREATE_APPLICATION_DOCKERFILE_SOURCE.PATH && (
						<Form.Field.TextInput
							label="Dockerfile Path"
							error={errors.dockerfilePath?.message}
							placeholder="deploy/Dockerfile"
							{...register("dockerfilePath")}
						/>
					)}

				{buildStrategy === CREATE_APPLICATION_BUILD_STRATEGY.DOCKERFILE &&
					dockerfileSource === CREATE_APPLICATION_DOCKERFILE_SOURCE.INLINE && (
						<Form.Field.TextArea
							label="Dockerfile Inline"
							error={errors.dockerfileInline?.message}
							rows={9}
							{...register("dockerfileInline")}
						/>
					)}
			</div>
		</section>
	);
};
