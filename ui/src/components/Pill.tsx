import type { PropsWithChildren } from "react";
import styles from "./Pill.module.css";

export function PillList({ children }: PropsWithChildren) {
  return <div className={styles.pillList}>{children}</div>;
}

export function Pill({ children }: PropsWithChildren) {
  return <span className={styles.pill}>{children}</span>;
}
