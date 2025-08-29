import {BrowserRouter, Routes, Route, Navigate} from "react-router-dom";
import LandingPage from "../pages/landing";
import DashboardPage from "../pages/dashboard";

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
      </Routes>
    </BrowserRouter>
  );
}
