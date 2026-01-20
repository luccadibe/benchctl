import type { PropsWithChildren } from "react";
import styles from "./EmptyState.module.css";

export default function EmptyState({ children }: PropsWithChildren) {
  return <div className={styles.empty}>{children}</div>;
}
