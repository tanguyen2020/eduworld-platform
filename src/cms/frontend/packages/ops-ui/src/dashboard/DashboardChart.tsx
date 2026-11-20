// DashboardChart (F056, DESIGN_SYSTEM §4.4, SCHOOL_LMS §8.7): the one chart library of the Admin UI (ECharts, loaded on
// demand so it stays out of the portal entry chunk) for column and line series — score distribution, 8-week trend,
// active minutes. Series color from the Admin token (`colorPrimary`), value axis starting at 0, values in the caller's
// format; the container is an image with an aria-label summarising every value (the caller builds it from server data),
// so the chart is never the only way to read the numbers. Rendering failures (e.g. no canvas) leave the label in place.
import { theme } from 'antd';
import { useEffect, useRef } from 'react';

export interface DashboardChartProps {
  kind: 'bar' | 'line';
  categories: string[];
  values: (number | null)[];
  /** Full textual summary of the data (read by screen readers). */
  ariaLabel: string;
  testId?: string;
}

type EChartsModule = typeof import('echarts/core');
let loading: Promise<EChartsModule> | null = null;

/** Loads the tree-shaken ECharts build once (bar, line, grid, tooltip, aria, SVG renderer). */
function loadECharts(): Promise<EChartsModule> {
  loading ??= Promise.all([import('echarts/core'), import('echarts/charts'), import('echarts/components'), import('echarts/renderers')]).then(([core, charts, components, renderers]) => {
    core.use([charts.BarChart, charts.LineChart, components.GridComponent, components.TooltipComponent, components.AriaComponent, renderers.SVGRenderer]);
    return core;
  });
  return loading;
}

export function DashboardChart({ kind, categories, values, ariaLabel, testId }: DashboardChartProps) {
  const { token } = theme.useToken();
  const ref = useRef<HTMLDivElement>(null);
  const height = token.controlHeightLG * 6;
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    let disposed = false;
    let chart: { setOption: (o: unknown) => void; resize: () => void; dispose: () => void } | null = null;
    let observer: ResizeObserver | null = null;
    loadECharts()
      .then((echarts) => {
        if (disposed) return;
        chart = echarts.init(el, undefined, { renderer: 'svg', height });
        chart.setOption({
          aria: { enabled: false },
          color: [token.colorPrimary],
          textStyle: { color: token.colorText, fontFamily: token.fontFamily },
          grid: { left: token.marginLG, right: token.marginLG, top: token.margin, bottom: token.marginLG, containLabel: true },
          tooltip: { trigger: 'axis' },
          xAxis: { type: 'category', data: categories, axisLabel: { color: token.colorTextSecondary } },
          yAxis: { type: 'value', min: 0, axisLabel: { color: token.colorTextSecondary }, splitLine: { lineStyle: { color: token.colorSplit } } },
          series: [{ type: kind, data: values }],
        });
        if (typeof ResizeObserver !== 'undefined') {
          observer = new ResizeObserver(() => chart?.resize());
          observer.observe(el);
        }
      })
      .catch((e: unknown) => console.warn('DashboardChart: rendering skipped', e));
    return () => {
      disposed = true;
      observer?.disconnect();
      chart?.dispose();
    };
  }, [kind, categories, values, height, token]);
  return <div ref={ref} role="img" aria-label={ariaLabel} data-testid={testId} style={{ width: '100%', minHeight: height }} />;
}
