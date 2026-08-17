import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { usePollingData } from '@/hooks/use-polling-data'
import { ApiError } from '@/lib/api'

describe('usePollingData', () => {
  it('busca no mount e expõe data', async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true })
    const { result, unmount } = renderHook(() => usePollingData(fetcher, 60_000))

    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toEqual({ ok: true })
    expect(result.current.error).toBeNull()
    expect(fetcher).toHaveBeenCalledTimes(1)
    unmount()
  })

  it('expõe mensagem de ApiError', async () => {
    const fetcher = vi.fn().mockRejectedValue(new ApiError(500, 'boom'))
    const { result, unmount } = renderHook(() => usePollingData(fetcher, 60_000))
    await waitFor(() => expect(result.current.error).toBe('boom'))
    expect(result.current.data).toBeNull()
    unmount()
  })

  it('não busca quando enabled é false', async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true })
    const { result, unmount } = renderHook(() => usePollingData(fetcher, 60_000, false))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(fetcher).not.toHaveBeenCalled()
    expect(result.current.data).toBeNull()
    unmount()
  })

  it('reload dispara nova busca', async () => {
    const fetcher = vi.fn().mockResolvedValueOnce('a').mockResolvedValueOnce('b')
    const { result, unmount } = renderHook(() => usePollingData(fetcher, 60_000))
    await waitFor(() => expect(result.current.data).toBe('a'))
    result.current.reload()
    await waitFor(() => expect(result.current.data).toBe('b'))
    expect(fetcher).toHaveBeenCalledTimes(2)
    unmount()
  })
})
