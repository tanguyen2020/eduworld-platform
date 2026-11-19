// speech text of a formula (CONTENT_MODEL §9.1, DESIGN_SYSTEM §4.6): the text read by TTS / used as alternative text
// in the game. Built from the internal MathML tree of MathJax 4. [Proposed] speech-rule-engine (MathJax's speech) has
// no Vietnamese locale, so the portal verbalises the tree itself with a small rule set (vi, en) covering the school
// constructs (fractions, roots, powers, indices, relations, arrows, chemistry); unknown symbols are read as-is.
// Pure functions, no DOM.
import { SPEECH_TEXT_MAX } from './mathjaxConfig';

export type SpeechLanguage = 'vi' | 'en';

/** Minimal view of a MathJax MmlNode (what the verbaliser reads). */
export interface MmlLike {
  kind: string;
  childNodes: (MmlLike | null)[];
  getText?: () => string;
  attributes?: { get: (name: string) => unknown };
}

interface Words {
  frac: (n: string, d: string) => string;
  sqrt: (x: string) => string;
  root: (x: string, i: string) => string;
  pow: (b: string, e: string) => string;
  sub: (b: string, s: string) => string;
  over: (b: string, o: string) => string;
  under: (b: string, u: string) => string;
  ops: Record<string, string>;
  idents: Record<string, string>;
}

const COMMON_GREEK = ['α', 'β', 'γ', 'δ', 'Δ', 'ε', 'θ', 'λ', 'μ', 'π', 'ρ', 'σ', 'Σ', 'τ', 'φ', 'ω', 'Ω'];
const GREEK_NAMES = ['alpha', 'beta', 'gamma', 'delta', 'Delta', 'epsilon', 'theta', 'lambda', 'mu', 'pi', 'rho', 'sigma', 'Sigma', 'tau', 'phi', 'omega', 'Omega'];
const greek = Object.fromEntries(COMMON_GREEK.map((g, i) => [g, GREEK_NAMES[i]!]));

/** mhchem arrows are private-use glyphs of the mhchem font extension (U+E408…U+E42B). */
function chemArrows(forward: string, equilibrium: string): Record<string, string> {
  return { '': forward, '': equilibrium, '': equilibrium, '': equilibrium, '': equilibrium, '': equilibrium };
}

const WORDS: Record<SpeechLanguage, Words> = {
  vi: {
    frac: (n, d) => `phân số ${n} trên ${d}`,
    sqrt: (x) => `căn bậc hai của ${x}`,
    root: (x, i) => `căn bậc ${i} của ${x}`,
    pow: (b, e) => (e === '2' ? `${b} bình phương` : e === '3' ? `${b} lập phương` : e === '∘' ? `${b} độ` : `${b} mũ ${e}`),
    sub: (b, s) => `${b} chỉ số ${s}`,
    over: (b, o) => (o === '→' ? `vectơ ${b}` : o === '¯' || o === '‾' ? `đoạn ${b}` : `${b} ${o}`),
    under: (b, u) => `${b} với ${u}`,
    ops: {
      '+': 'cộng', '-': 'trừ', '−': 'trừ', '×': 'nhân', '⋅': 'nhân', '÷': 'chia', ':': 'chia', '=': 'bằng', '≠': 'khác',
      '<': 'nhỏ hơn', '>': 'lớn hơn', '≤': 'nhỏ hơn hoặc bằng', '≥': 'lớn hơn hoặc bằng', '≈': 'xấp xỉ', '±': 'cộng trừ',
      '→': 'tạo thành', '⟶': 'tạo thành', '⇌': 'thuận nghịch', '⇄': 'thuận nghịch', '⟷': 'thuận nghịch', '⇒': 'suy ra', '⇔': 'tương đương',
      '∈': 'thuộc', '∉': 'không thuộc', '⊂': 'là tập con của', '∞': 'vô cùng', '%': 'phần trăm', '∘': 'độ', '°': 'độ',
      '∠': 'góc', '△': 'tam giác', '⊥': 'vuông góc với', '∥': 'song song với', '∑': 'tổng', '∫': 'tích phân', '(': 'mở ngoặc', ')': 'đóng ngoặc',
      ',': 'phẩy', '.': 'chấm', '|': 'giá trị tuyệt đối', ...chemArrows('tạo thành', 'thuận nghịch'),
    },
    idents: { ...greek, '⁡': '' },
  },
  en: {
    frac: (n, d) => `the fraction ${n} over ${d}`,
    sqrt: (x) => `the square root of ${x}`,
    root: (x, i) => `the root of index ${i} of ${x}`,
    pow: (b, e) => (e === '2' ? `${b} squared` : e === '3' ? `${b} cubed` : e === '∘' ? `${b} degrees` : `${b} to the power ${e}`),
    sub: (b, s) => `${b} sub ${s}`,
    over: (b, o) => (o === '→' ? `vector ${b}` : o === '¯' || o === '‾' ? `segment ${b}` : `${b} ${o}`),
    under: (b, u) => `${b} with ${u}`,
    ops: {
      '+': 'plus', '-': 'minus', '−': 'minus', '×': 'times', '⋅': 'times', '÷': 'divided by', ':': 'divided by', '=': 'equals', '≠': 'is not equal to',
      '<': 'is less than', '>': 'is greater than', '≤': 'is less than or equal to', '≥': 'is greater than or equal to', '≈': 'is approximately',
      '±': 'plus or minus', '→': 'yields', '⟶': 'yields', '⇌': 'is in equilibrium with', '⇄': 'is in equilibrium with', '⟷': 'is in equilibrium with',
      '⇒': 'implies', '⇔': 'is equivalent to', '∈': 'is in', '∉': 'is not in', '⊂': 'is a subset of', '∞': 'infinity', '%': 'percent', '∘': 'degrees',
      '°': 'degrees', '∠': 'angle', '△': 'triangle', '⊥': 'is perpendicular to', '∥': 'is parallel to', '∑': 'the sum', '∫': 'the integral',
      '(': 'open paren', ')': 'close paren', ',': 'comma', '.': 'point', '|': 'absolute value', ...chemArrows('yields', 'is in equilibrium with'),
    },
    idents: { ...greek, '⁡': '' },
  },
};

const clean = (s: string) => s.replace(/\s+/g, ' ').trim();

function text(node: MmlLike): string {
  return node.getText ? node.getText() : '';
}

/** Verbalises a MathJax internal MathML node. */
export function speak(node: MmlLike | null | undefined, lang: SpeechLanguage): string {
  if (!node) return '';
  const w = WORDS[lang];
  const kids = node.childNodes.filter((c): c is MmlLike => !!c);
  const at = (i: number) => speak(kids[i], lang);
  switch (node.kind) {
    case 'mi':
    case 'mn':
    case 'mtext': {
      const t = text(node);
      return w.idents[t] ?? t;
    }
    case 'mo': {
      const t = text(node);
      return w.ops[t] ?? t;
    }
    case 'mfrac':
      return w.frac(at(0), at(1));
    case 'msqrt':
      return w.sqrt(kids.map((k) => speak(k, lang)).join(' '));
    case 'mroot':
      return w.root(at(0), at(1));
    case 'msup':
      return w.pow(at(0), rawOrSpeak(kids[1], lang));
    case 'msub':
      return w.sub(at(0), at(1));
    case 'msubsup':
      return w.pow(w.sub(at(0), at(1)), rawOrSpeak(kids[2], lang));
    case 'mover':
      return w.over(at(0), rawOrSpeak(kids[1], lang));
    case 'munder':
      return w.under(at(0), at(1));
    case 'munderover':
      return `${at(0)} ${at(1)} ${at(2)}`;
    case 'mspace':
    case 'mphantom':
    case 'none':
    case 'mprescripts':
      return '';
    default:
      return clean(kids.map((k) => speak(k, lang)).join(' '));
  }
}

/** Token text when the node is a single symbol (used to recognise ² ³ ° → ‾), else its speech. */
function rawOrSpeak(node: MmlLike | null | undefined, lang: SpeechLanguage): string {
  if (!node) return '';
  let n: MmlLike | undefined = node;
  while (n && !n.getText && n.childNodes.filter(Boolean).length === 1) n = n.childNodes.find(Boolean) ?? undefined;
  if (n?.getText && (n.kind === 'mo' || n.kind === 'mn')) return n.getText();
  return speak(node, lang);
}

/** Speech text of a formula tree, capped at the OpenAPI `speech_text.maxLength`. */
export function speechText(root: MmlLike, lang: SpeechLanguage): string {
  return clean(speak(root, lang)).slice(0, SPEECH_TEXT_MAX);
}
