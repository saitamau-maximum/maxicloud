import {
	index,
	layout,
	prefix,
	type RouteConfig,
	route,
} from "@react-router/dev/routes";

export default [
	route("login", "routes/login/page.tsx"),
	route("register", "routes/register/page.tsx"),
	route("auth/callback", "routes/auth/callback.tsx"),
	layout("routes/layout.tsx", [
		index("routes/home/page.tsx"),
		...prefix("projects", [
			layout("routes/projects/layout.tsx", [
				index("routes/projects/page.tsx"),
				route("new", "routes/projects/new/page.tsx"),
			]),
			...prefix(":projectId", [
				layout("routes/projects/detail/layout.tsx", [
					index("routes/projects/detail/page.tsx"),
				]),
			]),
		]),
		...prefix("applications", [
			layout("routes/applications/layout.tsx", [
				index("routes/applications/page.tsx"),
				route("new", "routes/applications/new/page.tsx"),
			]),
			...prefix(":applicationId", [
				layout("routes/applications/detail/layout.tsx", [
					index("routes/applications/detail/page.tsx"),
				]),
			]),
		]),
		...prefix("deployments", [
			layout("routes/deployments/layout.tsx", [
				index("routes/deployments/page.tsx"),
			]),
			...prefix(":deploymentId", [
				layout("routes/deployments/detail/layout.tsx", [
					index("routes/deployments/detail/page.tsx"),
				]),
			]),
		]),
	]),
] satisfies RouteConfig;
