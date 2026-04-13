import {useEffect} from "react";
import {useNavigate, useSearchParams} from "react-router-dom";
import {saveTokenAuth} from "../../shared/lib/auth";

export function OAuthCallback() {
  const [params] = useSearchParams();
  const navigate = useNavigate();

  useEffect(() => {
    const token = params.get("token");
    const username = params.get("username");
    if (token && username) {
      saveTokenAuth(token, username);
      navigate("/dashboard", {replace: true});
    } else {
      navigate("/", {replace: true});
    }
  }, [params, navigate]);

  return <div className="flex items-center justify-center min-h-screen">Signing in...</div>;
}
