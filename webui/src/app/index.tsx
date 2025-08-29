import {BrowserRouter, Routes, Route, Navigate} from "react-router-dom";
import LandingPage from "../pages/landing";
import DashboardPage from "../pages/dashboard";
import BucketsPage from "../pages/buckets";
import SourcesPage from "../pages/sources";
import SourcePage from "../pages/source";

export default function App() {
  const isAuthenticated = Boolean(localStorage.getItem("auth"));

  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/"
          element={isAuthenticated ? <Navigate to="/dashboard" replace /> : <LandingPage />}
        />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/buckets" element={<BucketsPage />} />
        <Route path="/sources" element={<SourcesPage />} />
        <Route path="/sources/:id" element={<SourcePage />} />
      </Routes>
    </BrowserRouter>
  );
}
