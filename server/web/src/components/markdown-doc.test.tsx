import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { MarkdownPreview } from './markdown-doc'

afterEach(cleanup)

describe('MarkdownPreview', () => {
  it('renderiza título, link, tabela e não deixa HTML cru', () => {
    render(
      <MarkdownPreview
        text={`# xvpn-client

Cliente em [\`PLAN.md\`](../PLAN.md).

## Arquitetura

| Plataforma | Motor |
|---|---|
| Linux | kernel |

\`\`\`bash
task build
\`\`\`
`}
      />,
    )
    expect(screen.getByRole('heading', { level: 1, name: 'xvpn-client' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Arquitetura' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /PLAN.md/ })).toHaveAttribute('href', '../PLAN.md')
    expect(screen.getByRole('columnheader', { name: 'Plataforma' })).toBeInTheDocument()
    expect(screen.getByText('task build')).toBeInTheDocument()
    expect(screen.queryByText('# xvpn-client')).not.toBeInTheDocument()
  })

  it('não executa javascript: em links', () => {
    const { container } = render(<MarkdownPreview text={`[x](javascript:alert(1))`} />)
    const el = container.querySelector('a')
    expect(el).toBeTruthy()
    expect(el?.getAttribute('href') ?? '').not.toMatch(/javascript/i)
  })
})
