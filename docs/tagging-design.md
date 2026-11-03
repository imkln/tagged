# Tags and tag search

Proposed revision to the current milestone. Issue #9 remains unchanged because it was already approved; subsequent scope decisions live here, in chat, and in the implementation PR.

## First delivery

New JPEGs gain metadata and ordinary country/city tags. The gallery shows tags and offers one tag picker. There are no country/state/city controls, location-specific tag types, camera tags, date tags, or state tags. Available metadata may appear in photo details. Keep newest-first date ordering with upload-date fallback and defer existing-photo backfill.

Manual tag editing, batch tagging, AI calls, and tag descriptions remain later work. The data and search contracts support arbitrary tag names from the start; only the automatic producer is restricted to country and city in this delivery.

## Tag identity and storage

- A tag has an ID and display name. Normalize names with Unicode normalization, case folding, trimming, and collapsed whitespace. Keep accents and punctuation meaningful. ` Rome ` and `rome` identify the same tag.
- Use a deterministic ID derived from the normalized name, shared by every producer. No location namespace, geographic hierarchy, parent relationship, or special country/city identity. Two places called `Paris` share the `Paris` tag; selecting a country as well narrows the results.
- Store assignments on each photo as tag ID, display name, and sources. Start with source `metadata`; later add `manual` and `ai`. The UI shows one chip even if multiple sources supplied the same tag.
- Derive the searchable tag catalog from saved assignments. No separate registry, search service, table, or index for the initial personal library. When display casing differs, select a deterministic label for catalog suggestions; matching always uses identity.
- Keep extracted metadata separate from tags. Saving camera details, dates, or coordinates must not automatically make them tags.
- Use conditional updates to the fields owned by processing. Never replace an entire photo record with a stale worker snapshot. Repeating a producer's update replaces only that producer's assignments and cannot create duplicates or remove assignments from another source.
- When manual removal is implemented, persist a per-photo dismissal so automated retries cannot silently restore a removed tag. The first delivery does not expose manual removal; its tests cover idempotence and preserving other sources, not a speculative editing API.

## Search experience and API

- One accessible tag input suggests existing tags by case-insensitive name substring. Typing narrows suggestions; selecting a suggestion adds a removable chip. Typing alone does not execute a photo search or create a tag.
- Selected tags use AND matching: `Italy` + `Rome` finds photos with both. Later, `Italy` + `Cat` uses exactly the same UI and backend code.
- Suggestions exclude already selected tags and come from the full library, so a zero-result search does not trap the user. Support keyboard selection, chip removal, and Reset filters.
- Distinguish an empty library from no matching photos and from a failed search. Preserve the current filters during upload polling and pagination; changing filters resets pagination.
- `GET /api/tags?q=rom` returns unique `{id, name}` suggestions in deterministic name order. It is owner-authorized like all other library data.
- `GET /api/photos?tag=<id>&tag=<id>&limit=50&cursor=...` matches all selected tag IDs before pagination and returns photos, total matched count, and an optional next cursor. Duplicate tag IDs are harmless. Unknown tag IDs produce no matches.
- Order by capture time, falling back to upload time; undated photos last; photo ID breaks ties. Preserve recorded camera-local times when offsets are absent rather than assuming a timezone.
- Bind opaque pagination cursors to the selected tags and sort order. Reject malformed cursors. No full-text photo search, query language, OR/NOT operators, or relevance scoring.
- Initial implementation scans the existing photo table, filters and sorts on the server, then returns one page. This keeps the current storage architecture but scans remain proportional to library size. Concurrent uploads can shift pages; a snapshot guarantee is not included. Add an index only when library size justifies it.

## Country/city producer

- Prefer valid embedded country/city text when present. Otherwise, use valid GPS coordinates to resolve country and city. Only publish available country/city names as ordinary tags. No guessing from filenames or image contents, and no substituting a district, state, or nearby point of interest for a missing city.
- Use consistent English place labels when the lookup supports them. Country-only results are useful; missing GPS or unresolved locations produce no location tags. Do not fail a valid thumbnail because a place lookup failed.
- Resolve locations on the server. Save the result so browsing, searches, polling, and refreshes never trigger provider calls. Do not log precise GPS coordinates.
- Propose Amazon Location ReverseGeocode as the first provider to evaluate because the application already runs on AWS. It returns country and locality components. Persisted results require `IntendedUse=Storage`, which has different pricing from transient lookups: https://docs.aws.amazon.com/location/latest/APIReference/API_geoplaces_ReverseGeocode.html and https://docs.aws.amazon.com/location/latest/developerguide/places-intended-use.html .
- Provider choice remains pending: confirm locality semantics, pricing for stored results, coordinates sent, permissions, and a per-batch call budget before enabling it. No provider requests or AWS changes are authorized by this design document. Mock the lookup in tests. A lookup failure leaves the photo viewable without generated place tags; automatic recovery/manual retry is deferred and must be reported as a limitation.

## Tests to add after design approval

- One normalized identity across casing, whitespace, and producers; distinct meaningful names remain distinct.
- Country and city become ordinary tags; country-only and no-location results work; other metadata never becomes tags.
- Repeated processing is idempotent; producer updates preserve unrelated sources and photo fields.
- Lookup success, partial result, invalid GPS, no result, and failure using a fake provider; preserve valid thumbnails and originals.
- Generic AND matching, duplicate selections, unknown tags, stable ordering, and pagination after filtering.
- Unique catalog suggestions, query matching, and access denial for catalog/search routes.
- Browser suggestion selection, keyboard behavior, chips, reset, no matches, request failures, and retaining selection during polling.

Existing metadata extraction, orientation, omission, and date-ordering tests remain pending review. No tests are run and no behavior is implemented until the owner approves the tests.
