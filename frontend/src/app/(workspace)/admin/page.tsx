import { redirect } from "next/navigation";
import { routes } from "@/config/routes";
export default function AdminPage() {
  redirect(routes.admin.dashboard);
}
