# tagged

A personal photo library that makes thousands of photos easier to find and review.

Upload photos from a desktop or mobile browser, preserve their metadata, and use a vision-capable AI to suggest searchable tags. Over time, tagged will also help review similar compositions and find photos that need manual attention.

## First working version

Start with one small workflow: **upload a JPEG and see it in a private gallery**.

Build and review each piece separately before adding the next. This repository currently contains the project brief and plan only; there is no application or deployed infrastructure yet.

## Intended direction

- A responsive browser interface, using the supplied dark gallery mockup as a visual reference.
- A Go backend deployed to AWS.
- Private photo storage in S3 and library records in PostgreSQL.
- Background AI tagging, with SQS added when the processing pipeline needs it.
- A configurable vision provider, with API credentials kept on the server.

These are starting preferences, not infrastructure already provisioned. Hosting, authentication, costs, and the AI provider will be decided in their own reviewable pieces.

## Later capabilities

- Combine tags to filter photos and correct suggestions manually.
- Extract capture dates, camera details, and available location metadata without guessing missing information.
- Surface untagged or uncertain photos for review, separately from processing failures.
- Define a new tag and explicitly request a background pass over older photos to look for matches.
- Suggest groups of similar compositions so you can compare shots and choose what to keep. Never delete photos automatically.

See [the incremental plan](docs/plan.md) for the proposed sequence.
