import React from "react";
import ReactDOM from "react-dom/client";
import {ThemeProvider} from "./app/providers";
import {ToastProvider} from "@heroui/toast";
import App from "./app";
import "./app/index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider>
      <App />
      <ToastProvider placement="bottom-right" maxVisibleToasts={3} />
    </ThemeProvider>
  </React.StrictMode>
);
