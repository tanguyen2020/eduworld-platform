// Virtual keyboard of MathInput (F053-AC-16, ADR-0055, ADMIN_PORTAL §1.4): three tabs Toán / Lý / Hóa with school
// constructs — fraction, root, power, index, matrix, `\ce{}` chemistry, `\pu{}` units. Keycaps insert LaTeX
// (`#@` = selection, `#?` / `#0` = placeholder). Labels are translated by the caller. Pure data.

export type KeyboardTab = 'math' | 'physics' | 'chemistry';

export interface KeycapSpec {
  latex?: string;
  insert?: string;
  label?: string;
}

export interface KeyboardLayoutSpec {
  id: string;
  label: string;
  tooltip: string;
  rows: (string | KeycapSpec)[][];
}

const ROWS: Record<KeyboardTab, (string | KeycapSpec)[][]> = {
  math: [
    ['7', '8', '9', { latex: '\\frac{#@}{#?}' }, { latex: '#@^{#?}' }, { latex: '#@_{#?}' }, { latex: '\\sqrt{#0}' }, { latex: '\\sqrt[#?]{#0}' }],
    ['4', '5', '6', '\\times', '\\div', '+', '-', '=', '\\neq'],
    ['1', '2', '3', '<', '>', '\\leq', '\\geq', '\\approx', '\\pm'],
    ['0', ',', '\\pi', '\\infty', '\\%', { latex: '#@^{\\circ}' }, '\\angle', '\\triangle', '\\perp', '\\parallel'],
    [{ latex: '\\begin{pmatrix}#0 & #? \\\\ #? & #?\\end{pmatrix}' }, { latex: '\\left(#0\\right)' }, { latex: '\\left|#0\\right|' }, { latex: '\\overline{#0}' }, { latex: '\\overrightarrow{#0}' }, '\\in', '\\notin', '\\subset'],
  ],
  physics: [
    [{ latex: '\\vec{#0}' }, '\\Delta', '\\alpha', '\\beta', '\\lambda', '\\mu', '\\rho', '\\omega', '\\Omega'],
    [{ latex: '\\frac{#@}{#?}' }, { latex: '#@^{#?}' }, { latex: '#@_{#?}' }, '\\cdot', '\\times', '=', '\\approx', '\\propto'],
    [{ latex: '\\pu{#0 m/s}', label: 'm/s' }, { latex: '\\pu{#0 m/s^2}', label: 'm/s²' }, { latex: '\\pu{#0 N}', label: 'N' }, { latex: '\\pu{#0 J}', label: 'J' }, { latex: '\\pu{#0 W}', label: 'W' }, { latex: '\\pu{#0 \\Omega}', label: 'Ω' }, { latex: '\\pu{#0 V}', label: 'V' }, { latex: '\\pu{#0 A}', label: 'A' }],
    [{ latex: '\\pu{#0 kg}', label: 'kg' }, { latex: '\\pu{#0 m}', label: 'm' }, { latex: '\\pu{#0 s}', label: 's' }, { latex: '\\pu{#0 Pa}', label: 'Pa' }, { latex: '\\pu{#0 Hz}', label: 'Hz' }, { latex: '#@^{\\circ}\\mathrm{C}', label: '°C' }],
  ],
  chemistry: [
    [{ latex: '\\ce{#0}', label: '\\ce{}' }, { insert: '\\ce{->}', label: '→' }, { insert: '\\ce{<=>}', label: '⇌' }, { insert: '\\ce{^}', label: '↑' }, { insert: '\\ce{v}', label: '↓' }, { latex: '\\ce{#0^{2+}}', label: 'ion' }],
    [{ insert: '\\ce{H2O}', label: 'H₂O' }, { insert: '\\ce{CO2}', label: 'CO₂' }, { insert: '\\ce{O2}', label: 'O₂' }, { insert: '\\ce{H2}', label: 'H₂' }, { insert: '\\ce{NaCl}', label: 'NaCl' }, { insert: '\\ce{H2SO4}', label: 'H₂SO₄' }],
    [{ latex: '#@_{#?}' }, { latex: '#@^{#?}' }, '+', '=', { latex: '\\pu{#0 mol}', label: 'mol' }, { latex: '\\pu{#0 g/mol}', label: 'g/mol' }, { latex: '\\pu{#0 mol/L}', label: 'mol/L' }],
  ],
};

export const KEYBOARD_TABS: readonly KeyboardTab[] = ['math', 'physics', 'chemistry'];

/** MathLive layouts for the three tabs; `t` returns the translated tab label / tooltip. */
export function keyboardLayouts(t: (key: string) => string): KeyboardLayoutSpec[] {
  return KEYBOARD_TABS.map((tab) => ({ id: `ew-${tab}`, label: t(`math.keyboard.${tab}`), tooltip: t(`math.keyboard.${tab}Tooltip`), rows: ROWS[tab] }));
}
