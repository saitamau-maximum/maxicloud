import * as v from "valibot";
import {
	CREATE_APPLICATION_ACCESS_MODE,
	CREATE_APPLICATION_BUILD_STRATEGY,
	CREATE_APPLICATION_DOCKERFILE_SOURCE,
} from "~/constants";

export const CreateApplicationSchema = v.object({
	projectId: v.pipe(
		v.string(),
		v.minLength(1, "プロジェクトを選択してください"),
	),
	applicationName: v.pipe(
		v.string(),
		v.minLength(1, "アプリケーション名を入力してください"),
		v.regex(
			/^[A-Za-z0-9][-A-Za-z0-9_.]*[A-Za-z0-9]$/,
			"アプリケーション名は英数字とハイフン、アンダースコア、ピリオドのみ使用できます",
		),
		v.maxLength(64, "アプリケーション名は64文字以内で入力してください"),
	),
	repositoryId: v.pipe(
		v.string(),
		v.minLength(1, "リポジトリを選択してください"),
	),
	branch: v.pipe(v.string(), v.minLength(1, "ブランチを選択してください")),
	buildStrategy: v.union([
		v.literal(CREATE_APPLICATION_BUILD_STRATEGY.BUILDPACKS),
		v.literal(CREATE_APPLICATION_BUILD_STRATEGY.DOCKERFILE),
	]),
	dockerfileSource: v.union([
		v.literal(CREATE_APPLICATION_DOCKERFILE_SOURCE.PATH),
		v.literal(CREATE_APPLICATION_DOCKERFILE_SOURCE.INLINE),
	]),
	dockerfilePath: v.string(),
	dockerfileInline: v.string(),
	exposureMode: v.union([
		v.literal(CREATE_APPLICATION_ACCESS_MODE.PUBLIC),
		v.literal(CREATE_APPLICATION_ACCESS_MODE.PRIVATE),
		v.literal(CREATE_APPLICATION_ACCESS_MODE.IDP),
	]),
	domainPrefix: v.string(),
	domainSuffix: v.string(),
	domainEdited: v.boolean(),
	port: v.pipe(
		v.string(),
		v.minLength(1, "ポート番号を入力してください"),
		v.regex(/^\d+$/, "ポート番号は数字で入力してください"),
		v.transform(Number),
		v.integer("ポート番号は整数で入力してください"),
		v.minValue(1, "ポート番号は1〜65535の範囲で指定してください"),
		v.maxValue(65535, "ポート番号は1〜65535の範囲で指定してください"),
	),
	envText: v.string(),
	secrets: v.array(
		v.object({
			id: v.string(),
			key: v.string(),
			value: v.string(),
		}),
	),
});

export type CreateApplicationInputValues = v.InferInput<
	typeof CreateApplicationSchema
>;
export type CreateApplicationOutput = v.InferOutput<
	typeof CreateApplicationSchema
>;
