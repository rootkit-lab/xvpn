// Spinner genérico usado tanto pelo Suspense (code-splitting de páginas,
// ver App.tsx) quanto pelo ProtectedRoute enquanto GET /auth/me ainda não
// voltou — nesse segundo caso não dá pra decidir se o papel do usuário
// autoriza a rota sem esperar essa resposta.
export function PageFallback({ label }: { label?: string }) {
  return (
    <div className="flex h-64 flex-col items-center justify-center gap-3">
      <div className="size-8 animate-spin rounded-full border-2 border-primary/30 border-t-primary" />
      {label ? <p className="text-sm text-muted-foreground">{label}</p> : null}
    </div>
  )
}
