import type { Metadata } from "next";
import { AuthShell } from "@/components/login/AuthShell";
import { ForgotForm } from "./ForgotForm";

export const metadata: Metadata = {
  title: "Forgot password · Mindfries",
  description: "Get a link to choose a new password.",
};

export default function ForgotPasswordPage() {
  return (
    <AuthShell>
      <ForgotForm />
    </AuthShell>
  );
}
