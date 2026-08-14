// Spinner genérico usado tanto pelo Suspense (code-splitting de páginas,
// ver App.tsx) quanto pelo ProtectedRoute enquanto GET /auth/me ainda não
// voltou — nesse segundo caso não dá pra decidir se o papel do usuário
// autoriza a rota sem esperar essa resposta.
export function PageFallback() {
  return (
    <div className="flex h-64 items-center justify-center">
      <div className="size-8 animate-spin rounded-full border-2 border-primary/30 border-t-primary" />
    </div>
  )
}
