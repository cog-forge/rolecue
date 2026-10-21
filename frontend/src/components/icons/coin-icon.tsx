// Gold stacked-coin treatment based on the supplied visual reference.
export function CoinIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      aria-hidden="true"
      fill="none"
    >
      <ellipse
        cx="15.5"
        cy="16"
        rx="6"
        ry="6.5"
        fill="#D78A18"
        stroke="#FFE39A"
        strokeWidth="1.2"
        transform="rotate(24 15.5 16)"
      />
      <ellipse
        cx="10"
        cy="10"
        rx="7"
        ry="8"
        fill="#F5B938"
        stroke="#FFE39A"
        strokeWidth="1.4"
        transform="rotate(24 10 10)"
      />
      <ellipse
        cx="10"
        cy="10"
        rx="4.8"
        ry="5.8"
        stroke="#C58319"
        strokeWidth="1"
        transform="rotate(24 10 10)"
      />
      <path d="m11.5 5-4 5h3l-2 5 5-6h-3l1-4Z" fill="#FFF2BA" />
      <path
        d="m17 13-2 4h2l-1 3"
        stroke="#FFE39A"
        strokeWidth="1.2"
        strokeLinecap="round"
      />
    </svg>
  );
}
