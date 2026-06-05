import { redirect } from "react-router";
import { setToken } from "~/utils/auth";

export const clientLoader = ({ request }: { request: Request }) => {
	const token = new URL(request.url).searchParams.get("token");
	if (!token) {
		return redirect("/login");
	}
	setToken(token);
	return redirect("/");
};

export default function AuthCallback() {
	return null;
}
