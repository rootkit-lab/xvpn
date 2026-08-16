import { createContext, use, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, clearToken, setToken, type User } from '@/lib/api'

interface AuthContextValue {
  isAuthenticated: boolean
  // user só é null enquanto isLoadingUser é true (sonda /auth/me) ou se
  // essa chamada falhou por algum motivo além de 401 — nesse caso a UI
  // deve tratar como "papel desconhecido" e não mostrar nada sensível.
  user: User | null
  isLoadingUser: boolean
  login: (username: string, password: string) => Promise<User>
  logout: () => Promise<void>
  refreshUser: () => Promise<User>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [user, setUser] = useState<User | null>(null)
  const [isLoadingUser, setIsLoadingUser] = useState(true)

  useEffect(() => {
    let cancelled = false
    setIsLoadingUser(true)
    api
      .me()
      .then((u) => {
        if (cancelled) return
        setUser(u)
        setIsAuthenticated(true)
      })
      .catch(() => {
        if (cancelled) return
        setUser(null)
        setIsAuthenticated(false)
      })
      .finally(() => {
        if (!cancelled) setIsLoadingUser(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const { token, user: loggedInUser } = await api.login(username, password)
    if (token) setToken(token)
    setUser(loggedInUser)
    setIsLoadingUser(false)
    setIsAuthenticated(true)
    return loggedInUser
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      // cookie já pode ter expirado
    }
    clearToken()
    setUser(null)
    setIsAuthenticated(false)
    setIsLoadingUser(false)
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
