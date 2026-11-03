# Working on tagged

## Workflow

- Propose milestones in GitHub issues and get the owner's approval before starting implementation; skip issues for small changes.
- Deliver useful, user-facing milestones in focused PRs. Use descriptive branches: `feature/`, `fix/`, `docs/`, or `chore/`. Never push directly to `main`.
- Link the issue, explain how to try the result, and report relevant limitations. Include screenshots for UI changes when practical.
- When the owner says a review is ready, read GitHub feedback and push fixes to the same PR. Keep the PR current with decisions made in chat; leave approved issues unchanged.
- Leave merging and resolving the owner's review threads to the owner unless explicitly asked.
- Include infrastructure changes and expected costs in the PR. Apply AWS changes only when explicitly instructed.
- Write readable, maintainable code with clear names and responsibilities. Avoid speculative abstractions; comment only when a non-obvious reason needs explaining.
- Follow TDD: write meaningful behavior tests, run them and confirm they fail before implementing the behavior, then implement and refactor with the tests passing.
- Present tests for the owner's review and wait for approval before running them or implementing the behavior.
- Use idiomatic structure and standard automated formatting for all code and configuration. Separate components, styles, and responsibilities logically. Keep tests as readable as production code, with focused cases, descriptive names, and checked setup errors.
- Match the surrounding repository style before writing or changing code. Separate logical chunks with blank lines, including test setup, actions, and assertions; separate independent checks and helper-building steps where it improves readability. Leave a blank line between CSS rule blocks, including inside media queries. Automated formatters do not replace this manual spacing review.
- Reserve `docs/` for maintained, long-term project documentation. Do not commit temporary design documents, proposals, implementation plans, review notes, or scratch documents. The existing `docs/plan.md` is the maintained roadmap.
- Never commit or push personally identifiable information or secrets, including in fixtures, logs, or generated files. Use fictional test identities and external configuration; inspect staged changes before committing.

## Issue proposals

Keep proposals brief and human-readable. Use these sections:

- **Description:** A high-level description of the change, followed by a list of individual pieces to implement as acceptance criteria.
- **Design:** A short list of proposed implementation choices, with enough technical detail for the owner to steer the approach.

Approval:

- The owner reviews the proposal and approves it or requests changes before implementation starts.
- Record approval with an `approved` label before starting implementation.
- After approval, leave the issue unchanged: no edits, progress comments, or status updates. Keep implementation discussion in chat and the PR.

## PR format

Keep PRs brief and human-readable. Do not include automated test results; CI reports them. Use these sections:

- **Description:** What the change delivers and the main changes made to achieve it, at a high level.
- **Testing:** A short list of behaviors to check once the feature branch is live. Do not tell the reviewer to deploy or run automated tests.
- **Notes (optional):** Relevant limitations, infrastructure costs, or other useful context.
