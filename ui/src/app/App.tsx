import styles from "./App.module.css";
import { useRoute } from "../router/router";
import Link from "../router/Link";
import Button from "../components/Button";
import { useTheme } from "../lib/theme";
import { useRuns } from "../lib/store";
import DashboardView from "../views/DashboardView";
import CompareView from "../views/CompareView";
import RunDetailView from "../views/RunDetailView";

export default function App() {
  const route = useRoute();
  const { theme, toggle } = useTheme();
  const { runs, loading, error } = useRuns();

  return (
    <div className={styles.app}>
      <header className={styles.topbar}>
        <div className={styles.brand}>
          <div className={styles.brandTitle}>benchctl UI</div>
          <div className={styles.brandSub}>Exploration console</div>
        </div>
        <div className={styles.topActions}>
          <nav className={styles.nav}>
            <Link
              to="/"
              className={`${styles.navLink} ${route.name === "dashboard" ? styles.navLinkActive : ""}`}
            >
              Dashboard
            </Link>
            <Link
              to="/compare"
              className={`${styles.navLink} ${route.name === "compare" ? styles.navLinkActive : ""}`}
            >
              Compare
            </Link>
          </nav>
          <Button variant="ghost" type="button" onClick={toggle} aria-pressed={theme === "dark"}>
            Theme: {theme === "dark" ? "Dark" : "Light"}
          </Button>
        </div>
      </header>
      <main className={styles.main}>
        {route.name === "dashboard" && (
          <DashboardView runs={runs} loading={loading} error={error} theme={theme} />
        )}
        {route.name === "compare" && <CompareView runs={runs} loading={loading} error={error} />}
        {route.name === "run" && <RunDetailView runId={route.runId} />}
      </main>
    </div>
  );
}
