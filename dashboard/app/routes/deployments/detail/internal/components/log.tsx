import { useEffect, useRef, useState } from "react";
import { css } from "styled-system/css";

export const Log = ({ lines }: { lines: string[] }) => {
	const viewportRef = useRef<HTMLDivElement>(null);
	const [stickToBottom, setStickToBottom] = useState(true);

	useEffect(() => {
		if (!stickToBottom) {
			return;
		}
		const viewport = viewportRef.current;
		if (!viewport) {
			return;
		}
		viewport.scrollTop = viewport.scrollHeight;
	});

	const onScroll = () => {
		const viewport = viewportRef.current;
		if (!viewport) {
			return;
		}
		const distanceFromBottom =
			viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
		setStickToBottom(distanceFromBottom < 24);
	};

	return (
		<div className={css({ display: "grid", gap: 2 })}>
			<div
				ref={viewportRef}
				onScroll={onScroll}
				className={css({
					border: "1px solid",
					borderColor: "gray.100",
					borderRadius: "sm",
					background: "white",
					maxHeight: "420px",
					overflowY: "auto",
					padding: "3 5",
				})}
			>
				{lines.length === 0 ? (
					<span className={css({ fontSize: "xs", color: "gray.500" })}>
						ログを待機中...
					</span>
				) : (
					lines.map((line, i) => (
						<div key={i} className={css({ lineHeight: 1.8 })}>
							<span
								className={css({
									fontSize: "sm",
									color: "gray.700",
									whiteSpace: "pre-wrap",
									wordBreak: "break-word",
								})}
							>
								{line}
							</span>
						</div>
					))
				)}
			</div>
		</div>
	);
};
