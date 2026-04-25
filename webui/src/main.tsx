import React from "react";
import ReactDOM from "react-dom/client";
import {AuthProvider, ThemeProvider} from "./app/providers";
import {ToastProvider} from "@heroui/toast";
import App from "./app";
import "./app/index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider>
      <AuthProvider>
        <App />
        <ToastProvider placement="bottom-right" maxVisibleToasts={3} />
      </AuthProvider>
    </ThemeProvider>
  </React.StrictMode>
);
