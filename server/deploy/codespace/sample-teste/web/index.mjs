const who = process.env.TESTE_WHO || "XCODESPACES";
const marker = process.env.TESTE_MARK || "(sem TESTE_MARK — grave um ENV plaintext em xgit.corp/teste/settings)";

console.log(`node ok — olá, ${who}`);
console.log(`TESTE_MARK=${marker}`);
if (process.env.XCS_LLM_KEY) {
  console.error("XCS_LLM_KEY não deveria existir no container");
  process.exit(1);
}
