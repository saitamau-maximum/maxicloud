import { css } from "styled-system/css";
import { Button } from "~/components/ui/button";
import { Panel } from "~/components/ui/panel";
import { APP_NAME } from "~/constants";
import { useSession } from "~/hooks/use-session";

export default function LoginPage() {
	const { login } = useSession();

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
			<div
				className={css({
					width: "100%",
					maxWidth: "520px",
				})}
			>
				<Panel>
					<div>
						<h1
							className={css({ margin: 0, fontSize: "2xl", color: "gray.700" })}
						>
							{APP_NAME}
						</h1>
						<p
							className={css({
								marginTop: 1,
								marginBottom: 0,
								color: "gray.500",
								fontSize: "sm",
							})}
						>
							Maximum ID でログインしてください。
						</p>
					</div>
					<div className={css({ display: "flex", gap: 2, marginTop: 2 })}>
						<Button type="button" variant="primary" onClick={() => login()}>
							Login with Maximum ID
						</Button>
					</div>
				</Panel>
			</div>
		</div>
	);
}
