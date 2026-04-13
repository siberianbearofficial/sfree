import {BrowserRouter, Routes, Route, Navigate} from "react-router-dom";
import LandingPage from "../pages/landing";
import DashboardPage from "../pages/dashboard";
import BucketsPage from "../pages/buckets";
import BucketPage from "../pages/bucket";
import SourcesPage from "../pages/sources";
import SourcePage from "../pages/source";
import {OAuthCallback} from "../pages/auth-callback";
import {isAuthenticated} from "../shared/lib/auth";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/"
          element={isAuthenticated() ? <Navigate to="/dashboard" replace /> : <LandingPage />}
        />
        <Route path="/auth/callback" element={<OAuthCallback />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/buckets" element={<BucketsPage />} />
        <Route path="/buckets/:id" element={<BucketPage />} />
        <Route path="/sources" element={<SourcesPage />} />
        <Route path="/sources/:id" element={<SourcePage />} />
      </Routes>
    </BrowserRouter>
  );
}
