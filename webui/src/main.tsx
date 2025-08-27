import React from "react";
import ReactDOM from "react-dom/client";
import {ThemeProvider} from "./app/providers";
import App from "./app";
import "./app/index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </React.StrictMode>
);
