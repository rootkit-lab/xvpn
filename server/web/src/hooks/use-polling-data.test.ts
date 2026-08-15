import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { usePollingData } from '@/hooks/use-polling-data'
import { ApiError } from '@/lib/api'

afterEach(() => {
  vi.useRealTimers()
})

describe('usePollingData', () => {
  it('busca no mount e expõe data', async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true })
    const { result } = renderHook(() => usePollingData(fetcher, 60_000))

    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toEqual({ ok: true })
    expect(result.current.error).toBeNull()
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('não sobrepõe fetches enquanto inFlight', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    let calls = 0
    const fetcher = vi.fn(async () => {
      calls += 1
      if (calls === 1) {
        await gate
        return 'primeiro'
      }
      return 'segundo'
    })

    const { result } = renderHook(() => usePollingData(fetcher, 20))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))

    await act(async () => {
      await new Promise((r) => setTimeout(r, 60))
    })
    expect(fetcher).toHaveBeenCalledTimes(1)

    await act(async () => {
      release()
    })
    await waitFor(() => expect(result.current.data).toBe('primeiro'))
  })

  it('expõe mensagem de ApiError', async () => {
    const fetcher = vi.fn().mockRejectedValue(new ApiError(500, 'boom'))
    const { result } = renderHook(() => usePollingData(fetcher, 60_000))
    await waitFor(() => expect(result.current.error).toBe('boom'))
    expect(result.current.data).toBeNull()
  })
})
