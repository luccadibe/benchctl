import type { ButtonHTMLAttributes } from "react";
import styles from "./Button.module.css";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "default" | "primary" | "ghost";
  compact?: boolean;
};

export default function Button({
  className,
  variant = "default",
  compact = false,
  ...props
}: ButtonProps) {
  const classes = [
    styles.button,
    variant === "primary" ? styles.primary : "",
    variant === "ghost" ? styles.ghost : "",
    compact ? styles.compact : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");

  return <button className={classes} {...props} />;
}
