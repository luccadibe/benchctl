import { createRoot } from "react-dom/client";
import "uplot/dist/uPlot.min.css";
import "@fontsource/space-grotesk/400.css";
import "@fontsource/space-grotesk/600.css";
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "./styles/global.css";
import App from "./app/App";

const container = document.getElementById("app");
if (!container) {
  throw new Error("app container not found");
}

createRoot(container).render(<App />);
