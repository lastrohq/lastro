// O exemplo do README: cria uma cobrança no sandbox e espera ela ser paga.
import { Vintem } from "@vintemhq/sdk";

const vintem = new Vintem({ url: process.env.VINTEM_URL, token: process.env.VINTEM_TOKEN });
const cob = await vintem.cob.criar({
  calendario: { expiracao: 3600 },
  valor: { original: "49.90" },
  chave: "sandbox@vintem.dev",
});
console.log(cob.txid, cob.status, cob.pixCopiaECola);
