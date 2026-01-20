import { useEffect, useState } from "react";

export type Route =
  | { name: "dashboard" }
  | { name: "compare" }
  | { name: "run"; runId: string };

export function parseRoute(pathname: string): Route {
  if (pathname.startsWith("/run/")) {
    const runId = decodeURIComponent(pathname.split("/")[2] || "");
    return { name: "run", runId };
  }
  if (pathname === "/compare") {
    return { name: "compare" };
  }
  return { name: "dashboard" };
}

function notify() {
  window.dispatchEvent(new Event("benchctl:navigate"));
}

export function navigate(path: string) {
  window.history.pushState({}, "", path);
  notify();
}

export function useRoute(): Route {
  const [route, setRoute] = useState(() => parseRoute(window.location.pathname));

  useEffect(() => {
    const handle = () => setRoute(parseRoute(window.location.pathname));
    const handleCustom = () => handle();
    window.addEventListener("popstate", handle);
    window.addEventListener("benchctl:navigate", handleCustom);
    return () => {
      window.removeEventListener("popstate", handle);
      window.removeEventListener("benchctl:navigate", handleCustom);
    };
  }, []);

  return route;
}
