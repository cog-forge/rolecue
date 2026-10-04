import Image from "next/image";
import * as React from "react";

export interface LoginViewProps {
  children?: React.ReactNode;
}

export function LoginView({ children }: LoginViewProps) {
  return (
    <main className="min-h-screen bg-[#fafafa] text-[#262626] antialiased">
      <div className="flex min-h-screen">
        {/* Left column: Authentication surface */}
        <div className="flex min-h-screen w-full flex-col lg:w-1/2">
          <div className="mx-auto flex min-h-screen w-full max-w-140 flex-col justify-center px-6 py-10 sm:px-12 lg:px-16">
            <section
              aria-label="Account access"
              className="relative mx-auto w-full max-w-100 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-500"
            >
              {children}
            </section>
          </div>
        </div>

        <aside
          aria-hidden="true"
          className="relative hidden min-h-screen lg:block lg:w-1/2 p-3"
        >
          <div className="relative h-full w-full overflow-hidden rounded-3xl border border-black/5 shadow-[0_4px_32px_rgba(0,0,0,0.04)] bg-[#f0f0f0]">
            <Image
              src="/images/login-art.webp"
              alt=""
              fill
              loading="eager"
              sizes="50vw"
              className="object-cover transition-transform duration-700 hover:scale-[1.01]"
            />
          </div>
        </aside>
      </div>
    </main>
  );
}
