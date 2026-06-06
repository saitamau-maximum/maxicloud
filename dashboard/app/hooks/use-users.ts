import { useQuery } from "@tanstack/react-query";
import { useRepository } from "~/hooks/use-repository";
import type { UserAccount } from "~/repository/user";

export const useUsersQuery = () => {
	const { userRepository } = useRepository();
	return useQuery({
		queryKey: userRepository.getMe$$key(),
		queryFn: async (): Promise<UserAccount[]> => {
			const user = await userRepository.getMe();
			return user ? [user] : [];
		},
		initialData: [],
	});
};
