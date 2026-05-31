import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useState,
} from "react";
import type { UserAccount } from "~/repository/user";
import { clearToken, getToken } from "~/utils/auth";
import { env } from "~/utils/env";
import { useRepository } from "./use-repository";

type SessionContextValue = {
	isReady: boolean;
	isLoggedIn: boolean;
	me: UserAccount | null;
	login: () => void;
	logout: () => void;
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

	const login = useCallback(() => {
		const redirectTo = `${window.location.origin}/auth/callback`;
		window.location.href = `${env("BASE_URL")}/auth/login?redirect_to=${encodeURIComponent(
			redirectTo,
		)}`;
	}, []);

	const logout = useCallback(() => {
		clearToken();
		setMe(null);
		window.location.href = "/login";
	}, []);

	const value = useMemo(
		() => ({
			isReady,
			isLoggedIn: !!me,
			me,
			login,
			logout,
		}),
		[me, isReady, login, logout],
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
