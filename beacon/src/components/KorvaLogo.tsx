interface Props {
  size?: number
  className?: string
}

/**
 * Korva brand mark — Neural K.
 * The letter K is formed by three circuit lines connecting five node dots,
 * representing the AI memory network at the core of Korva.
 * The central hub node (slightly larger) is the Vault connection point.
 */
export function KorvaLogo({ size = 24, className }: Props) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-label="Korva"
    >
      {/* Rounded container */}
      <rect
        x="0.75" y="0.75" width="22.5" height="22.5" rx="6.5"
        fill="#f0883e18" stroke="#f0883e38" strokeWidth="1"
      />

      {/* K — vertical bar */}
      <line
        x1="7.5" y1="5.5" x2="7.5" y2="18.5"
        stroke="#f0883e" strokeWidth="1.8" strokeLinecap="round"
      />
      {/* K — upper arm */}
      <line
        x1="7.5" y1="12" x2="16" y2="5.5"
        stroke="#f0883e" strokeWidth="1.8" strokeLinecap="round"
      />
      {/* K — lower arm */}
      <line
        x1="7.5" y1="12" x2="16" y2="18.5"
        stroke="#f0883e" strokeWidth="1.8" strokeLinecap="round"
      />

      {/* Node: top of bar */}
      <circle cx="7.5" cy="5.5" r="1.5" fill="#f0883e" />
      {/* Node: bottom of bar */}
      <circle cx="7.5" cy="18.5" r="1.5" fill="#f0883e" />
      {/* Node: central hub — larger, this is the Vault core */}
      <circle cx="7.5" cy="12" r="2.1" fill="#f0883e" />
      {/* Node: upper-right tip */}
      <circle cx="16" cy="5.5" r="1.5" fill="#f0883e" />
      {/* Node: lower-right tip */}
      <circle cx="16" cy="18.5" r="1.5" fill="#f0883e" />
    </svg>
  )
}
