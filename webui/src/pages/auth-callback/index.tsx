import {useEffect} from "react";
import {useNavigate, useSearchParams} from "react-router-dom";
import {saveCookieAuth} from "../../shared/lib/auth";

export function OAuthCallback() {
  const [params] = useSearchParams();
  const navigate = useNavigate();

  useEffect(() => {
    const username = params.get("username");
    if (username) {
      saveCookieAuth(username);
      navigate("/dashboard", {replace: true});
    } else {
      navigate("/", {replace: true});
    }
  }, [params, navigate]);

  return <div className="flex items-center justify-center min-h-screen">Signing in...</div>;
}
