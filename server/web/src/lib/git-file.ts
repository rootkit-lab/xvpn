export type GitFileGlyph =
  | 'folder'
  | 'go'
  | 'ts'
  | 'js'
  | 'md'
  | 'yaml'
  | 'json'
  | 'html'
  | 'css'
  | 'python'
  | 'shell'
  | 'image'
  | 'lock'
  | 'git'
  | 'docker'
  | 'task'
  | 'text'
  | 'file'

export function fileExt(name: string): string {
  const n = name.trim().toLowerCase()
  if (n.endsWith('.tar.gz')) return '.tar.gz'
  const i = n.lastIndexOf('.')
  return i >= 0 ? n.slice(i) : ''
}

export function isMarkdownFile(name: string): boolean {
  const ext = fileExt(name)
  return ext === '.md' || ext === '.markdown' || ext === '.mdx'
}

export function gitFileGlyph(name: string, type: string): GitFileGlyph {
  if (type === 'tree') {
    return 'folder'
  }
  const lower = name.trim().toLowerCase()
  if (lower === 'dockerfile' || lower.startsWith('dockerfile.')) {
    return 'docker'
  }
  if (lower === 'makefile' || lower === 'taskfile.yml' || lower === 'taskfile.yaml') {
    return 'task'
  }
  if (lower === '.gitignore' || lower === '.gitattributes' || lower === '.gitmodules') {
    return 'git'
  }
  if (lower === 'go.mod' || lower === 'go.sum' || lower === 'go.work') {
    return 'go'
  }
  const ext = fileExt(name)
  switch (ext) {
    case '.go':
      return 'go'
    case '.ts':
    case '.tsx':
      return 'ts'
    case '.js':
    case '.jsx':
    case '.mjs':
    case '.cjs':
      return 'js'
    case '.md':
    case '.markdown':
    case '.mdx':
      return 'md'
    case '.yml':
    case '.yaml':
      return 'yaml'
    case '.json':
      return 'json'
    case '.html':
    case '.htm':
      return 'html'
    case '.css':
    case '.scss':
    case '.sass':
    case '.less':
      return 'css'
    case '.py':
      return 'python'
    case '.sh':
    case '.bash':
    case '.zsh':
      return 'shell'
    case '.png':
    case '.jpg':
    case '.jpeg':
    case '.gif':
    case '.svg':
    case '.webp':
    case '.ico':
      return 'image'
    case '.lock':
      return 'lock'
    case '.txt':
    case '.log':
      return 'text'
    default:
      return 'file'
  }
}
