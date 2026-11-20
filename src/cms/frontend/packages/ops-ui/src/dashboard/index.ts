// @eduworld/ops-ui/dashboard — shared dashboard components (F056; reused by F080/F081). Exported from a sub-path so the
// chart library (ECharts, loaded on demand by DashboardChart) never enters the portal entry chunk. No API calls here.
export { createDashboardQueue, DashboardQueueProvider, useDashboardQueue } from './dashboardQueue';
export type { DashboardQueue } from './dashboardQueue';
export { DashboardWidget, DashboardMetric, formatMetric, DASHBOARD_HINT_CODES } from './DashboardWidget';
export type { DashboardGrain, DashboardMetaView, DashboardWidgetError, DashboardWidgetProps, DashboardMetricProps, MetricFormat } from './DashboardWidget';
export { DashboardRangePicker, RANGE_PRESETS, allowedGrains, isDateDisabled, minGrainFor, presetRange, rangeDays } from './DashboardRangePicker';
export type { CalendarPeriod, DashboardRange, DashboardRangeLimits, DashboardRangePickerProps, RangePreset } from './DashboardRangePicker';
export { SuppressedCell } from './SuppressedCell';
export { MasteryHeatmap, MASTERY_LEVELS, MASTERY_SYMBOL, levelColors } from './MasteryHeatmap';
export type { HeatmapCell, HeatmapRow, MasteryHeatmapProps, MasteryLevel } from './MasteryHeatmap';
export { ExportButton, EXPORT_FAILURE_CODES, linkUsable } from './ExportButton';
export type { ExportButtonProps, ExportJobStatus, ExportJobView } from './ExportButton';
export { DashboardChart } from './DashboardChart';
export type { DashboardChartProps } from './DashboardChart';
