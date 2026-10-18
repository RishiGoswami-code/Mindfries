/**
 * The scattered curved shapes from the reference, redrawn in the
 * dashboard's own family — violet, lavender, the stat cards' pink and mint —
 * rather than the reference's mixed rainbow: a login page is a brand moment,
 * and five unrelated hues would read as a different product's screenshot
 * rather than Mindfries'. Purely decorative: aria-hidden and click-through,
 * positioned absolutely around a relatively-positioned container the page
 * supplies.
 */
export function LoginBlobs() {
  const blobs = [
    { top: "8%", left: "22%", size: 46, tint: "#F6C9E6", rotate: -20 },
    { top: "16%", right: "12%", size: 40, tint: "#5B3BE0", rotate: 140 },
    { top: "44%", left: "10%", size: 34, tint: "#B9A7F6", rotate: 60 },
    { bottom: "16%", left: "14%", size: 40, tint: "#DCD3FB", rotate: -30 },
    { bottom: "10%", right: "14%", size: 44, tint: "#A8E6D3", rotate: 100 },
  ] as const;

  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden">
      {blobs.map((b, i) => (
        <span
          key={i}
          className="absolute opacity-70"
          style={{
            top: "top" in b ? b.top : undefined,
            bottom: "bottom" in b ? b.bottom : undefined,
            left: "left" in b ? b.left : undefined,
            right: "right" in b ? b.right : undefined,
            width: b.size,
            height: b.size,
            backgroundColor: b.tint,
            borderRadius: "100% 100% 100% 0%",
            transform: `rotate(${b.rotate}deg)`,
          }}
        />
      ))}
    </div>
  );
}
