import { useId } from 'react'

interface Props {
  size?: number
  className?: string
}

/**
 * Korva brand mark — Vault K.
 * A bold geometric monogram of the letter K rendered in the signature
 * volt-green → cyan gradient, seated in a rounded "core" container.
 * The coral orbit dot at the upper tip is the brand accent.
 */
export function KorvaLogo({ size = 24, className }: Props) {
  const id = useId()
  const grad = `korva-grad-${id}`
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      role="img"
      className={className}
      aria-label="Korva"
    >
      <title>Korva</title>
      <defs>
        <linearGradient id={grad} x1="4" y1="4" x2="20" y2="20" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#00F5A0" />
          <stop offset="1" stopColor="#22D3EE" />
        </linearGradient>
      </defs>

      {/* Rounded core container */}
      <rect x="0.5" y="0.5" width="23" height="23" rx="6" fill="#05080F" />
      <rect
        x="0.5" y="0.5" width="23" height="23" rx="6"
        fill="none" stroke="#00F5A0" strokeOpacity="0.18"
      />

      {/* K monogram — bold rounded strokes */}
      <g stroke={`url(#${grad})`} strokeWidth="2.6" strokeLinecap="round">
        <line x1="7" y1="5" x2="7" y2="19" />
        <line x1="7" y1="12" x2="16.5" y2="5" />
        <line x1="7" y1="12" x2="16.5" y2="19" />
      </g>

      {/* Coral orbit accent at the upper tip */}
      <circle cx="16.5" cy="5" r="1.7" fill="#FF6B35" />
    </svg>
  )
}
