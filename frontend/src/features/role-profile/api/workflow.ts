import { routes } from "@/config/routes";
import type { JDWorkflow } from "@/features/job-description";
import { roleProfileKeys } from "./keys";
import {
  createRoleProfile,
  getRoleProfile,
  analyzeRoleProfile,
  createPDFUpload,
  completePDFUpload,
} from "./requests";

export const candidateJDWorkflow: JDWorkflow = {
  audience: "candidate",
  api: {
    create: createRoleProfile,
    get: getRoleProfile,
    analyze: analyzeRoleProfile,
    createUpload: createPDFUpload,
    completeUpload: completePDFUpload,
  },
  keys: roleProfileKeys,
  links: { list: routes.roleProfiles.root, detail: routes.roleProfiles.detail },
};
