import {
  ChartGantt,
  Store,
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
  AudioLines,
  ChartNoAxesCombined,
  Wallet,
  ScrollText,
  SlidersHorizontal,
  ListChecks,
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
          label: "Role Profiles",
          href: routes.roleProfiles.root,
          icon: FileText,
          match: [
            routes.roleProfiles.root,
            routes.targetJds,
            "/interviews/new",
          ],
        },
        {
          label: "Learning Roadmaps",
          href: routes.roadmaps,
          icon: ChartGantt,
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
        { label: "Avatar Marketplace", href: routes.marketplace, icon: Store },
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
          label: "Transactions",
          href: routes.billing,
          icon: Wallet,
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
        {
          label: "Scoring rubrics",
          href: routes.recruiter.rubrics,
          icon: ListChecks,
        },
        {
          label: "Personal Avatar Studio",
          href: routes.avatarStudio,
          icon: UserRound,
        },
        { label: "Avatar Marketplace", href: routes.marketplace, icon: Store },
      ],
    },
    {
      label: "Account",
      items: [
        {
          label: "Transactions",
          href: routes.recruiter.billing,
          icon: Wallet,
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
      label: "Marketplace",
      items: [
        {
          label: "Marketplace Review",
          href: routes.admin.marketplace,
          icon: Store,
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
          label: "Access Audit",
          href: routes.admin.accessAudit,
          icon: ScrollText,
        },
        {
          label: "Voice Profiles",
          href: routes.admin.voices,
          icon: AudioLines,
        },
        {
          label: "Interview Guidelines",
          href: routes.admin.interviewGuidelines,
          icon: SlidersHorizontal,
        },
        {
          label: "Scoring rubrics",
          href: routes.admin.rubrics,
          icon: ListChecks,
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
  // A roadmap detail lives under its profile URL, but belongs to the roadmap library.
  if (
    role === "candidate" &&
    /^\/role-profiles\/[^/]+\/roadmap\/?$/.test(pathname)
  )
    return workspaceNavigation.candidate
      .flatMap((group) => group.items)
      .find((item) => item.href === routes.roadmaps);
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
    routes.roleProfiles.root,
    routes.roadmaps,
    routes.avatarStudio,
    routes.marketplace,
    routes.jobs,
    routes.applications,
    routes.history,
    routes.billing,
    routes.profile,
    routes.settings,
  ].some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

export const interviewSteps = [
  { label: "Role Profile", href: routes.interviews.new.jobDescription },
  { label: "Requirements", href: routes.interviews.new.skills },
  { label: "Setup", href: routes.interviews.new.setup },
  { label: "Interviewer", href: routes.interviews.new.interviewer },
  { label: "Preflight", href: routes.interviews.new.preflight },
];
