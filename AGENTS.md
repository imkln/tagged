# Working on tagged

## Development and review

- Agree on the milestone, scope, and visible result before implementation. Use `docs/plan.md` as the single project to-do list.
- Deliver complete user-facing workflows in focused PRs. The owner is an experienced developer; avoid splitting work into tiny setup-only PRs or adding unrelated changes.
- Work on a `codex/` branch. Commit and push the scoped changes, then open a PR. Do not push directly to `main`.
- Run checks appropriate to the change. In the PR, describe the resulting behavior, how to try it, validation performed, and material limitations. Include screenshots for visible interface changes when practical.
- The owner reviews on GitHub and tells the agent when the review is ready. Read the review comments and requested changes, address them, and push fixes to the same PR.
- Respond to review feedback with what changed and how it was verified. Explain disagreements and ask for decisions when needed. Do not dismiss reviews or resolve the owner's review threads on their behalf unless asked.
- Repeat the review cycle until the owner is satisfied. The owner merges, or explicitly asks the agent to merge. Approval alone is not an instruction to merge.
- Before creating or changing AWS resources, include the infrastructure changes and expected running cost in the PR for review, and obtain the owner's instruction to apply them. Preparing infrastructure code does not authorize deployment.

## Project conventions

- Use Go for the backend, S3 for photos, DynamoDB for metadata and tags, and SQS for background processing. Keep the browser interface usable on desktop and mobile.
- Start with owner-only access. Enforce authorization on the backend, including uploads and photo retrieval. Public viewing belongs to its milestone and requires explicitly published photos; uploads remain private by default.
- Keep the README focused on the product. Keep planned work in the table in `docs/plan.md`; do not duplicate to-do lists or label planned features as “future.” Describe current behavior accurately and distinguish it from the finished product when necessary.
- Add `.gitignore` entries only for files or tools actually used by the project. Do not prepopulate speculative rules.
- Keep credentials and personal photos out of Git. Never delete photos automatically.
