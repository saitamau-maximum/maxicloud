import { redirect } from "react-router";

export const clientLoader = () => redirect("/");

export default function AuthCallback() {
	return null;
}
