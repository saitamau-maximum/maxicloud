import { USER_STATUS, type ValueOf } from "~/constants";
import type { User } from "~/gen/maxicloud/v1/user_pb";
import { connectClient } from "~/utils/connect";

export type UserStatus = ValueOf<typeof USER_STATUS>;

export type UserAccount = {
	id: string;
	displayId: string;
	displayName: string;
	email: string;
	status: UserStatus;
	joinedAt?: string;
};

export interface IUserRepository {
	getMe$$key(): readonly ["me"];
	getMe(): Promise<UserAccount | null>;
}

const toUser = (user: User): UserAccount => ({
	id: user.id,
	displayId: user.displayId,
	displayName: user.displayName,
	email: "",
	status: USER_STATUS.ACTIVE,
});

export class UserRepository implements IUserRepository {
	getMe$$key() {
		return ["me"] as const;
	}

	async getMe(): Promise<UserAccount | null> {
		const res = await connectClient.user.getMe({});
		if (!res.user) {
			return null;
		}
		return toUser(res.user);
	}
}
