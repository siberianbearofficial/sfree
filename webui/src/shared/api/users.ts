import {apiJson} from "./client";

export async function createUser(
  username: string,
): Promise<{id: string; created_at: string; password: string}> {
  return apiJson<{id: string; created_at: string; password: string}>(
    "/users",
    "Failed to create user",
    {
      auth: false,
      method: "POST",
      json: {username},
    },
  );
}
