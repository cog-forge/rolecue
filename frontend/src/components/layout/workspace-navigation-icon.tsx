"use client";

import { useEffect, useRef } from "react";
import { useWorkspacePreferences } from "@/providers/workspace-provider";
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
import { LayoutGridIcon } from "@/components/icons/animated/layout-grid";
import { FileTextIcon } from "@/components/icons/animated/file-text";
import { HistoryIcon } from "@/components/icons/animated/history";
import { UserIcon } from "@/components/icons/animated/user";
import { BriefcaseBusinessIcon } from "@/components/icons/animated/briefcase-business";
import { ClipboardCheckIcon } from "@/components/icons/animated/clipboard-check";
import { CreditCardIcon } from "@/components/icons/animated/credit-card";
import { UsersRoundIcon } from "@/components/icons/animated/users-round";
import { ShieldCheckIcon } from "@/components/icons/animated/shield-check";
import { MicIcon } from "@/components/icons/animated/mic";
import { SlidersHorizontalIcon } from "@/components/icons/animated/sliders-horizontal";
import { BrainIcon } from "@/components/icons/animated/brain";
import { AudioLinesIcon } from "@/components/icons/animated/audio-lines";
import { ChartLineIcon } from "@/components/icons/animated/chart-line";

import { workspaceNavigation, type NavigationGroup } from "@/config/navigation";

const animatedIcons = new Map([
  [LayoutDashboard, LayoutGridIcon],
  [FileText, FileTextIcon],
  [History, HistoryIcon],
  [UserRound, UserIcon],
  [BriefcaseBusiness, BriefcaseBusinessIcon],
  [ClipboardList, ClipboardCheckIcon],
  [CreditCard, CreditCardIcon],
  [UsersRound, UsersRoundIcon],
  [ShieldCheck, ShieldCheckIcon],
  [Mic, MicIcon],
  [SlidersHorizontal, SlidersHorizontalIcon],
  [BrainCircuit, BrainIcon],
  [ListChecks, ClipboardCheckIcon],
  [AudioLines, AudioLinesIcon],
  [ChartNoAxesCombined, ChartLineIcon],
]);

function withMotion(groups: readonly NavigationGroup[]) {
  return groups.map((group) => ({
    ...group,
    items: group.items.map((item) => ({
      ...item,
      animatedIcon: animatedIcons.get(item.icon),
    })),
  }));
}
export const workspaceNavigationWithMotion = {
  candidate: withMotion(workspaceNavigation.candidate),
  recruiter: withMotion(workspaceNavigation.recruiter),
  admin: withMotion(workspaceNavigation.admin),
};

// Drive the upstream icon from the entire navigation row, including keyboard focus.
export function WorkspaceNavigationIcon({
  icon: Icon,
  animate,
  animatedIcon: AnimatedIcon,
}: {
  icon: LucideIcon;
  animate: boolean;
  animatedIcon: typeof LayoutGridIcon | undefined;
}) {
  const { reducedMotion } = useWorkspacePreferences();
  const handle = useRef<{
    startAnimation: () => void;
    stopAnimation: () => void;
  }>(null);
  useEffect(() => {
    if (reducedMotion || !handle.current) return;
    if (animate) handle.current.startAnimation();
    else handle.current.stopAnimation();
  }, [animate, reducedMotion]);
  if (reducedMotion || !AnimatedIcon)
    return <Icon className="size-[18px]" strokeWidth={1.6} aria-hidden />;
  return (
    <AnimatedIcon
      ref={handle}
      size={18}
      aria-hidden
      className="[&_svg]:stroke-[1.6]"
    />
  );
}
