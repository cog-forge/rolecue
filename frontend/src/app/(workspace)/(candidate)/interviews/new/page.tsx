import { redirect } from "next/navigation";
import { routes } from "@/config/routes";
export default function NewInterviewPage() {
  redirect(routes.interviews.new.jobDescription);
}
