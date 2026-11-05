// The only host a PayOS checkout lives on. Deployments that use another PayOS checkout host list it in
// NEXT_PUBLIC_PAYOS_CHECKOUT_HOSTS (comma separated hostnames); nothing else is ever navigated to.
const defaultCheckoutHosts = ["pay.payos.vn"];

export function trustedCheckoutHosts(
  configured: string | undefined = process.env.NEXT_PUBLIC_PAYOS_CHECKOUT_HOSTS,
) {
  const hosts = (configured ?? "")
    .split(",")
    .map((host) => host.trim().toLowerCase())
    .filter((host) => /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$/.test(host));
  return hosts.length > 0 ? hosts : defaultCheckoutHosts;
}

// An absolute https URL on a trusted checkout host, with no credentials and the default port.
export function isTrustedCheckoutUrl(
  value: string,
  hosts: readonly string[] = trustedCheckoutHosts(),
) {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return false;
  }
  return (
    url.protocol === "https:" &&
    !url.username &&
    !url.password &&
    !url.port &&
    hosts.includes(url.hostname.toLowerCase())
  );
}

// Navigates to the PayOS checkout, and only to a trusted one. Returns false when it refuses.
export function redirectToCheckout(value: string) {
  if (!isTrustedCheckoutUrl(value)) return false;
  window.location.href = value;
  return true;
}
