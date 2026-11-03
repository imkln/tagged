# tagged

A private photo library for uploading, searching, and organizing photos from desktop or mobile.

The finished app will combine manual and AI-generated tags to make your library searchable, let you correct tags and apply new ones to older photos, and surface untagged images for review. It will group similar shots so you can choose the best and delete unwanted copies.

## Upload architecture

```mermaid
flowchart LR
    Browser -->|Authorize uploads and browse| API[Go API]
    API -->|Photo records| DB[(DynamoDB)]
    Browser -->|Authorized uploads and viewing| S3[(Private S3)]
    S3 -->|Original uploaded| Worker
    Worker -->|Thumbnail| S3
    Worker -->|Processing status| DB
```
