import { useTranslations } from "next-intl";

// The navigation config keeps the stable English names; the catalog supplies what people read.
const groupKeys = {
  Practice: "practice",
  Career: "career",
  Account: "account",
  Recruitment: "recruitment",
  Governance: "governance",
  Operations: "operations",
  Finance: "finance",
} as const;

const itemKeys = {
  Dashboard: "dashboard",
  "Role Profiles": "roleProfiles",
  "Interview History": "interviewHistory",
  "Personal Avatar Studio": "avatarStudio",
  "Job Board": "jobBoard",
  "My Applications": "myApplications",
  Transactions: "transactions",
  "My Job Postings": "myJobPostings",
  "Received Applications": "receivedApplications",
  Accounts: "accounts",
  "Job Posting Moderation": "jobPostingModeration",
  "Interview Sessions": "interviewSessions",
  "Access Audit": "accessAudit",
  "Voice Profiles": "voiceProfiles",
  "Interview Guidelines": "interviewGuidelines",
  "Payment Transactions": "paymentTransactions",
  "Revenue Reports": "revenueReports",
} as const;

function isGroup(label: string): label is keyof typeof groupKeys {
  return Object.hasOwn(groupKeys, label);
}
function isItem(label: string): label is keyof typeof itemKeys {
  return Object.hasOwn(itemKeys, label);
}

export function useNavigationLabels() {
  const t = useTranslations("common.shell.nav");
  return {
    group: (label: string) =>
      isGroup(label) ? t(`groups.${groupKeys[label]}`) : label,
    item: (label: string) =>
      isItem(label) ? t(`items.${itemKeys[label]}`) : label,
  };
}

export type ShellRole = "candidate" | "recruiter" | "admin";

// "Administrator" is the English name of the admin role.
export function useRoleLabel() {
  const t = useTranslations("common.shell.roles");
  return (role: ShellRole) => t(role);
}
