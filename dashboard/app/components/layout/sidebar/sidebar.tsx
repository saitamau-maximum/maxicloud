import { useState } from "react";
import { css } from "styled-system/css";
import { APP_NAME } from "~/constants";
import { useSession } from "~/hooks/use-session";
import { SidebarBackdrop } from "./backdrop";
import { SidebarHamburgerButton } from "./hamburger-button";
import { SidebarNavigation } from "./navigation";
import { SidebarProfile } from "./profile";

export const Sidebar = () => {
	const [isMenuOpen, setIsMenuOpen] = useState(false);
	const { me } = useSession();
	const closeMenu = () => setIsMenuOpen(false);

	return (
		<>
			<SidebarHamburgerButton
				isOpen={isMenuOpen}
				onToggle={() => setIsMenuOpen((prev) => !prev)}
			/>

			<SidebarBackdrop isOpen={isMenuOpen} onClose={closeMenu} />

			<aside
				className={css({
					width: "100%",
					maxWidth: "260px",
					height: "100%",
					padding: 6,
					borderRight: "1px solid",
					borderRightColor: "gray.100",
					background: "white",
					display: "flex",
					flexDirection: "column",
					justifyContent: "space-between",
					mdDown: {
						position: "fixed",
						right: 0,
						top: 0,
						transform: isMenuOpen ? "translateX(0)" : "translateX(100%)",
						transition: "transform 0.25s",
						boxShadow: "0 0 16px rgba(0, 0, 0, 0.25)",
						zIndex: 25,
						maxWidth: "320px",
					},
				})}
			>
				<div>
					<p
						className={css({
							margin: 0,
							color: "gray.500",
							fontSize: "xs",
							textTransform: "uppercase",
							letterSpacing: "0.1em",
						})}
					>
						{APP_NAME}
					</p>
					<h1
						className={css({ marginTop: 1, marginBottom: 6, fontSize: "2xl" })}
					>
						{APP_NAME}
					</h1>

					<SidebarNavigation onNavigate={closeMenu} />
				</div>

				<SidebarProfile user={me ?? undefined} />
			</aside>
		</>
	);
};
