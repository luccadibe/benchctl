import type { PropsWithChildren } from "react";
import styles from "./Preview.module.css";

type PreviewProps = PropsWithChildren<{
  className?: string;
}>;

export default function Preview({ children, className }: PreviewProps) {
  const classes = [styles.preview, className ?? ""].filter(Boolean).join(" ");
  return <div className={classes}>{children}</div>;
}
