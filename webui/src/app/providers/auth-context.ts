import {createContext} from "react";
import type {CurrentUser} from "../../shared/api/auth";

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

export type AuthContextValue = {
  clearSession: () => void;
  refreshSession: () => Promise<CurrentUser | null>;
  status: AuthStatus;
  user: CurrentUser | null;
};

export const AuthContext = createContext<AuthContextValue | null>(null);
