// One place for how the Applications list and the Application detail print dates.
const submittedFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});
// The deadline is a moment, so it is shown in the viewer's own time zone, named to avoid misreading.
const deadlineFormat = new Intl.DateTimeFormat("en", {
  year: "numeric",
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  timeZoneName: "short",
});

export function formatSubmitted(iso: string) {
  return submittedFormat.format(new Date(iso));
}

export function formatDeadline(iso: string) {
  return deadlineFormat.format(new Date(iso));
}
