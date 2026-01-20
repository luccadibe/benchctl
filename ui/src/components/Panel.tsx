import type { PropsWithChildren } from "react";
import styles from "./Panel.module.css";

type PanelProps = PropsWithChildren<{
  className?: string;
  soft?: boolean;
}>;

export default function Panel({ children, className, soft = false }: PanelProps) {
  const classes = [styles.panel, soft ? styles.soft : "", className ?? ""]
    .filter(Boolean)
    .join(" ");
  return <div className={classes}>{children}</div>;
}
