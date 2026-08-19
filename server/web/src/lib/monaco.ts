import { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'
import editorWorker from 'monaco-editor/editor/editor.worker?worker'
import cssWorker from 'monaco-editor/language/css/css.worker?worker'
import htmlWorker from 'monaco-editor/language/html/html.worker?worker'
import jsonWorker from 'monaco-editor/language/json/json.worker?worker'
import tsWorker from 'monaco-editor/language/typescript/ts.worker?worker'

self.MonacoEnvironment = {
  getWorker(_workerId: string, label: string) {
    if (label === 'json') return new jsonWorker()
    if (label === 'css' || label === 'scss' || label === 'less') return new cssWorker()
    if (label === 'html' || label === 'handlebars' || label === 'razor') return new htmlWorker()
    if (label === 'typescript' || label === 'javascript') return new tsWorker()
    return new editorWorker()
  },
}

loader.config({ monaco })

const LANG: Record<string, string> = {
  go: 'go',
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  json: 'json',
  md: 'markdown',
  mdx: 'markdown',
  yml: 'yaml',
  yaml: 'yaml',
  scss: 'scss',
  css: 'css',
  html: 'html',
  htm: 'html',
  sh: 'shell',
  bash: 'shell',
  py: 'python',
  rs: 'rust',
  toml: 'ini',
  ini: 'ini',
  xml: 'xml',
  svg: 'xml',
  sql: 'sql',
  dockerfile: 'dockerfile',
}

export function languageForPath(path: string): string {
  const base = path.split('/').pop()?.toLowerCase() ?? ''
  if (base === 'dockerfile' || base === 'makefile') return base
  const ext = base.includes('.') ? base.slice(base.lastIndexOf('.') + 1) : ''
  return LANG[ext] || 'plaintext'
}

export function defineIhuullTheme(): string {
  monaco.editor.defineTheme('ihuull-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [],
    colors: {
      'editor.background': '#14161c',
      'editor.foreground': '#f4f6fa',
      'editorLineNumber.foreground': '#6b7280',
      'editorCursor.foreground': '#4eb8e8',
      'editor.selectionBackground': '#4eb8e833',
      'editor.lineHighlightBackground': '#1c1f28',
    },
  })
  return 'ihuull-dark'
}
