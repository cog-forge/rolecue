import { Eye } from "lucide-react";

/** Keep the eye mounted so repeated toggles smoothly reverse the same transition. */
export function PasswordVisibilityIcon({ visible }: { visible: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={`relative inline-flex size-4 shrink-0 transition-transform duration-320 ease-in-out motion-reduce:transform-none motion-reduce:transition-none ${visible ? "scale-95" : "scale-100"}`}
    >
      <Eye
        className={`size-4 [&_circle]:transition-opacity [&_circle]:duration-320 [&_circle]:ease-in-out motion-reduce:[&_circle]:transition-none ${visible ? "[&_circle]:opacity-50" : "[&_circle]:opacity-100"}`}
      />
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        className="absolute inset-0 size-4"
      >
        <path
          d="M3 3 21 21"
          pathLength="1"
          strokeDasharray="1"
          strokeDashoffset={visible ? 0 : 1}
          className="transition-[stroke-dashoffset,opacity] duration-320 ease-in-out motion-reduce:transition-none"
          opacity={visible ? 1 : 0}
        />
      </svg>
    </span>
  );
}
