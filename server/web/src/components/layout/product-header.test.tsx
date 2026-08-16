import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { productDisplayName } from '@xvpn/ui/react/products'

afterEach(cleanup)

describe('ProductHeader', () => {
  it('mostra só o app e o slot direito — sem wordmark ihuull', () => {
    render(<ProductHeader product="marketplace" trailing={<button type="button">Sair</button>} />)
    const header = screen.getByRole('banner')
    expect(header).toHaveAttribute('data-product', 'marketplace')
    expect(within(header).getByRole('link', { name: 'Marketplace Store' })).toBeInTheDocument()
    expect(within(header).getByText('Marketplace')).toBeInTheDocument()
    expect(within(header).getByText('Store')).toBeInTheDocument()
    expect(within(header).queryByRole('link', { name: 'ihuull' })).not.toBeInTheDocument()
    expect(within(header).queryByText('buscar')).not.toBeInTheDocument()
    expect(within(header).getByRole('button', { name: 'Sair' })).toBeInTheDocument()
  })

  it('usa a convenção de vitrine XVPN/XCHAT Client, XGROUP Social', () => {
    expect(productDisplayName('xvpn')).toBe('XVPN Client')
    expect(productDisplayName('xchat')).toBe('XCHAT Client')
    expect(productDisplayName('xgroup')).toBe('XGROUP Social')
    expect(productDisplayName('xdriver')).toBe('XDRIVER Drive')
    expect(productDisplayName('marketplace')).toBe('Marketplace Store')
  })

  it('na landing da marca mostra ihuull sem kicker de produto', () => {
    render(<ProductHeader product="ihuull" />)
    const header = screen.getByRole('banner')
    expect(header).toHaveAttribute('data-product', 'ihuull')
    expect(within(header).getByRole('link', { name: 'ihuull' })).toBeInTheDocument()
    expect(within(header).queryByText('plataforma')).not.toBeInTheDocument()
  })
})
