import type { PropsWithChildren } from "react";
import styles from "./Table.module.css";

type TableProps = PropsWithChildren<{
  className?: string;
}>;

export default function Table({ children, className }: TableProps) {
  const classes = [styles.table, className ?? ""].filter(Boolean).join(" ");
  return <table className={classes}>{children}</table>;
}
