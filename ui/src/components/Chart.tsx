import { useEffect, useRef } from "react";
import uPlot from "uplot";
import styles from "./Chart.module.css";

type ChartProps = {
  data: uPlot.AlignedData;
  options: Omit<uPlot.Options, "width">;
  className?: string;
  small?: boolean;
};

export default function Chart({ data, options, className, small = false }: ChartProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return undefined;
    }
    container.innerHTML = "";
    const width = container.clientWidth || 520;
    const plot = new uPlot({ ...options, width }, data, container);
    return () => {
      plot.destroy();
    };
  }, [data, options]);

  const classes = [styles.chart, small ? styles.small : "", className ?? ""]
    .filter(Boolean)
    .join(" ");

  return <div className={classes} ref={containerRef} />;
}
