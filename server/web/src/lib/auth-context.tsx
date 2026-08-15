import { createContext, use, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, clearToken, getToken, setToken, type User } from '@/lib/api'

interface AuthContextValue {
  isAuthenticated: boolean
  // user só é null enquanto isLoadingUser é true (token existe, mas
  // /auth/me ainda não voltou) ou se essa chamada falhou por algum motivo
  // além de 401 (que já limpa o token via lib/api.ts) — nesse caso a UI
  // deve tratar como "papel desconhecido" e não mostrar nada sensível.
  user: User | null
  isLoadingUser: boolean
  login: (username: string, password: string) => Promise<User>
  logout: () => void
  refreshUser: () => Promise<User>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(() => getToken() !== null)
  const [user, setUser] = useState<User | null>(null)
  const [isLoadingUser, setIsLoadingUser] = useState(() => getToken() !== null)

  useEffect(() => {
    if (!isAuthenticated) {
      setUser(null)
      setIsLoadingUser(false)
      return
    }
    let cancelled = false
    setIsLoadingUser(true)
    api
      .me()
      .then((u) => {
        if (!cancelled) setUser(u)
      })
      .catch(() => {
        // 401 já é tratado globalmente em lib/api.ts (limpa token e manda
        // pro /login); qualquer outro erro só deixa user null, escondendo
        // navegação sensível até a próxima tentativa.
      })
      .finally(() => {
        if (!cancelled) setIsLoadingUser(false)
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated])

  const login = useCallback(async (username: string, password: string) => {
    const { token, user: loggedInUser } = await api.login(username, password)
    setToken(token)
    setUser(loggedInUser)
    setIsLoadingUser(false)
    setIsAuthenticated(true)
    return loggedInUser
  }, [])

  const logout = useCallback(() => {
    clearToken()
    setUser(null)
    setIsAuthenticated(false)
  }, [])

  const refreshUser = useCallback(async () => {
    const u = await api.me()
    setUser(u)
    return u
  }, [])

  const value = useMemo(
    () => ({ isAuthenticated, user, isLoadingUser, login, logout, refreshUser }),
    [isAuthenticated, user, isLoadingUser, login, logout, refreshUser],
  )

  return <AuthContext value={value}>{children}</AuthContext>
}

export function useAuth(): AuthContextValue {
  const ctx = use(AuthContext)
  if (!ctx) {
    throw new Error('useAuth deve ser usado dentro de um AuthProvider')
  }
  return ctx
}
