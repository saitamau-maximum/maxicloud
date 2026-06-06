import { valibotResolver } from "@hookform/resolvers/valibot";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router";
import { Button } from "~/components/ui/button";
import { Form } from "~/components/ui/form";
import { Panel } from "~/components/ui/panel";

import { useUsersQuery } from "~/hooks";
import { useSession } from "~/hooks/use-session";

import { useCreateProject } from "~/routes/projects/new/internal/hooks/use-create-project";
import {
	type CreateProjectInput,
	type CreateProjectOutput,
	CreateProjectSchema,
} from "~/routes/projects/new/internal/schema";

export default function NewProjectPage() {
	const navigate = useNavigate();
	const { me } = useSession();
	const { data: users } = useUsersQuery();
	const { mutateAsync: createProject, isPending } = useCreateProject();

	const {
		register,
		handleSubmit,
		setValue,
		formState: { errors },
	} = useForm<CreateProjectInput, unknown, CreateProjectOutput>({
		resolver: valibotResolver(CreateProjectSchema),
		defaultValues: {
			name: "",
			description: "",
			ownerId: "",
		},
	});

	// ユーザー一覧取得後にデフォルト owner をセット
	useEffect(() => {
		const defaultId = me?.id ?? users[0]?.id;
		if (defaultId) setValue("ownerId", defaultId);
	}, [me, users, setValue]);

	const onSubmit = async (data: CreateProjectOutput) => {
		await createProject(data);
		navigate("/projects");
	};

	return (
		<Panel>
			<form onSubmit={handleSubmit(onSubmit)}>
				<Form.FieldSet>
					<Form.Field.TextInput
						label="Project Name"
						required
						error={errors.name?.message}
						{...register("name")}
					/>

					<Form.Field.TextArea
						label="Description"
						rows={4}
						error={errors.description?.message}
						{...register("description")}
					/>

					<Form.Field.WithLabel label="Owner" required>
						{(id) => (
							<>
								<Form.Select id={id} {...register("ownerId")}>
									{users.map((user) => (
										<option key={user.id} value={user.id}>
											{user.displayName}
										</option>
									))}
								</Form.Select>
								<Form.ErrorDisplay error={errors.ownerId?.message} />
							</>
						)}
					</Form.Field.WithLabel>

					<Button type="submit" variant="primary" disabled={isPending}>
						{isPending ? "Creating..." : "Create Project"}
					</Button>
				</Form.FieldSet>
			</form>
		</Panel>
	);
}
