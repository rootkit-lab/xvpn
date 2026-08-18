#!/usr/bin/env node
// Gera themes/ihuull-dark.json a partir de $dark em _color-system.scss.
// Não copie oklch/hex à mão — rode: node shared/vscode-theme/gen.mjs
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const scssPath = join(here, '../ui/scss/_color-system.scss')
const outPath = join(here, 'themes/ihuull-dark.json')

function parseDarkMap(scss) {
  const start = scss.indexOf('$dark: (')
  if (start < 0) throw new Error('$dark não encontrado')
  const body = scss.slice(start + '$dark: ('.length)
  const end = body.indexOf('\n);')
  if (end < 0) throw new Error('$dark sem fechamento')
  const map = {}
  for (const line of body.slice(0, end).split('\n')) {
    const trimmed = line.replace(/\/\/.*$/, '').trim()
    const m = trimmed.match(/^([a-z0-9-]+):\s*(oklch\([^)]+\))\s*,?\s*$/)
    if (m) map[m[1]] = m[2]
  }
  if (!map.background || !map.primary) throw new Error('$dark incompleto')
  return map
}

function parseOklch(raw) {
  const m = raw.match(/^oklch\(\s*([0-9.]+)\s+([0-9.]+)\s+([0-9.]+)(?:\s*\/\s*([0-9.]+)%?)?\s*\)$/)
  if (!m) throw new Error(`oklch inválido: ${raw}`)
  let alpha = m[4] === undefined ? 1 : Number(m[4])
  if (m[4] !== undefined && raw.includes('%')) alpha = alpha / 100
  return { L: Number(m[1]), C: Number(m[2]), h: Number(m[3]), a: alpha }
}

function oklchToSrgb({ L, C, h }) {
  const hr = (h * Math.PI) / 180
  const a = C * Math.cos(hr)
  const b = C * Math.sin(hr)
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b
  const s_ = L - 0.0894841775 * a - 1.291485548 * b
  const l = l_ ** 3
  const m = m_ ** 3
  const s = s_ ** 3
  return {
    r: +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    g: -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    b: -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  }
}

function toGamma(c) {
  const x = Math.min(1, Math.max(0, c))
  return x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055
}

function hexByte(n) {
  return Math.round(n * 255)
    .toString(16)
    .padStart(2, '0')
}

function oklchToHex(raw) {
  const p = parseOklch(raw)
  const rgb = oklchToSrgb(p)
  const hex = `#${hexByte(toGamma(rgb.r))}${hexByte(toGamma(rgb.g))}${hexByte(toGamma(rgb.b))}`
  if (p.a >= 1) return hex
  return hex + hexByte(p.a)
}

function themeFromDark(dark) {
  const c = (name) => oklchToHex(dark[name])
  return {
    name: 'ihuull Dark',
    type: 'dark',
    colors: {
      foreground: c('foreground'),
      descriptionForeground: c('muted-foreground'),
      errorForeground: c('destructive'),
      focusBorder: c('ring'),
      'editor.background': c('background'),
      'editor.foreground': c('foreground'),
      'editor.lineHighlightBackground': c('muted'),
      'editor.selectionBackground': c('secondary'),
      'editorCursor.foreground': c('primary'),
      'editorLineNumber.foreground': c('muted-foreground'),
      'editorLineNumber.activeForeground': c('foreground'),
      'editorWidget.background': c('card'),
      'editorWidget.border': c('border'),
      'sideBar.background': c('card'),
      'sideBar.foreground': c('foreground'),
      'sideBar.border': c('border'),
      'sideBarTitle.foreground': c('muted-foreground'),
      'activityBar.background': c('background'),
      'activityBar.foreground': c('primary'),
      'activityBar.inactiveForeground': c('muted-foreground'),
      'activityBar.border': c('border'),
      'activityBarBadge.background': c('primary'),
      'activityBarBadge.foreground': c('primary-foreground'),
      'statusBar.background': c('card'),
      'statusBar.foreground': c('foreground'),
      'statusBar.noFolderBackground': c('card'),
      'statusBar.debuggingBackground': c('destructive'),
      'titleBar.activeBackground': c('background'),
      'titleBar.activeForeground': c('foreground'),
      'titleBar.inactiveBackground': c('background'),
      'titleBar.inactiveForeground': c('muted-foreground'),
      'titleBar.border': c('border'),
      'tab.activeBackground': c('card'),
      'tab.inactiveBackground': c('background'),
      'tab.activeForeground': c('foreground'),
      'tab.inactiveForeground': c('muted-foreground'),
      'tab.border': c('border'),
      'tab.activeBorderTop': c('primary'),
      'editorGroupHeader.tabsBackground': c('background'),
      'panel.background': c('card'),
      'panel.border': c('border'),
      'terminal.background': c('background'),
      'terminal.foreground': c('foreground'),
      'input.background': c('input'),
      'input.foreground': c('foreground'),
      'input.border': c('border'),
      'dropdown.background': c('popover'),
      'dropdown.foreground': c('popover-foreground'),
      'list.activeSelectionBackground': c('secondary'),
      'list.activeSelectionForeground': c('secondary-foreground'),
      'list.hoverBackground': c('muted'),
      'list.inactiveSelectionBackground': c('muted'),
      'button.background': c('primary'),
      'button.foreground': c('primary-foreground'),
      'button.hoverBackground': c('glow'),
      'badge.background': c('primary'),
      'badge.foreground': c('primary-foreground'),
      'scrollbarSlider.background': c('border'),
      'gitDecoration.modifiedResourceForeground': c('glow-amber'),
      'gitDecoration.untrackedResourceForeground': c('safe'),
      'gitDecoration.deletedResourceForeground': c('destructive'),
      'peekViewEditor.background': c('card'),
      'welcomePage.background': c('background'),
    },
    tokenColors: [
      { scope: ['comment'], settings: { foreground: c('muted-foreground') } },
      { scope: ['keyword', 'storage'], settings: { foreground: c('primary') } },
      { scope: ['string'], settings: { foreground: c('safe') } },
      { scope: ['constant.numeric'], settings: { foreground: c('glow-amber') } },
      { scope: ['invalid'], settings: { foreground: c('destructive') } },
    ],
  }
}

const scss = readFileSync(scssPath, 'utf8')
const dark = parseDarkMap(scss)
const theme = themeFromDark(dark)
const json = `${JSON.stringify(theme, null, 2)}\n`

if (process.argv.includes('--check')) {
  let current = ''
  try {
    current = readFileSync(outPath, 'utf8')
  } catch {
    current = ''
  }
  if (current !== json) {
    console.error('ihuull-dark.json desatualizado — rode: node shared/vscode-theme/gen.mjs')
    process.exit(1)
  }
  process.exit(0)
}

mkdirSync(dirname(outPath), { recursive: true })
writeFileSync(outPath, json)
console.log(`wrote ${outPath}`)
