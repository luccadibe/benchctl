import type { AnchorHTMLAttributes, PropsWithChildren } from "react";
import { navigate } from "./router";

type LinkProps = PropsWithChildren<
  AnchorHTMLAttributes<HTMLAnchorElement> & {
    to: string;
  }
>;

export default function Link({ to, onClick, children, ...props }: LinkProps) {
  return (
    <a
      {...props}
      href={to}
      onClick={(event) => {
        if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
          return;
        }
        event.preventDefault();
        onClick?.(event);
        navigate(to);
      }}
    >
      {children}
    </a>
  );
}
