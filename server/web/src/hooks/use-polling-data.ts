import { useCallback, useEffect, useState } from 'react'
import { ApiError } from '@/lib/api'

interface PollingState<T> {
  data: T | null
  error: string | null
  loading: boolean
  reload: () => void
}

// usePollingData busca `fetcher` ao montar e a cada `intervalMs` — usado nas
// telas que exibem estado "ao vivo" da interface WireGuard (dashboard,
// dispositivos), onde os dados podem mudar sem nenhuma ação do admin.
export function usePollingData<T>(fetcher: () => Promise<T>, intervalMs = 10_000): PollingState<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [reloadToken, setReloadToken] = useState(0)

  const reload = useCallback(() => setReloadToken((t) => t + 1), [])

  useEffect(() => {
    let cancelled = false

    async function run() {
      try {
        const result = await fetcher()
        if (!cancelled) {
          setData(result)
          setError(null)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Falha ao carregar dados')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    run()
    const id = setInterval(run, intervalMs)
    return () => {
      cancelled = true
      clearInterval(id)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reloadToken, intervalMs])

  return { data, error, loading, reload }
}
