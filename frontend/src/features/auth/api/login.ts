import { apiClient } from "@/lib/api/client";
import {
  loginResponseSchema,
  loginSchema,
  type LoginCredentials,
} from "./login-schema";

export async function login(credentials: LoginCredentials) {
  const validatedCredentials = loginSchema.parse(credentials);
  const response = await apiClient.post<unknown>(
    "/auth/login",
    validatedCredentials,
  );
  const parsedResponse = loginResponseSchema.parse(response.data);

  return { accessToken: parsedResponse.data };
}
