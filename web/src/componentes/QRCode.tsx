import { useEffect, useState } from "react";
import { toString } from "qrcode";
import { CopyField } from "./CopyField";

/** BR Code renderizado e o "Pix Copia e Cola" logo abaixo. */
export function QRCode({ brcode }: { brcode: string }) {
  const [svg, setSvg] = useState("");
  useEffect(() => {
    toString(brcode, { type: "svg", margin: 1, errorCorrectionLevel: "M" }).then(setSvg, () => setSvg(""));
  }, [brcode]);
  return (
    <figure className="qrcode" style={{ margin: 0, display: "grid", gridTemplateColumns: "minmax(0, 1fr)", gap: "var(--space-3)", maxWidth: 260, minWidth: 0 }}>
      <div role="img" aria-label="QR Code do Pix" style={{ background: "#ffffff", padding: "var(--space-2)", borderRadius: "var(--radius-md)", border: "1px solid var(--border)" }} dangerouslySetInnerHTML={{ __html: svg }} />
      <figcaption>
        <div className="muted" style={{ fontSize: "var(--text-12)" }}>Pix Copia e Cola</div>
        <CopyField valor={brcode} rotulo="Pix Copia e Cola" />
      </figcaption>
    </figure>
  );
}
