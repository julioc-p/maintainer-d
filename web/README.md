This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Implementation

1) Web UI calls the BFF

- In `web/src/app/page.tsx` we call:
  - `GET /api/projects?query=...&limit=...&offset=...&sort=name&direction=asc|desc&maturity=...`
- This is done in a `useEffect` so the cards always reflect current filters/pagination.

2) BFF verifies session

- The route `/api/projects` is registered with `requireSession`, so only authenticated users can access it.
- That middleware checks the `md_session` cookie against the in-memory session store.

3) BFF queries SQLite via GORM

- The handler is `handleProjects` in `cmd/web-bff/main.go`.
- It uses a `SQLStore` to access the DB (new `DB()` accessor in `db/store_impl.go).

Key query steps:

- Start a base query:
  - `base := s.store.DB().Model(&model.Project{})`
- Apply filters:
  - Maturity filter: `WHERE projects.maturity IN (...)`
  - Query search (project name or maintainer name/login):
    - `JOIN maintainer_projects + JOIN maintainers`
    - `WHERE LOWER(projects.name) LIKE ... OR LOWER(maint.name) LIKE ... OR LOWER(maint.git_hub_account) LIKE ...`

4) Count total results

- `base.Distinct("projects.id").Count(&total)`
- This gives you the “1–20 of N” count.

5) Page + sort

- Pull only IDs for the page:
  - `SELECT DISTINCT projects.id ORDER BY projects.name asc|desc LIMIT ... OFFSET ...`
- This gives you just the project IDs you need.

6) Load project + maintainers

- Query for those IDs with maintainers preloaded:
  - `Preload("Maintainers").Where("projects.id IN ?", ids).Find(&results)`
- Assemble a stable output list in the same order as the ID list.

7) Response shape

- JSON response:

```
{
  "total": 123,
  "projects": [
    {
      "id": 1,
      "name": "Kubernetes",
      "maturity": "Graduated",
      "maintainers": ["alice", "bob"]
    }
  ]
}
```

8) Web renders

- Cards are driven directly by that response; the “Maintainers” table is just the list of names from the response.
