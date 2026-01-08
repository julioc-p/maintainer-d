"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import AppShell from "@/components/AppShell";
import MaintainerCard from "@/components/MaintainerCard";
import styles from "./page.module.css";

type MaintainerDetail = {
  id: number;
  name: string;
  email: string;
  github: string;
  githubEmail: string;
  status: string;
  company?: string;
  projects: string[];
};

const maintainerDataHasChanged = (
  current: MaintainerDetail | null,
  next: MaintainerDetail
): boolean => {
  if (!current) {
    return true;
  }
  if (
    current.id !== next.id ||
    current.name !== next.name ||
    current.email !== next.email ||
    current.github !== next.github ||
    current.githubEmail !== next.githubEmail ||
    current.status !== next.status ||
    current.company !== next.company
  ) {
    return true;
  }
  if (current.projects.length !== next.projects.length) {
    return true;
  }
  for (let index = 0; index < current.projects.length; index += 1) {
    if (current.projects[index] !== next.projects[index]) {
      return true;
    }
  }
  return false;
};

export default function MaintainerPage() {
  const [maintainer, setMaintainer] = useState<MaintainerDetail | null>(null);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const maintainerId = params?.id;
  const pollIntervalMs = 5000;

  const bffBaseUrl = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_BFF_BASE_URL || "/api";
    return raw.replace(/\/+$/, "");
  }, []);

  useEffect(() => {
    let alive = true;
    if (!maintainerId) {
      return () => {
        alive = false;
      };
    }

    const loadMaintainer = async () => {
      if (maintainer === null) {
        setStatus("loading");
        setError(null);
      }
      try {
        const response = await fetch(
          `${bffBaseUrl}/api/maintainers/${maintainerId}`,
          { credentials: "include" }
        );
        if (!response.ok) {
          if (response.status === 401) {
            router.push("/");
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as MaintainerDetail;
        if (alive && maintainerDataHasChanged(maintainer, data)) {
          setMaintainer(data);
        }
      } catch (err) {
        if (alive && error !== "Unable to load maintainer") {
          setError("Unable to load maintainer");
        }
      } finally {
        if (alive) {
          setStatus("ready");
        }
      }
    };

    void loadMaintainer();
    const intervalId = window.setInterval(loadMaintainer, pollIntervalMs);

    return () => {
      alive = false;
      window.clearInterval(intervalId);
    };
  }, [bffBaseUrl, error, maintainer, maintainerId, pollIntervalMs, router]);

  return (
    <AppShell>
      <div className={styles.page}>
        <div className={styles.container}>
          {status === "loading" && <div className={styles.banner}>Loading…</div>}
          {error && <div className={styles.banner}>{error}</div>}
          {maintainer && (
            <MaintainerCard
              name={maintainer.name}
              email={maintainer.email}
              github={maintainer.github}
              githubEmail={maintainer.githubEmail}
              status={maintainer.status}
              company={maintainer.company}
              projects={maintainer.projects}
            />
          )}
        </div>
      </div>
    </AppShell>
  );
}
