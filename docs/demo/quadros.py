# Gera os quadros do GIF do README a partir de uma sessão real do sandbox
# (capturada por docs/demo/gerar-gif.sh). Uso: python3 quadros.py <pasta-de-trabalho>
import json, html, sys, os
G = sys.argv[1]
banner = open(f"{G}/run/banner.txt").read().rstrip("\n").split("\n")
criada = json.load(open(f"{G}/run/criada.json")); paga = json.load(open(f"{G}/run/paga.json"))
token = next(l.split("=",1)[1] for l in banner if "export VINTEM_TOKEN=" in l)
txid = criada["txid"]

P = '<span class="p">$</span> '
def esc(s): return html.escape(s)
quadros = []  # (linhas_html, segundos)
def tela(linhas, seg): quadros.append((list(linhas), seg))
def digitar(antes, cmd, passo=4, seg=0.045):
    for i in range(passo, len(cmd) + passo, passo):
        tela(antes + [P + esc(cmd[:i]) + '<span class="c">▋</span>'], seg)
    return antes + [P + esc(cmd)]

# Cena 1: sobe o sandbox
t = digitar([], "vintem sandbox", 2, 0.06)
tela(t + ['<span class="c">▋</span>'], 0.5)
corpo = []
for l in banner:
    e = esc(l)
    if l.startswith("🧪"): e = '<span class="t">' + e + '</span>'
    for rotulo in ("API ", "Conta ", "API key ", "Dados "):
        if l.strip().startswith(rotulo): e = e.replace(esc(rotulo), '<span class="d">' + esc(rotulo) + '</span>', 1)
    corpo.append(e)
tela(t + corpo, 2.6)

# Cena 2: cria a cobrança e espera o pagamento
t = digitar([], f"export VINTEM_TOKEN={token}", 12, 0.03)
cmd = "curl -s -X POST localhost:8080/cob -H \"Authorization: Bearer $VINTEM_TOKEN\" -H \"Idempotency-Key: pedido-42\" -d '{\"valor\":{\"original\":\"49.90\"},\"chave\":\"sandbox@vintem.dev\",\"calendario\":{\"expiracao\":3600}}' | jq '{txid, status}'"
t = digitar(t, cmd, 9, 0.03)
saida = ['{', f'  <span class="k">"txid"</span>: <span class="s">"{txid}"</span>,', '  <span class="k">"status"</span>: <span class="y">"ATIVA"</span>', '}']
tela(t + saida, 1.3)
t = t + saida + ['', '<span class="m"># o pagador do sandbox paga em 5 s…</span>']
tela(t, 1.4)
cmd2 = f"curl -s localhost:8080/cob/{txid} -H \"Authorization: Bearer $VINTEM_TOKEN\" | jq .status"
t = digitar(t, cmd2, 8, 0.03)
tela(t + ['<span class="g">"' + paga["status"] + '"</span>  <span class="m">✔ Pix pago, sem gateway</span>'], 3.0)

tpl = open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "tela.html")).read()
lista = []
for i, (linhas, seg) in enumerate(quadros):
    open(f"{G}/frames/q{i:03d}.html", "w").write(tpl.replace("{{LINHAS}}", "\n".join(linhas)))
    lista.append((f"q{i:03d}", seg))
json.dump(lista, open(f"{G}/frames/lista.json", "w"))
print(len(quadros), "quadros,", round(sum(s for _, s in lista), 1), "s")
