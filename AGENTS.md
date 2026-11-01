# Working on tagged

## Workflow

- Agree on scope and acceptance criteria in chat. Record agreed milestones in GitHub issues; skip issues for small changes.
- Deliver useful, user-facing milestones in focused PRs. Use descriptive branches: `feature/`, `fix/`, `docs/`, or `chore/`. Never push directly to `main`.
- Link the issue, explain how to try the result, and report relevant checks and limitations. Include screenshots for UI changes when practical.
- When the owner says a review is ready, read GitHub feedback and push fixes to the same PR. Keep the issue/PR current with decisions made in chat.
- Leave merging and resolving the owner's review threads to the owner unless explicitly asked.
- Include infrastructure changes and expected costs in the PR. Apply AWS changes only when explicitly instructed.
- Write readable, maintainable code with clear names and responsibilities. Avoid speculative abstractions; comment only when a non-obvious reason needs explaining.
- Follow TDD: write meaningful behavior tests, run them and confirm they fail before implementing the behavior, then implement and refactor with the tests passing.
- Present tests for the owner's review and wait for approval before implementing the behavior.
- Follow idiomatic language conventions and standard formatting (`gofmt` for Go). Keep tests as readable as production code: focused cases, descriptive names, normal multiline control flow, and checked setup errors.
