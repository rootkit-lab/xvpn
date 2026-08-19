import { describe, expect, it } from 'vitest'
import { archiveExtractable, driverFileKind, driverOpenMode, fileSuffix } from './driver-kind'

describe('driver-kind', () => {
  it('reconhece texto, mídia e compactados', () => {
    expect(driverFileKind('notas.txt', false)).toBe('text')
    expect(driverFileKind('foto.PNG', false)).toBe('image')
    expect(driverFileKind('clip.webm', false)).toBe('video')
    expect(driverFileKind('docs.pdf', false)).toBe('pdf')
    expect(driverFileKind('pacote.tar.gz', false)).toBe('archive')
    expect(driverFileKind('pasta', true)).toBe('folder')
    expect(fileSuffix('a.TAR.GZ')).toBe('.tar.gz')
  })

  it('só zip/tgz extraem; abrir decide editar vs ver', () => {
    expect(archiveExtractable('a.zip')).toBe(true)
    expect(archiveExtractable('a.rar')).toBe(false)
    expect(driverOpenMode('text')).toBe('edit')
    expect(driverOpenMode('image')).toBe('view')
    expect(driverOpenMode('archive')).toBeNull()
  })
})
