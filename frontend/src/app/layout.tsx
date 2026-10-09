import type { Metadata } from "next";
import localFont from "next/font/local";
import "./globals.css";
import { site } from "@/config/site";
import { AppProviders } from "@/providers/app-providers";

const albertSans = localFont({
  src: [
    {
      path: "../assets/fonts/albert-sans/albert-sans-variable.woff2",
      weight: "100 900",
      style: "normal",
    },
    {
      path: "../assets/fonts/albert-sans/albert-sans-italic-variable.woff2",
      weight: "100 900",
      style: "italic",
    },
  ],
  display: "swap",
  variable: "--font-albert-sans",
});

export const metadata: Metadata = {
  title: {
    default: site.name,
    template: `%s | ${site.name}`,
  },
  description: site.description,
  icons: {
    icon: [
      {
        url: "/rolecue-cue.png",
        type: "image/png",
        sizes: "200x200",
      },
    ],
  },
};
export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `var t="system";try{var p=JSON.parse(localStorage.getItem("rolecue.workspace-preferences")||"null");t=p?.state?.theme??p?.theme??t;if(t!=="light"&&t!=="dark"&&t!=="system")t="system"}catch{}var w=/^\\/(admin|recruiter|dashboard|interviews|reports|target-jds|avatar-studio|jobs|applications|history|billing|profile|settings)(\\/|$)/.test(location.pathname);document.documentElement.classList.toggle("dark",w&&(t==="dark"||(t==="system"&&matchMedia("(prefers-color-scheme: dark)").matches)))`,
          }}
        />
      </head>
      <body
        className={`${albertSans.variable} min-h-screen bg-background font-sans text-foreground antialiased`}
      >
        <AppProviders>{children}</AppProviders>
      </body>
    </html>
  );
}
