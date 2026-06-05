import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useEffect, useMemo, useRef, useState } from "react";
import { DEPLOYMENT_STATUS } from "~/constants";
import {
	type DeploymentStatus,
	mapDeploymentStatus,
} from "~/repository/deployment";
import { connectClient } from "~/utils/connect";
import { formatElapsedSeconds } from "~/utils/date";

type WatchState = {
	status: DeploymentStatus | null;
	elapsedSeconds: number;
	finishedAt?: Date;
	logLines: { id: number; line: string }[];
};

export const useWatchDeployment = (deploymentId: string) => {
	const [state, setState] = useState<WatchState>({
		status: null,
		elapsedSeconds: 0,
		finishedAt: undefined,
		logLines: [],
	});
	const [nowTick, setNowTick] = useState(0);
	const nextLogID = useRef(0);

	useEffect(() => {
		const abortController = new AbortController();

		(async () => {
			try {
				const stream = connectClient.deployment.watchDeployment(
					{ deploymentId },
					{ signal: abortController.signal },
				);
				for await (const event of stream) {
					const e = event.event;
					if (e.case === "deploymentStatusChanged") {
						setNowTick(0);
						setState((prev) => ({
							...prev,
							status: mapDeploymentStatus(e.value.status),
							elapsedSeconds: Number(e.value.elapsedSeconds),
							finishedAt: e.value.finishedAt
								? timestampDate(e.value.finishedAt)
								: undefined,
						}));
					} else if (e.case === "deploymentLogChunk") {
						setState((prev) => ({
							...prev,
							logLines: [
								...prev.logLines,
								{ id: nextLogID.current++, line: e.value.line },
							],
						}));
					}
				}
			} catch (error) {
				// AbortError は正常終了
				if (abortController.signal.aborted) {
					return;
				}
				const message =
					error instanceof Error
						? error.message
						: "failed to watch deployment stream";
				setState((prev) => ({
					...prev,
					logLines: [
						...prev.logLines,
						{ id: nextLogID.current++, line: `[stream error] ${message}` },
					],
				}));
			}
		})();

		return () => {
			abortController.abort();
		};
	}, [deploymentId]);

	useEffect(() => {
		if (state.status !== DEPLOYMENT_STATUS.IN_PROGRESS) return;
		const id = window.setInterval(() => setNowTick((v) => v + 1), 1000);
		return () => window.clearInterval(id);
	}, [state.status]);

	const duration = useMemo(() => {
		const currentSeconds =
			state.status === DEPLOYMENT_STATUS.IN_PROGRESS
				? state.elapsedSeconds + nowTick
				: state.elapsedSeconds;
		return formatElapsedSeconds(currentSeconds);
	}, [state.elapsedSeconds, state.status, nowTick]);

	return { ...state, duration };
};
