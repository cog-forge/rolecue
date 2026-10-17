export function isAccountLocked(
  user: { banned?: boolean | null; banExpires?: Date | null },
  now: number,
) {
  return (
    user.banned === true &&
    (!user.banExpires || user.banExpires.getTime() > now)
  );
}
