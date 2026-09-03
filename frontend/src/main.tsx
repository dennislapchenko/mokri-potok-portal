import React from "react";
import ReactDOM from "react-dom/client";
import { HashRouter } from "react-router-dom";
import { I18n } from "./i18n";
import App from "./App";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18n>
      <HashRouter>
        <App />
      </HashRouter>
    </I18n>
  </React.StrictMode>
);
