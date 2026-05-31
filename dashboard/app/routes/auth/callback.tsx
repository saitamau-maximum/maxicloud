import { useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { setToken } from "~/utils/auth";

export default function AuthCallback() {
	const navigate = useNavigate();
	const [params] = useSearchParams();

	useEffect(() => {
		const token = params.get("token");
		if (token) {
			setToken(token);
			navigate("/", { replace: true });
		} else {
			navigate("/login", { replace: true });
		}
	}, [params, navigate]);

	return null;
}
