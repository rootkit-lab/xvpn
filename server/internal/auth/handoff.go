package auth

import (
	"fmt"
	"html"
)

// HandoffContinueHTML is a top-level auto-POST so "Continuar como" never
// puts the JWE in JSON/localStorage (cookie HttpOnly stays isolated).
func HandoffContinueHTML(action, token, dest string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html lang="pt-BR"><head><meta charset="utf-8"><title>ihuull</title></head><body>
<form id="f" method="post" action="%s">
<input type="hidden" name="token" value="%s">
<input type="hidden" name="return" value="%s">
<noscript><button type="submit">Continuar</button></noscript>
</form>
<script>document.getElementById("f").submit()</script>
</body></html>`, html.EscapeString(action), html.EscapeString(token), html.EscapeString(dest))
}
