import {useEffect} from "react";
import {useNavigate} from "react-router-dom";
import {useAuth} from "../../app/providers";

export function OAuthCallback() {
  const {refreshSession} = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    void refreshSession()
      .then((user) => {
        navigate(user ? "/dashboard" : "/", {replace: true});
      })
      .catch(() => {
        navigate("/", {replace: true});
      });
  }, [navigate, refreshSession]);

  return <div className="flex items-center justify-center min-h-screen">Signing in...</div>;
}
