import type { PropsWithChildren } from "react";
import styles from "./Tag.module.css";

export default function Tag({ children }: PropsWithChildren) {
  return <span className={styles.tag}>{children}</span>;
}
