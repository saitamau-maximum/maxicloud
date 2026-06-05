import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { UserAccount } from "~/repository/user";
import { clearToken, getToken } from "~/utils/auth";
import { useRepository } from "./use-repository";

type SessionContextValue = {
	isReady: boolean;
	isLoggedIn: boolean;
	me: UserAccount | null;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export const SessionProvider = ({
	children,
}: {
	children: React.ReactNode;
}) => {
	const { userRepository } = useRepository();
	const [me, setMe] = useState<UserAccount | null>(null);
	const [isReady, setIsReady] = useState(false);

	useEffect(() => {
		let cancelled = false;

		const bootstrap = async () => {
			const token = getToken();
			if (!token) {
				if (!cancelled) setIsReady(true);
				return;
			}
			try {
				const user = await userRepository.getMe();
				if (!cancelled) setMe(user);
			} catch {
				clearToken();
				if (!cancelled) setMe(null);
			} finally {
				if (!cancelled) setIsReady(true);
			}
		};

		void bootstrap();

		return () => {
			cancelled = true;
		};
	}, [userRepository]);

	const value = useMemo(
		() => ({
			isReady,
			isLoggedIn: !!me,
			me,
		}),
		[me, isReady],
	);

	return (
		<SessionContext.Provider value={value}>{children}</SessionContext.Provider>
	);
};

export const useSession = () => {
	const context = useContext(SessionContext);
	if (!context) {
		throw new Error("useSession must be used within SessionProvider");
	}
	return context;
};
