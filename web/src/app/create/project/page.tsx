"use client";

import { useMemo } from "react";
import { useRouter } from "next/navigation";
import AppShell from "@/components/AppShell";
import ProjectCreateForm from "@/components/ProjectCreateForm";
import styles from "./page.module.css";

export default function CreateProjectPage() {
  const router = useRouter();
  const bffBaseUrl = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_BFF_BASE_URL || "/api";
    return raw.replace(/\/+$/, "");
  }, []);
  const apiBaseUrl = useMemo(() => {
    if (bffBaseUrl === "") {
      return "/api";
    }
    if (bffBaseUrl.endsWith("/api")) {
      return bffBaseUrl;
    }
    return `${bffBaseUrl}/api`;
  }, [bffBaseUrl]);

  return (
    <AppShell>
      <main className={styles.main}>
        <header className={styles.header}>
          <div>
            <h1>Create New Project</h1>
            <p>
              Use the onboarding issue to seed project metadata, then add the project
              files.
            </p>
          </div>
        </header>
        <ProjectCreateForm
          apiBaseUrl={apiBaseUrl}
          onCancel={() => router.push("/")}
          onCreated={(projectId) => router.push(`/projects/${projectId}`)}
        />
      </main>
    </AppShell>
  );
}
