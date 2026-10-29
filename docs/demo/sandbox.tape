# Gravação do GIF do README (#20). Gere com: vhs docs/demo/sandbox.tape
# Precisa do lastro no PATH (go build -o lastro ./cmd/lastro) e do jq.

Output docs/demo/sandbox.gif
Set FontSize 16
Set Width 1200
Set Height 720
Set TypingSpeed 40ms

Hide
Type "rm -f /tmp/lastro-demo.* && clear" Enter
Show

Type "LASTRO_DB=/tmp/lastro-demo.db lastro sandbox 2>/dev/null | tee /tmp/lastro-demo.log &" Sleep 300ms Enter
Sleep 4s

Hide
Type "export LASTRO_TOKEN=$(grep -o 'lst_[^ ]*' /tmp/lastro-demo.log | head -1) && clear" Enter
Show

Type `TXID=$(curl -s -X POST localhost:8080/cob -H "Authorization: Bearer $LASTRO_TOKEN" -H "Idempotency-Key: pedido-$RANDOM" -H 'Content-Type: application/json' -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"sandbox@lastro.dev"}' | jq -r .txid)` Sleep 300ms Enter
Sleep 1s
Type `curl -s localhost:8080/cob/$TXID -H "Authorization: Bearer $LASTRO_TOKEN" | jq .status` Sleep 300ms Enter
Sleep 5s
Type `curl -s localhost:8080/cob/$TXID -H "Authorization: Bearer $LASTRO_TOKEN" | jq .status` Sleep 300ms Enter
Sleep 3s

Hide
Type "kill %1" Enter
