"use client";

import { useEffect, useMemo, useState } from "react";
import { FiltersSection } from "clo-ui/components/FiltersSection";
import { Pagination } from "clo-ui/components/Pagination";
import { SortOptions } from "clo-ui/components/SortOptions";
import AppShell from "@/components/AppShell";
import ProjectCard from "@/components/ProjectCard";
import { usePathname } from "next/navigation";
import styles from "./page.module.css";

type Project = {
  id: number;
  name: string;
  maturity: string;
  maintainers: { id: number; name: string; github: string }[];
};

export default function Home() {
  const [query, setQuery] = useState("");
  const [projects, setProjects] = useState<Project[]>([]);
  const [totalProjects, setTotalProjects] = useState(0);
  const [projectsStatus, setProjectsStatus] = useState<
    "idle" | "loading" | "ready"
  >("idle");
  const [projectsError, setProjectsError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [sortBy, setSortBy] = useState("name");
  const [sortDirection, setSortDirection] = useState("asc");
  const [activeMaturity, setActiveMaturity] = useState<string[]>([]);
  const pathname = usePathname();

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
    if (typeof window === "undefined") {
      return;
    }
    const syncQuery = () => {
      const params = new URLSearchParams(window.location.search);
      const nextQuery = params.get("query") || "";
      setQuery((current) => (current === nextQuery ? current : nextQuery));
    };
    syncQuery();
    window.addEventListener("popstate", syncQuery);
    window.addEventListener("md-search", syncQuery);
    return () => {
      window.removeEventListener("popstate", syncQuery);
      window.removeEventListener("md-search", syncQuery);
    };
  }, [pathname]);

  useEffect(() => {
    setPage(1);
  }, [query, activeMaturity]);

  useEffect(() => {
    let alive = true;
    const loadProjects = async () => {
      setProjectsStatus("loading");
      setProjectsError(null);
      try {
        const params = new URLSearchParams();
        if (query.trim()) {
          params.set("query", query.trim());
        }
        params.set("limit", String(limit));
        params.set("offset", String((page - 1) * limit));
        params.set("sort", sortBy);
        params.set("direction", sortDirection);
        if (activeMaturity.length > 0) {
          params.set("maturity", activeMaturity.join(","));
        }
        const response = await fetch(
          `${apiBaseUrl}/projects?${params.toString()}`,
          { credentials: "include" }
        );
        if (!response.ok) {
          if (response.status === 401) {
            if (alive) {
              setProjects([]);
              setTotalProjects(0);
            }
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as {
          total: number;
          projects: Project[];
        };
        if (alive) {
          setProjects(data.projects);
          setTotalProjects(data.total);
        }
      } catch {
        if (alive) {
          setProjectsError("Unable to load projects");
        }
      } finally {
        if (alive) {
          setProjectsStatus("ready");
        }
      }
    };
    void loadProjects();
    return () => {
      alive = false;
    };
  }, [
    activeMaturity,
    apiBaseUrl,
    limit,
    page,
    query,
    sortBy,
    sortDirection,
  ]);

  const filtersSection = useMemo(
    () => ({
      title: "Maturity level",
      options: [
        { name: "Sandbox", value: "Sandbox" },
        { name: "Incubating", value: "Incubating" },
        { name: "Graduated", value: "Graduated" },
        { name: "Archived", value: "Archived" },
      ],
    }),
    []
  );

  const offset = (page - 1) * limit;
  const rangeStart = totalProjects === 0 ? 0 : offset + 1;
  const rangeEnd = Math.min(offset + limit, totalProjects);

  return (
    <AppShell>
      <main className={styles.main}>
        {projectsStatus === "loading" && (
          <div className={styles.statusBanner}>Loading projects…</div>
        )}
        {projectsError && <div className={styles.statusBanner}>{projectsError}</div>}

        <section className={styles.controlsRow}>
          <div className={styles.resultsCount}>
            {rangeStart}-{rangeEnd} of {totalProjects} projects
          </div>
          <div className={styles.controls}>
            <SortOptions
              width={220}
              options={[
                { label: "Name A-Z", by: "name", direction: "asc" },
                { label: "Name Z-A", by: "name", direction: "desc" },
              ]}
              by={sortBy}
              direction={sortDirection}
              onSortChange={(value) => {
                const [by, direction] = value.split("_");
                setSortBy(by || "name");
                setSortDirection(direction || "asc");
                setPage(1);
              }}
            />
            <div className={styles.showSelect}>
              <label htmlFor="showLimit">Show:</label>
              <select
                id="showLimit"
                value={limit}
                onChange={(event) => {
                  setLimit(Number(event.target.value));
                  setPage(1);
                }}
              >
                {[10, 20, 30].map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </section>

        <section className={styles.layout}>
          <aside className={styles.sidebar}>
            <div className={styles.filtersHeader}>Filters</div>
            <FiltersSection
              section={filtersSection}
              visibleTitle
              device="desktop"
              activeFilters={activeMaturity}
              onChange={(name, value, checked) => {
                const selection = value || name;
                setPage(1);
                setActiveMaturity((prev) => {
                  if (checked) {
                    return [...prev, selection];
                  }
                  return prev.filter((item) => item !== selection);
                });
              }}
            />
          </aside>

          <div className={styles.content}>
            <div className={styles.cardsGrid}>
              {projects.map((project) => (
                <ProjectCard
                  key={project.id}
                  id={project.id}
                  name={project.name}
                  maturity={project.maturity}
                  maintainers={project.maintainers}
                />
              ))}
            </div>
            <div className={styles.pagination}>
              <Pagination
                limit={limit}
                total={totalProjects}
                offset={offset}
                active={page}
                onChange={(nextPage) => setPage(nextPage)}
              />
            </div>
          </div>
        </section>
      </main>

    </AppShell>
  );
}
