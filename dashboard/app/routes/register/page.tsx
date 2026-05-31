import { useNavigate } from "react-router";
import { css } from "styled-system/css";
import { Button } from "~/components/ui/button";
import { Panel } from "~/components/ui/panel";
import { APP_NAME } from "~/constants";

export default function RegisterPage() {
	const navigate = useNavigate();

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
			<section
				className={css({
					width: "100%",
					maxWidth: "620px",
				})}
			>
				<Panel>
					<div>
						<h1
							className={css({ margin: 0, fontSize: "2xl", color: "gray.700" })}
						>
							OIDC Login
						</h1>
						<p
							className={css({
								marginTop: 1,
								marginBottom: 0,
								color: "gray.500",
								fontSize: "sm",
							})}
						>
							{APP_NAME} は OIDC
							連携ログイン前提で実装します。申請フォームは利用しません。
						</p>
					</div>

					<div className={css({ display: "flex", gap: 2, marginTop: 4 })}>
						<Button
							type="button"
							variant="primary"
							onClick={() => navigate("/login")}
						>
							Go to Login
						</Button>
					</div>
				</Panel>
			</section>
		</div>
	);
}
