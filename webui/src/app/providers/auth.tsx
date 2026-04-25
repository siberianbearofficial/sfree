import {
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {getCurrentUser, type CurrentUser} from "../../shared/api/auth";
import {ApiError, showErrorToast} from "../../shared/api/error";
import {AuthContext, type AuthStatus} from "./auth-context";

export function AuthProvider({children}: {children: ReactNode}) {
  const [status, setStatus] = useState<AuthStatus>("loading");
  const [user, setUser] = useState<CurrentUser | null>(null);

  const clearSession = useCallback(() => {
    setUser(null);
    setStatus("unauthenticated");
  }, []);

  const refreshSession = useCallback(async () => {
    try {
      const nextUser = await getCurrentUser();
      setUser(nextUser);
      setStatus("authenticated");
      return nextUser;
    } catch (err) {
      if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
        setUser(null);
        setStatus("unauthenticated");
        return null;
      }
      setUser(null);
      setStatus("unauthenticated");
      throw err;
    }
  }, []);

  useEffect(() => {
    void refreshSession().catch((err) => {
      showErrorToast(err);
    });
  }, [refreshSession]);

  return (
    <AuthContext.Provider value={{clearSession, refreshSession, status, user}}>
      {children}
    </AuthContext.Provider>
  );
}
