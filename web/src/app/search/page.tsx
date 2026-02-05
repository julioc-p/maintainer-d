"use client";

import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import AppShell from "@/components/AppShell";
import styles from "./page.module.css";

type SearchProject = {
  id: number;
  name: string;
  githubOrg?: string;
  onboardingIssue?: string | null;
  legacyMaintainerRef?: string;
  dotProjectYamlRef?: string;
};

type SearchMaintainer = {
  id: number;
  name: string;
  github: string;
  email?: string;
  company?: string;
};

type SearchCompany = {
  id: number;
  name: string;
};

type SearchResponse = {
  query: string;
  projects: SearchProject[];
  maintainers: SearchMaintainer[];
  companies: SearchCompany[];
};

export default function GlobalSearchPage() {
  const searchParams = useSearchParams();
  const query = (searchParams.get("query") || "").trim();
  const [results, setResults] = useState<SearchResponse | null>(null);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);

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

  useEffect(() => {
    let alive = true;
    if (!query) {
      setResults(null);
      setStatus("idle");
      setError(null);
      return;
    }
    const load = async () => {
      setStatus("loading");
      setError(null);
      try {
        const response = await fetch(
          `${apiBaseUrl}/search?query=${encodeURIComponent(query)}`,
          { credentials: "include" }
        );
        if (!response.ok) {
          if (response.status === 401 || response.status === 403) {
            setError("You do not have access to global search.");
            setStatus("ready");
            return;
          }
          const text = await response.text();
          throw new Error(text || `unexpected status ${response.status}`);
        }
        const data = (await response.json()) as SearchResponse;
        if (alive) {
          setResults(data);
        }
      } catch (err) {
        if (alive) {
          setError((err as Error).message || "Unable to load search results.");
        }
      } finally {
        if (alive) {
          setStatus("ready");
        }
      }
    };
    void load();
    return () => {
      alive = false;
    };
  }, [apiBaseUrl, query]);

  const hasQuery = query.length > 0;
  const hasResults =
    results &&
    (results.projects.length > 0 ||
      results.maintainers.length > 0 ||
      results.companies.length > 0);

  return (
    <AppShell>
      <main className={styles.main}>
        <header className={styles.header}>
          <h1>Global Search</h1>
          <p>Search projects, maintainers, and companies.</p>
          {hasQuery ? <div className={styles.query}>Query: “{query}”</div> : null}
        </header>

        {!hasQuery ? (
          <div className={styles.empty}>Enter a search term in the top bar.</div>
        ) : status === "loading" ? (
          <div className={styles.empty}>Searching…</div>
        ) : error ? (
          <div className={styles.error}>{error}</div>
        ) : hasResults ? (
          <div className={styles.results}>
            {results?.projects.length ? (
              <section className={styles.section}>
                <h2>Projects</h2>
                <ul>
                  {results.projects.map((project) => (
                    <li key={project.id}>
                      <Link href={`/projects/${project.id}`}>{project.name}</Link>
                      {project.githubOrg ? (
                        <span className={styles.meta}> · {project.githubOrg}</span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

            {results?.maintainers.length ? (
              <section className={styles.section}>
                <h2>Maintainers</h2>
                <ul>
                  {results.maintainers.map((maintainer) => (
                    <li key={maintainer.id}>
                      <Link href={`/maintainers/${maintainer.id}`}>{maintainer.name}</Link>
                      {maintainer.github ? (
                        <span className={styles.meta}> · {maintainer.github}</span>
                      ) : null}
                      {maintainer.email ? (
                        <span className={styles.meta}> · {maintainer.email}</span>
                      ) : null}
                      {maintainer.company ? (
                        <span className={styles.meta}> · {maintainer.company}</span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

            {results?.companies.length ? (
              <section className={styles.section}>
                <h2>Companies</h2>
                <ul>
                  {results.companies.map((company) => (
                    <li key={company.id}>
                      <Link href={`/companies/${company.id}`}>{company.name}</Link>
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

          </div>
        ) : (
          <div className={styles.empty}>No results found.</div>
        )}
      </main>
    </AppShell>
  );
}
