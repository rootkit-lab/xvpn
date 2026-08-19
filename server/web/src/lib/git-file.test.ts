import { describe, expect, it } from 'vitest'
import { fileExt, gitFileGlyph, isMarkdownFile } from './git-file'

describe('git-file', () => {
  it('reconhece markdown e extensões', () => {
    expect(isMarkdownFile('README.md')).toBe(true)
    expect(isMarkdownFile('notes.markdown')).toBe(true)
    expect(isMarkdownFile('page.mdx')).toBe(true)
    expect(isMarkdownFile('main.go')).toBe(false)
    expect(fileExt('a.TAR.GZ')).toBe('.tar.gz')
  })

  it('escolhe o glifo pelo tipo e pelo nome', () => {
    expect(gitFileGlyph('frontend', 'tree')).toBe('folder')
    expect(gitFileGlyph('main.go', 'blob')).toBe('go')
    expect(gitFileGlyph('go.mod', 'blob')).toBe('go')
    expect(gitFileGlyph('App.tsx', 'blob')).toBe('ts')
    expect(gitFileGlyph('marketplace.yaml', 'blob')).toBe('yaml')
    expect(gitFileGlyph('Taskfile.yml', 'blob')).toBe('task')
    expect(gitFileGlyph('README.md', 'blob')).toBe('md')
    expect(gitFileGlyph('.gitignore', 'blob')).toBe('git')
    expect(gitFileGlyph('Dockerfile', 'blob')).toBe('docker')
    expect(gitFileGlyph('photo.PNG', 'blob')).toBe('image')
    expect(gitFileGlyph('mystery.bin', 'blob')).toBe('file')
  })
})
