import type {ReactNode} from "react";
import {BrowserRouter, Routes, Route, Navigate} from "react-router-dom";
import LandingPage from "../pages/landing";
import DashboardPage from "../pages/dashboard";
import BucketsPage from "../pages/buckets";
import BucketPage from "../pages/bucket";
import SourcesPage from "../pages/sources";
import SourcePage from "../pages/source";
import {OAuthCallback} from "../pages/auth-callback";
import {useAuth} from "./providers";
import {AppShell} from "../widgets/app-shell";

function FullPageStatus({label}: {label: string}) {
  return (
    <div className="flex min-h-screen items-center justify-center text-sm text-default-500">
      {label}
    </div>
  );
}

function RequireAuth({children}: {children: ReactNode}) {
  const {status} = useAuth();
  if (status === "loading") {
    return <FullPageStatus label="Loading session..." />;
  }
  if (status !== "authenticated") {
    return <Navigate to="/" replace />;
  }
  return <>{children}</>;
}

function RequireGuest({children}: {children: ReactNode}) {
  const {status} = useAuth();
  if (status === "loading") {
    return <FullPageStatus label="Loading session..." />;
  }
  if (status === "authenticated") {
    return <Navigate to="/dashboard" replace />;
  }
  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/"
          element={
            <RequireGuest>
              <LandingPage />
            </RequireGuest>
          }
        />
        <Route path="/auth/callback" element={<OAuthCallback />} />
        <Route
          element={
            <RequireAuth>
              <AppShell />
            </RequireAuth>
          }
        >
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/buckets" element={<BucketsPage />} />
          <Route path="/buckets/:id" element={<BucketPage />} />
          <Route path="/sources" element={<SourcesPage />} />
          <Route path="/sources/:id" element={<SourcePage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
