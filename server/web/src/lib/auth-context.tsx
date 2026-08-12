import { createContext, use, useCallback, useMemo, useState, type ReactNode } from 'react'
import { api, clearToken, getToken, setToken } from '@/lib/api'

interface AuthContextValue {
  isAuthenticated: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(() => getToken() !== null)

  const login = useCallback(async (username: string, password: string) => {
    const { token } = await api.login(username, password)
    setToken(token)
    setIsAuthenticated(true)
  }, [])

  const logout = useCallback(() => {
    clearToken()
    setIsAuthenticated(false)
  }, [])

  const value = useMemo(() => ({ isAuthenticated, login, logout }), [isAuthenticated, login, logout])

  return <AuthContext value={value}>{children}</AuthContext>
}

export function useAuth(): AuthContextValue {
  const ctx = use(AuthContext)
  if (!ctx) {
    throw new Error('useAuth deve ser usado dentro de um AuthProvider')
  }
  return ctx
}
