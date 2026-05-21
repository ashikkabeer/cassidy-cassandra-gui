import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { Toaster } from "sonner";
import App from "./App";
import "./styles/globals.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
      <Toaster
        theme="dark"
        position="bottom-right"
        toastOptions={{
          style: {
            background: "hsl(240 10% 5%)",
            border: "1px solid hsl(240 4% 22%)",
            color: "hsl(0 0% 98%)",
            fontSize: "12.5px",
          },
        }}
      />
    </BrowserRouter>
  </React.StrictMode>,
);
