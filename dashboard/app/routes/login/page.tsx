import { redirect } from "react-router";
import { css } from "styled-system/css";
import { env } from "~/utils/env";

export const clientLoader = () => {
	const redirectTo = `${window.location.origin}/auth/callback`;
	return redirect(
		`${env("BASE_URL")}/auth/login?redirect_to=${encodeURIComponent(
			redirectTo,
		)}`,
	);
};

export default function LoginPage() {
	return (
		<div
			className={css({
				width: "100%",
				minHeight: "100dvh",
				display: "flex",
				justifyContent: "center",
				alignItems: "center",
				padding: 4,
			})}
		>
			<p className={css({ color: "gray.500", fontSize: "sm" })}>
				Redirecting to Maximum ID...
			</p>
		</div>
	);
}
