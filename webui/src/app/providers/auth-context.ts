import {createContext, useContext} from "react";
import type {CurrentUser} from "../../shared/api/auth";

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

export type AuthContextValue = {
  clearSession: () => void;
  refreshSession: () => Promise<CurrentUser | null>;
  status: AuthStatus;
  user: CurrentUser | null;
};

export const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (value === null) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return value;
}
