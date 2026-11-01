# Small, reviewable pieces

Each piece should be usable or understandable on its own. Review it before starting the next; the list below is a proposed sequence, not authorization to implement everything at once.

| Piece | Deliverable | What to review |
| --- | --- | --- |
| 1. Project setup | Private GitHub repository, brief, and this plan | Is the problem and first working version right? |
| 2. Gallery shell | Responsive interface with clearly labeled sample data | Desktop/mobile layout, navigation, and empty state |
| 3. Local photo upload | Upload JPEGs, persist them locally, and view them in the gallery | One real photo can be uploaded and viewed after restart |
| 4. Metadata | Capture date, dimensions, and camera metadata where available | Missing metadata and image orientation are handled honestly |
| 5. First AWS deployment | Authentication, private S3 storage, PostgreSQL, and an internet-accessible app | Deployment plan and expected ongoing costs before creating resources; authenticated upload and viewing afterward |
| 6. Background tagging | Queue, worker, one vision provider, processing states, and retries | One uploaded photo receives suggested tags; failures are visible and retryable |
| 7. Search and corrections | Combined tag filters and manual tag edits | Filters work together and AI processing preserves user corrections |
| 8. Needs review and new tags | Untagged/uncertain view, tag definitions, and opt-in backfill | Review states are clear and backfill preserves existing corrections |
| 9. Similar photos | Suggested composition groups and side-by-side review | Useful grouping and explicit selection; deletion is a separate user action |

## Boundaries for the first working version

- One owner and JPEG uploads. HEIC, RAW, video, sharing, and iCloud sync can wait.
- Local development first; AWS deployment is a separate piece.
- No AI calls or ongoing cloud spend during project setup.
- No automatic photo deletion.
- The reference screenshot communicates design direction; its sample photos, counts, trips, and storage figures are not real library data.

## Decisions to make when relevant

- **Before uploading real photos to AWS:** account/region, sign-in method, retention, backups, location privacy, and an acceptable monthly budget.
- **Before AI tagging:** provider/model, credentials, which image/metadata is sent externally, confidence handling, and a per-photo cost limit.
- **Before similarity review:** grouping method, what counts as the same composition, and reversible deletion behavior.
