import {
  LayoutDashboard,
  FileText,
  History,
  UserRound,
  BriefcaseBusiness,
  ClipboardList,
  CreditCard,
  UsersRound,
  ShieldCheck,
  Mic,
  SlidersHorizontal,
  BrainCircuit,
  ListChecks,
  AudioLines,
  ChartNoAxesCombined,
  type LucideIcon,
} from "lucide-react";
import type { ProductRole } from "./permissions";
import { routes } from "./routes";
export const publicNavigation = [
  { label: "Home", href: routes.home },
  { label: "Pricing", href: routes.pricing },
  { label: "Log in", href: routes.login },
];
export interface NavigationItem {
  label: string;
  href: string;
  icon: LucideIcon;
  match?: readonly string[];
}
export interface NavigationGroup {
  label: string;
  items: readonly NavigationItem[];
}
export const workspaceNavigation: Record<
  ProductRole,
  readonly NavigationGroup[]
> = {
  candidate: [
    {
      label: "Practice",
      items: [
        { label: "Dashboard", href: routes.dashboard, icon: LayoutDashboard },
        {
          label: "Target JDs",
          href: routes.targetJds,
          icon: FileText,
          match: [routes.targetJds, "/interviews/new"],
        },
        {
          label: "Interview History",
          href: routes.history,
          icon: History,
          match: [routes.history, "/reports"],
        },
        {
          label: "Personal Avatar Studio",
          href: routes.avatarStudio,
          icon: UserRound,
        },
      ],
    },
    {
      label: "Career",
      items: [
        { label: "Job Board", href: routes.jobs, icon: BriefcaseBusiness },
        {
          label: "My Applications",
          href: routes.applications,
          icon: ClipboardList,
        },
      ],
    },
    {
      label: "Account",
      items: [
        {
          label: "Membership & Billing",
          href: routes.billing,
          icon: CreditCard,
        },
      ],
    },
  ],
  recruiter: [
    {
      label: "Recruitment",
      items: [
        {
          label: "Dashboard",
          href: routes.recruiter.dashboard,
          icon: LayoutDashboard,
        },
        {
          label: "My Job Postings",
          href: routes.recruiter.jobPostings,
          icon: BriefcaseBusiness,
        },
        {
          label: "Received Applications",
          href: routes.recruiter.applications,
          icon: ClipboardList,
        },
      ],
    },
  ],
  admin: [
    {
      label: "Governance",
      items: [
        {
          label: "Dashboard",
          href: routes.admin.dashboard,
          icon: LayoutDashboard,
        },
        { label: "Accounts", href: routes.admin.users, icon: UsersRound },
        {
          label: "Job Posting Moderation",
          href: routes.admin.jobPostings,
          icon: ShieldCheck,
        },
      ],
    },
    {
      label: "Operations",
      items: [
        {
          label: "Interview Sessions",
          href: routes.admin.interviews,
          icon: Mic,
        },
        {
          label: "Interview Configuration",
          href: routes.admin.settings,
          icon: SlidersHorizontal,
        },
        {
          label: "AI Behaviour",
          href: routes.admin.aiBehaviour,
          icon: BrainCircuit,
        },
        {
          label: "Evaluation Criteria",
          href: routes.admin.evaluationCriteria,
          icon: ListChecks,
        },
        {
          label: "Voice Profiles",
          href: routes.admin.voices,
          icon: AudioLines,
        },
      ],
    },
    {
      label: "Finance",
      items: [
        {
          label: "Payment Transactions",
          href: routes.admin.billing,
          icon: CreditCard,
        },
        {
          label: "Revenue Reports",
          href: routes.admin.revenue,
          icon: ChartNoAxesCombined,
        },
      ],
    },
  ],
};
export const roleLabels: Record<ProductRole, string> = {
  candidate: "Candidate",
  recruiter: "Recruiter",
  admin: "Administrator",
};
export function workspaceHome(role: ProductRole) {
  return role === "admin"
    ? routes.admin.dashboard
    : role === "recruiter"
      ? routes.recruiter.dashboard
      : routes.dashboard;
}
export function getActiveNavigation(role: ProductRole, url: string) {
  const pathname = url.split(/[?#]/)[0];
  return workspaceNavigation[role]
    .flatMap((group) => group.items)
    .flatMap((item) =>
      (item.match ?? [item.href]).map((prefix) => ({ item, prefix })),
    )
    .filter(
      ({ prefix }) => pathname === prefix || pathname.startsWith(`${prefix}/`),
    )
    .sort((a, b) => b.prefix.length - a.prefix.length)[0]?.item;
}
export function isWorkspacePath(pathname: string) {
  return [
    "/admin",
    "/recruiter",
    "/interviews",
    "/reports",
    routes.dashboard,
    routes.targetJds,
    routes.avatarStudio,
    routes.jobs,
    routes.applications,
    routes.history,
    routes.billing,
    routes.profile,
    routes.settings,
  ].some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

export const interviewSteps = [
  { label: "Job description", href: routes.interviews.new.jobDescription },
  { label: "Skills", href: routes.interviews.new.skills },
  { label: "Setup", href: routes.interviews.new.setup },
  { label: "Interviewer", href: routes.interviews.new.interviewer },
  { label: "Preflight", href: routes.interviews.new.preflight },
];
