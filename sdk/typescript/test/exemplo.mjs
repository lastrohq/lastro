// O exemplo do README: cria uma cobrança no sandbox e espera ela ser paga.
import { Lastro } from "@lastrohq/sdk";

const lastro = new Lastro({ url: process.env.LASTRO_URL, token: process.env.LASTRO_TOKEN });
const cob = await lastro.cob.criar({
  calendario: { expiracao: 3600 },
  valor: { original: "49.90" },
  chave: "sandbox@lastro.dev",
});
console.log(cob.txid, cob.status, cob.pixCopiaECola);
