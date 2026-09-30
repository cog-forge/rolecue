import Image from "next/image";

export interface RoleCueMarkProps {
  className?: string;
  alt?: string;
  priority?: boolean;
}

/** Canonical standalone RoleCue mark for shared product surfaces. */
export function RoleCueMark({
  alt = "",
  className,
  priority = false,
}: RoleCueMarkProps) {
  return (
    <Image
      alt={alt}
      className={className}
      height={200}
      priority={priority}
      src="/rolecue-cue.png"
      width={200}
    />
  );
}
