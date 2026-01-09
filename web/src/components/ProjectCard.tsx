"use client";

import { Card } from "clo-ui/components/Card";
import styles from "./ProjectCard.module.css";

type Maintainer = {
  id: number;
  name: string;
  github: string;
};

type ProjectCardProps = {
  id: number;
  name: string;
  maturity: string;
  maintainers: Maintainer[];
};

export default function ProjectCard({
  id,
  name,
  maturity,
  maintainers,
}: ProjectCardProps) {
  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>
        <div className={styles.cardHeader}>
          <h2 className={styles.cardTitle}>
            <a className={styles.cardTitleLink} href={`/projects/${id}`}>
              {name}
            </a>
          </h2>
          <span className={styles.maturityChip}>{maturity}</span>
        </div>
        <table className={styles.maintainersTable}>
          <thead>
            <tr>
              <th>Name</th>
              <th>GitHub</th>
            </tr>
          </thead>
          <tbody>
            {maintainers.map((maintainer, index) => (
              <tr key={`${name}-${index}`}>
                <td>
                  <a className={styles.maintainerLink} href={`/maintainers/${maintainer.id}`}>
                    {maintainer.name || "Unknown"}
                  </a>
                </td>
                <td>{maintainer.github}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
