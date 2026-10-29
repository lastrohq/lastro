#!/usr/bin/env bash
# Regrava docs/demo/sandbox.gif (#20, #40) a partir de uma sessão real:
# sobe o lastro sandbox, cria uma cobrança, espera o pagamento e desenha os
# quadros com a saída de verdade. Precisa de go, curl, jq, python3, Google
# Chrome (ou Chromium) e ffmpeg. Roda da raiz do repositório.
set -euo pipefail

DEMO="$(cd "$(dirname "$0")" && pwd)"
TRAB="$(mktemp -d)"
trap 'kill "${PID:-}" 2>/dev/null || true; rm -rf "$TRAB"' EXIT
CHROME="$(command -v google-chrome || command -v chromium || command -v chromium-browser)"

mkdir -p "$TRAB/run" "$TRAB/frames"
go build -o "$TRAB/run/lastro" ./cmd/lastro
(cd "$TRAB/run" && exec env LASTRO_ADDR=127.0.0.1:8080 ./lastro sandbox > banner.txt 2>/dev/null) &
PID=$!
for _ in $(seq 50); do curl -sf localhost:8080/health >/dev/null && break; sleep 0.2; done

TOKEN="$(grep -o 'lst_[^ ]*' "$TRAB/run/banner.txt" | head -1)"
curl -s -X POST localhost:8080/cob -H "Authorization: Bearer $TOKEN" -H "Idempotency-Key: pedido-42" \
  -H 'Content-Type: application/json' \
  -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"sandbox@lastro.dev"}' > "$TRAB/run/criada.json"
sleep 6
curl -s "localhost:8080/cob/$(jq -r .txid "$TRAB/run/criada.json")" -H "Authorization: Bearer $TOKEN" > "$TRAB/run/paga.json"
jq -e '.status == "CONCLUIDA"' "$TRAB/run/paga.json" >/dev/null || { echo "a cobrança não foi paga"; exit 1; }

python3 "$DEMO/quadros.py" "$TRAB"
cd "$TRAB/frames"
for f in q*.html; do
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --no-first-run --user-data-dir="$TRAB/perfil" \
    --window-size=1036,620 --screenshot="${f%.html}.png" "file://$TRAB/frames/$f" >/dev/null 2>&1
done
python3 - <<'PY'
import json
l = json.load(open("lista.json"))
with open("concat.txt", "w") as o:
    for n, s in l:
        o.write(f"file '{n}.png'\nduration {s}\n")
    o.write(f"file '{l[-1][0]}.png'\n")
PY
ffmpeg -loglevel error -y -f concat -safe 0 -i concat.txt \
  -vf "fps=20,scale=900:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=64:stats_mode=diff[p];[b][p]paletteuse=dither=none" \
  -loop 0 "$DEMO/sandbox.gif"
echo "docs/demo/sandbox.gif regravado"
