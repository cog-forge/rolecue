"use client";

import { JDEntry } from "@/features/job-description";
import { candidateJDWorkflow } from "../api/workflow";

export function RoleProfileEntry() {
  return <JDEntry workflow={candidateJDWorkflow} />;
}
