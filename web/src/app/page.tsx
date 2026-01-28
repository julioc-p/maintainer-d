"use client";

import AppShell from "@/components/AppShell";
import ProjectsList from "@/components/ProjectsList";
import styles from "./page.module.css";

export default function Home() {
  return (
    <AppShell>
      <main className={styles.main}>
        <ProjectsList />
      </main>

    </AppShell>
  );
}
