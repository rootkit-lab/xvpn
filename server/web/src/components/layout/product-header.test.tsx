import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ProductHeader } from '@xvpn/ui/react/product-header'

afterEach(cleanup)

describe('ProductHeader', () => {
  it('mostra a marca ihuull, o produto e o slot direito', () => {
    render(
      <ProductHeader product="marketplace" trailing={<button type="button">Sair</button>}>
        <span>buscar</span>
      </ProductHeader>,
    )
    const header = screen.getByRole('banner')
    expect(header).toHaveAttribute('data-product', 'marketplace')
    expect(within(header).getByRole('link', { name: 'ihuull' })).toBeInTheDocument()
    expect(within(header).getByRole('link', { name: 'marketplace' })).toBeInTheDocument()
    expect(within(header).getByText('buscar')).toBeInTheDocument()
    expect(within(header).getByRole('button', { name: 'Sair' })).toBeInTheDocument()
  })

  it('omite o bloco de produto na landing da marca', () => {
    render(<ProductHeader product="ihuull" />)
    const header = screen.getByRole('banner')
    expect(header).toHaveAttribute('data-product', 'ihuull')
    expect(within(header).getByRole('link', { name: 'ihuull' })).toBeInTheDocument()
    expect(within(header).queryByText('plataforma')).not.toBeInTheDocument()
  })
})
